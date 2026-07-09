package handler

import (
	"bufio"
	"context"
	"net/url"
	"strings"

	"downloader-2607/internal/service"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"
)

func Download(ytdlp *service.YTDLP) fiber.Handler {
	return func(c *fiber.Ctx) error {
		rawURL := strings.TrimSpace(c.Query("url"))
		formatID := strings.TrimSpace(c.Query("format_id"))
		if rawURL == "" {
			return AppError{Status: fiber.StatusBadRequest, Code: "INVALID_URL", Message: "url is required"}
		}
		if formatID == "" {
			return AppError{Status: fiber.StatusBadRequest, Code: "BAD_REQUEST", Message: "format_id is required"}
		}

		filename := "download.mp4"
		if title := strings.TrimSpace(c.Query("title")); title != "" {
			filename = sanitizeFilename(title) + ".mp4"
		}

		c.Set(fiber.HeaderContentType, "application/octet-stream")
		c.Set(fiber.HeaderContentDisposition, `attachment; filename*=UTF-8''`+url.PathEscape(filename))

		c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
			err := ytdlp.Stream(context.Background(), rawURL, formatID, flushWriter{writer: w})
			if err != nil {
				log.Warn().Err(err).Str("url", rawURL).Str("format_id", formatID).Msg("download stream failed")
			}
		})

		return nil
	}
}

type flushWriter struct {
	writer *bufio.Writer
}

func (w flushWriter) Write(p []byte) (int, error) {
	n, err := w.writer.Write(p)
	if err != nil {
		return n, err
	}
	return n, w.writer.Flush()
}

func sanitizeFilename(name string) string {
	name = strings.TrimSpace(name)
	replacer := strings.NewReplacer("/", "-", "\\", "-", ":", "-", "*", "-", "?", "", `"`, "", "<", "", ">", "", "|", "-")
	name = replacer.Replace(name)
	if name == "" {
		return "download"
	}
	return name
}
