package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"downloader-2607/internal/model"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"
)

var allowedHosts = []string{
	"bilibili.com",
	"b23.tv",
	"douyin.com",
	"facebook.com",
	"fb.watch",
	"instagram.com",
	"m.facebook.com",
	"m.twitch.tv",
	"m.youtube.com",
	"music.youtube.com",
	"naver.com",
	"reddit.com",
	"redd.it",
	"soundcloud.com",
	"tiktok.com",
	"twitch.tv",
	"twitter.com",
	"vimeo.com",
	"vm.tiktok.com",
	"www.bilibili.com",
	"www.facebook.com",
	"www.instagram.com",
	"www.reddit.com",
	"www.tiktok.com",
	"www.twitch.tv",
	"www.youtube.com",
	"x.com",
	"youtu.be",
	"youtube.com",
}

type Config struct {
	Binary          string
	MetadataTimeout time.Duration
	DownloadTimeout time.Duration
	DownloadWorkers int
	Proxy           string
	CookiesFile     string
	CookiesBrowser  string
	JSRuntime       string
	Impersonate     string
}

type YTDLP struct {
	binary          string
	metadataTimeout time.Duration
	downloadTimeout time.Duration
	proxy           string
	cookiesFile     string
	cookiesBrowser  string
	jsRuntime       string
	impersonate     string
	downloadSlots   chan struct{}
}

type Error struct {
	Status  int
	Code    string
	Message string
}

func (e Error) Error() string {
	return e.Message
}

func NewYTDLP(config Config) *YTDLP {
	if config.Binary == "" {
		config.Binary = "yt-dlp"
	}
	if config.MetadataTimeout == 0 {
		config.MetadataTimeout = 30 * time.Second
	}
	if config.DownloadTimeout == 0 {
		config.DownloadTimeout = 30 * time.Minute
	}
	if config.DownloadWorkers < 1 {
		config.DownloadWorkers = 1
	}
	return &YTDLP{
		binary:          config.Binary,
		metadataTimeout: config.MetadataTimeout,
		downloadTimeout: config.DownloadTimeout,
		proxy:           config.Proxy,
		cookiesFile:     config.CookiesFile,
		cookiesBrowser:  config.CookiesBrowser,
		jsRuntime:       config.JSRuntime,
		impersonate:     config.Impersonate,
		downloadSlots:   make(chan struct{}, config.DownloadWorkers),
	}
}

func (y *YTDLP) Metadata(ctx context.Context, rawURL string) (model.MetadataResponse, error) {
	if err := validateURL(rawURL); err != nil {
		return model.MetadataResponse{}, err
	}

	ctx, cancel := context.WithTimeout(ctx, y.metadataTimeout)
	defer cancel()

	args := y.baseArgs()
	args = append(args, "--dump-single-json", "--no-playlist", rawURL)
	cmd := exec.CommandContext(ctx, y.binary, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if ctx.Err() == context.DeadlineExceeded {
		return model.MetadataResponse{}, Error{Status: fiber.StatusGatewayTimeout, Code: "EXTRACTION_TIMEOUT", Message: "metadata extraction timed out"}
	}
	if err != nil {
		message := stderr.String()
		log.Warn().Err(err).Str("stderr", message).Msg("yt-dlp metadata failed")
		return model.MetadataResponse{}, classifyExtractionError(message)
	}

	var raw metadataJSON
	if err := json.Unmarshal(output, &raw); err != nil {
		log.Warn().Err(err).Str("stdout", string(output)).Str("stderr", stderr.String()).Msg("yt-dlp metadata parse failed")
		return model.MetadataResponse{}, Error{Status: fiber.StatusBadGateway, Code: "EXTRACTION_FAILED", Message: "failed to parse metadata"}
	}

	return raw.toResponse(), nil
}

func (y *YTDLP) Stream(ctx context.Context, rawURL, formatID string, dst io.Writer) error {
	if err := validateURL(rawURL); err != nil {
		return err
	}
	if strings.ContainsAny(formatID, "\x00\r\n") {
		return Error{Status: fiber.StatusBadRequest, Code: "BAD_REQUEST", Message: "invalid format_id"}
	}

	ctx, cancel := context.WithTimeout(ctx, y.downloadTimeout)
	defer cancel()

	select {
	case y.downloadSlots <- struct{}{}:
		defer func() { <-y.downloadSlots }()
	case <-ctx.Done():
		return Error{Status: fiber.StatusGatewayTimeout, Code: "DOWNLOAD_TIMEOUT", Message: "download timed out while waiting"}
	}

	selector, merged := downloadSelector(formatID)
	if merged {
		return y.downloadMerged(ctx, rawURL, selector, dst)
	}

	args := y.baseArgs()
	args = append(args,
		"--no-playlist",
		"--no-part",
		"-f", formatID,
		"-o", "-",
		rawURL,
	)
	cmd := exec.CommandContext(ctx, y.binary, args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return Error{Status: fiber.StatusInternalServerError, Code: "STREAM_FAILED", Message: "failed to open download stream"}
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return Error{Status: fiber.StatusInternalServerError, Code: "STREAM_FAILED", Message: "failed to open download logs"}
	}

	if err := cmd.Start(); err != nil {
		log.Warn().Err(err).Msg("yt-dlp start failed")
		return Error{Status: fiber.StatusBadGateway, Code: "EXTRACTION_FAILED", Message: "failed to start downloader"}
	}

	errCh := make(chan error, 1)
	go logStderr(stderr)
	go func() {
		_, copyErr := io.Copy(dst, stdout)
		waitErr := cmd.Wait()
		if copyErr != nil {
			errCh <- copyErr
			return
		}
		errCh <- waitErr
	}()

	if err := <-errCh; err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return Error{Status: fiber.StatusGatewayTimeout, Code: "DOWNLOAD_TIMEOUT", Message: "download timed out"}
		}
		log.Warn().Err(err).Msg("yt-dlp stream failed")
		return Error{Status: fiber.StatusBadGateway, Code: "EXTRACTION_FAILED", Message: "download failed"}
	}

	return nil
}

