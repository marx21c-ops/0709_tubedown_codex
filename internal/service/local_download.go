package service

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"
)

var progressPercentPattern = regexp.MustCompile(`(?i)([0-9]+(?:\.[0-9]+)?)%`)

// DownloadLocal writes directly to a trusted local output template. It never
// invokes a shell and only accepts quality presets defined by this service.
func (y *YTDLP) DownloadLocal(ctx context.Context, rawURL, formatID, outputTemplate string, progress func(float64)) (string, error) {
	if err := validateURL(rawURL); err != nil {
		return "", err
	}
	selector, merged := downloadSelector(formatID)
	if !merged {
		return "", Error{Status: fiber.StatusBadRequest, Code: "BAD_REQUEST", Message: "unsupported quality preset"}
	}

	args := y.baseArgs()
	args = append(args,
		"--no-playlist",
		"--newline",
		"--no-part",
		"--merge-output-format", "mp4",
		"--windows-filenames",
		"--trim-filenames", "180",
		"--progress-template", "download:%(progress._percent_str)s",
		"--print", "after_move:filepath:%(filepath)s",
		"-f", selector,
		"-o", outputTemplate,
		rawURL,
	)

	cmd := exec.CommandContext(ctx, y.binary, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
	}
	cmd.WaitDelay = 5 * time.Second
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", Error{Status: fiber.StatusInternalServerError, Code: "DOWNLOAD_FAILED", Message: "failed to read download progress"}
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return "", Error{Status: fiber.StatusInternalServerError, Code: "DOWNLOAD_FAILED", Message: "failed to read downloader logs"}
	}
	var stderr bytes.Buffer

	if err := cmd.Start(); err != nil {
		return "", Error{Status: fiber.StatusBadGateway, Code: "DOWNLOAD_FAILED", Message: "failed to start downloader"}
	}

	var outputPath string
	var readers sync.WaitGroup
	readers.Add(2)
	go func() {
		defer readers.Done()
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(line, "filepath:") {
				outputPath = strings.TrimPrefix(line, "filepath:")
				continue
			}
			reportProgress(line, progress)
		}
	}()
	go func() {
		defer readers.Done()
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			line := scanner.Text()
			reportProgress(line, progress)
			stderr.WriteString(line)
			stderr.WriteByte('\n')
		}
	}()

	waitErr := cmd.Wait()
	readers.Wait()
	if waitErr != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			return "", Error{Status: fiber.StatusRequestTimeout, Code: "DOWNLOAD_CANCELLED", Message: "download cancelled"}
		}
		message := strings.TrimSpace(stderr.String())
		log.Warn().Err(waitErr).Str("stderr", message).Msg("local yt-dlp download failed")
		return "", classifyExtractionError(message)
	}
	if progress != nil {
		progress(100)
	}
	return outputPath, nil
}

func reportProgress(line string, progress func(float64)) {
	if progress == nil {
		return
	}
	match := progressPercentPattern.FindStringSubmatch(line)
	if len(match) != 2 {
		return
	}
	value, err := strconv.ParseFloat(match[1], 64)
	if err == nil {
		progress(value)
	}
}
