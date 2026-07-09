package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os/exec"
	"slices"
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
	Proxy           string
	CookiesFile     string
	JSRuntime       string
}

type YTDLP struct {
	binary          string
	metadataTimeout time.Duration
	downloadTimeout time.Duration
	proxy           string
	cookiesFile     string
	jsRuntime       string
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
	return &YTDLP{
		binary:          config.Binary,
		metadataTimeout: config.MetadataTimeout,
		downloadTimeout: config.DownloadTimeout,
		proxy:           config.Proxy,
		cookiesFile:     config.CookiesFile,
		jsRuntime:       config.JSRuntime,
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

func (y *YTDLP) baseArgs() []string {
	args := []string{"--ignore-config"}
	if y.proxy != "" {
		args = append(args, "--proxy", y.proxy)
	}
	if y.cookiesFile != "" {
		args = append(args, "--cookies", y.cookiesFile)
	}
	if y.jsRuntime != "" {
		args = append(args, "--js-runtimes", y.jsRuntime)
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
	formats := make([]model.Format, 0, len(m.Formats))
	seen := make(map[string]struct{})
	for _, f := range m.Formats {
		if f.FormatID == "" || f.Ext == "" {
			continue
		}
		if f.VCodec == "none" && f.ACodec == "none" {
			continue
		}

		resolution := f.Resolution
		if resolution == "" && f.Height > 0 {
			resolution = fmt.Sprintf("%.0fp", f.Height)
		}
		if resolution == "" {
			resolution = f.FormatNote
		}
		if resolution == "" {
			resolution = "audio"
		}

		key := f.FormatID + "|" + resolution + "|" + f.Ext
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}

		formats = append(formats, model.Format{
			FormatID:   f.FormatID,
			Resolution: resolution,
			Ext:        f.Ext,
			Note:       f.FormatNote,
			Protocol:   f.Protocol,
		})
	}

	return model.MetadataResponse{
		Title:     m.Title,
		Thumbnail: m.Thumbnail,
		Duration:  m.Duration,
		Formats:   formats,
	}
}