func (y *YTDLP) downloadMerged(ctx context.Context, rawURL, selector string, dst io.Writer) error {
	dir, err := os.MkdirTemp("", "tubedown-*")
	if err != nil {
		return Error{Status: fiber.StatusInternalServerError, Code: "DOWNLOAD_FAILED", Message: "failed to prepare download"}
	}
	defer os.RemoveAll(dir)

	outputTemplate := filepath.Join(dir, "video.%(ext)s")
	args := y.baseArgs()
	args = append(args,
		"--no-playlist",
		"--no-part",
		"--merge-output-format", "mp4",
		"-f", selector,
		"-o", outputTemplate,
		rawURL,
	)

	cmd := exec.CommandContext(ctx, y.binary, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return Error{Status: fiber.StatusGatewayTimeout, Code: "DOWNLOAD_TIMEOUT", Message: "download timed out"}
		}
		log.Warn().Err(err).Str("stderr", stderr.String()).Msg("yt-dlp merged download failed")
		return classifyExtractionError(stderr.String())
	}

	files, err := filepath.Glob(filepath.Join(dir, "video.*"))
	if err != nil || len(files) == 0 {
		return Error{Status: fiber.StatusBadGateway, Code: "DOWNLOAD_FAILED", Message: "downloaded file was not created"}
	}

	file, err := os.Open(files[0])
	if err != nil {
		return Error{Status: fiber.StatusInternalServerError, Code: "DOWNLOAD_FAILED", Message: "failed to open download"}
	}
	defer file.Close()
	if _, err := io.Copy(dst, file); err != nil {
		return Error{Status: fiber.StatusInternalServerError, Code: "STREAM_FAILED", Message: "failed to stream download"}
	}
	return nil
}

func downloadSelector(formatID string) (string, bool) {
	switch formatID {
	case "quality-2160":
		return "bestvideo[height<=2160]+bestaudio/best[height<=2160]", true
	case "quality-1440":
		return "bestvideo[height<=1440]+bestaudio/best[height<=1440]", true
	case "quality-1080":
		return "bestvideo[height<=1080]+bestaudio/best[height<=1080]", true
	case "quality-720":
		return "bestvideo[height<=720]+bestaudio/best[height<=720]", true
	case "quality-480":
		return "bestvideo[height<=480]+bestaudio/best[height<=480]", true
	case "quality-360":
		return "bestvideo[height<=360]+bestaudio/best[height<=360]", true
	default:
		return formatID, false
	}
}

func (y *YTDLP) baseArgs() []string {
	args := []string{"--ignore-config"}
	if y.proxy != "" {
		args = append(args, "--proxy", y.proxy)
	}
	if y.cookiesFile != "" {
		args = append(args, "--cookies", y.cookiesFile)
	}
	if y.cookiesBrowser != "" {
		args = append(args, "--cookies-from-browser", y.cookiesBrowser)
	}
	if y.jsRuntime != "" {
		args = append(args, "--js-runtimes", y.jsRuntime)
	}
	if y.impersonate != "" {
		args = append(args, "--impersonate", y.impersonate)
	}
	return args
}

func classifyExtractionError(message string) Error {
	lower := strings.ToLower(message)
	switch {
	case strings.Contains(lower, "too many requests") || strings.Contains(lower, "http error 429"):
		return Error{
			Status:  fiber.StatusBadGateway,
			Code:    "PLATFORM_RATE_LIMITED",
			Message: "YouTube is rate-limiting this server. Configure a proxy or cookies for Railway.",
		}
	case strings.Contains(lower, "sign in to confirm") || strings.Contains(lower, "not a bot"):
		return Error{
			Status:  fiber.StatusBadGateway,
			Code:    "PLATFORM_AUTH_REQUIRED",
			Message: "YouTube is asking this server to confirm it is not a bot. Configure YouTube cookies or a proxy.",
		}
	case strings.Contains(lower, "instagram api is not granting access") ||
		strings.Contains(lower, "empty media response") ||
		strings.Contains(lower, "use --cookies"):
		return Error{
			Status:  fiber.StatusBadGateway,
			Code:    "PLATFORM_AUTH_REQUIRED",
			Message: "This platform requires an accessible public post or cookies for authentication.",
		}
	case strings.Contains(lower, "no video could be found"):
		return Error{
			Status:  fiber.StatusBadRequest,
			Code:    "NO_VIDEO_FOUND",
			Message: "No downloadable video was found at this URL.",
		}
	case strings.Contains(lower, "unsupported url"):
		return Error{Status: fiber.StatusBadRequest, Code: "INVALID_URL", Message: "unsupported url"}
	default:
		return Error{Status: fiber.StatusBadGateway, Code: "EXTRACTION_FAILED", Message: "failed to extract metadata"}
	}
}

func validateURL(rawURL string) error {
	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return Error{Status: fiber.StatusBadRequest, Code: "INVALID_URL", Message: "invalid url"}
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return Error{Status: fiber.StatusBadRequest, Code: "INVALID_URL", Message: "unsupported url scheme"}
	}

	host := strings.ToLower(parsed.Hostname())
	if slices.Contains(allowedHosts, host) {
		return nil
	}
	for _, allowed := range allowedHosts {
		if strings.HasSuffix(host, "."+allowed) {
			return nil
		}
	}

	return Error{Status: fiber.StatusBadRequest, Code: "INVALID_URL", Message: "unsupported platform"}
}

func logStderr(r io.Reader) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			log.Debug().Str("yt_dlp", line).Msg("download progress")
		}
	}
}

type metadataJSON struct {
	Title     string       `json:"title"`
	Thumbnail string       `json:"thumbnail"`
	Duration  float64      `json:"duration"`
	Formats   []formatJSON `json:"formats"`
}

type formatJSON struct {
	FormatID   string  `json:"format_id"`
	Ext        string  `json:"ext"`
	Resolution string  `json:"resolution"`
	FormatNote string  `json:"format_note"`
	Protocol   string  `json:"protocol"`
	VCodec     string  `json:"vcodec"`
	ACodec     string  `json:"acodec"`
	Height     float64 `json:"height"`
}

func (m metadataJSON) toResponse() model.MetadataResponse {
	maxHeight := 0
	for _, f := range m.Formats {
		if f.VCodec != "none" && int(f.Height) > maxHeight {
			maxHeight = int(f.Height)
		}
	}

	qualities := []int{2160, 1440, 1080, 720, 480, 360}
	formats := make([]model.Format, 0, len(qualities))
	for _, quality := range qualities {
		if maxHeight > 0 && quality > maxHeight {
			continue
		}
		formats = append(formats, model.Format{
			FormatID:   fmt.Sprintf("quality-%d", quality),
			Resolution: fmt.Sprintf("%dp", quality),
			Ext:        "mp4",
			Note:       "video + audio",
			Quality:    quality,
		})
	}
	if len(formats) == 0 {
		formats = append(formats, model.Format{FormatID: "quality-360", Resolution: "360p", Ext: "mp4", Note: "video + audio", Quality: 360})
	}
	sort.Slice(formats, func(i, j int) bool { return formats[i].Quality > formats[j].Quality })

	return model.MetadataResponse{
		Title:     m.Title,
		Thumbnail: m.Thumbnail,
		Duration:  m.Duration,
		Formats:   formats,
	}
}
