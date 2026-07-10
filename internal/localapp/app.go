package localapp

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"downloader-2607/internal/model"
	"downloader-2607/internal/service"

	"github.com/gofiber/fiber/v2"
)

type App struct {
	server      *fiber.App
	ytdlp       *service.YTDLP
	token       string
	nonce       string
	downloadDir string
	mu          sync.RWMutex
	job         *Job
	cancel      context.CancelFunc
}

type Job struct {
	ID         string  `json:"id"`
	Title      string  `json:"title"`
	Quality    string  `json:"quality"`
	Status     string  `json:"status"`
	Progress   float64 `json:"progress"`
	OutputDir  string  `json:"output_dir"`
	Error      string  `json:"error,omitempty"`
	StartedAt  string  `json:"started_at"`
	FinishedAt string  `json:"finished_at,omitempty"`
}

type downloadRequest struct {
	URL      string `json:"url"`
	Title    string `json:"title"`
	FormatID string `json:"format_id"`
}

func New(ytdlp *service.YTDLP, downloadDir string) (*App, error) {
	token, err := randomToken(32)
	if err != nil {
		return nil, err
	}
	nonce, err := randomToken(18)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(downloadDir, 0o700); err != nil {
		return nil, fmt.Errorf("create download directory: %w", err)
	}

	a := &App{ytdlp: ytdlp, token: token, nonce: nonce, downloadDir: downloadDir}
	a.server = fiber.New(fiber.Config{
		AppName:               "TubeDown Local",
		DisableStartupMessage: true,
		ErrorHandler:          a.errorHandler,
	})
	a.server.Use(a.securityMiddleware)
	a.server.Get("/", a.home)
	a.server.Post("/api/metadata", a.requireToken, a.metadata)
	a.server.Post("/api/download", a.requireToken, a.startDownload)
	a.server.Get("/api/job", a.requireToken, a.getJob)
	a.server.Post("/api/cancel", a.requireToken, a.cancelDownload)
	return a, nil
}

func (a *App) Listen(address string) error { return a.server.Listen(address) }

func (a *App) securityMiddleware(c *fiber.Ctx) error {
	remote := net.ParseIP(c.Context().RemoteIP().String())
	if remote == nil || !remote.IsLoopback() {
		return fiber.ErrForbidden
	}
	host := strings.ToLower(c.Get(fiber.HeaderHost))
	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		host = parsedHost
	}
	host = strings.Trim(host, "[]")
	if host != "127.0.0.1" && host != "localhost" {
		return fiber.ErrForbidden
	}
	if origin := c.Get(fiber.HeaderOrigin); origin != "" {
		parsed, err := url.Parse(origin)
		if err != nil || (parsed.Hostname() != "127.0.0.1" && parsed.Hostname() != "localhost") {
			return fiber.ErrForbidden
		}
	}
	c.Set("Cache-Control", "no-store")
	c.Set("X-Content-Type-Options", "nosniff")
	c.Set("X-Frame-Options", "DENY")
	c.Set("Referrer-Policy", "no-referrer")
	c.Set("Content-Security-Policy", "default-src 'none'; script-src 'nonce-"+a.nonce+"'; style-src 'nonce-"+a.nonce+"'; img-src https: data:; connect-src 'self'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
	return c.Next()
}

func (a *App) requireToken(c *fiber.Ctx) error {
	if c.Get("X-TubeDown-Token") != a.token {
		return fiber.ErrForbidden
	}
	return c.Next()
}

func (a *App) home(c *fiber.Ctx) error {
	c.Type("html", "utf-8")
	html := strings.NewReplacer("{{TOKEN}}", a.token, "{{NONCE}}", a.nonce, "{{DOWNLOAD_DIR}}", a.downloadDir).Replace(localHTML)
	return c.SendString(html)
}

func (a *App) metadata(c *fiber.Ctx) error {
	var req model.MetadataRequest
	if err := c.BodyParser(&req); err != nil || strings.TrimSpace(req.URL) == "" {
		return service.Error{Status: fiber.StatusBadRequest, Code: "BAD_REQUEST", Message: "URL을 확인해 주세요."}
	}
	metadata, err := a.ytdlp.Metadata(context.Background(), strings.TrimSpace(req.URL))
	if err != nil {
		return err
	}
	return c.JSON(metadata)
}

func (a *App) startDownload(c *fiber.Ctx) error {
	var req downloadRequest
	if err := c.BodyParser(&req); err != nil {
		return service.Error{Status: fiber.StatusBadRequest, Code: "BAD_REQUEST", Message: "요청을 확인해 주세요."}
	}
	if !strings.HasPrefix(req.FormatID, "quality-") || strings.TrimSpace(req.URL) == "" {
		return service.Error{Status: fiber.StatusBadRequest, Code: "BAD_REQUEST", Message: "URL과 화질을 확인해 주세요."}
	}

	a.mu.Lock()
	if a.job != nil && (a.job.Status == "queued" || a.job.Status == "downloading") {
		a.mu.Unlock()
		return service.Error{Status: fiber.StatusConflict, Code: "DOWNLOAD_BUSY", Message: "이미 다운로드가 진행 중입니다."}
	}
	id, err := randomToken(9)
	if err != nil {
		a.mu.Unlock()
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel
	a.job = &Job{
		ID: id, Title: req.Title, Quality: req.FormatID, Status: "queued",
		OutputDir: a.downloadDir, StartedAt: time.Now().Format(time.RFC3339),
	}
	jobCopy := *a.job
	a.mu.Unlock()

	go a.runDownload(ctx, id, strings.TrimSpace(req.URL), req.FormatID)
	return c.Status(fiber.StatusAccepted).JSON(jobCopy)
}

func (a *App) runDownload(ctx context.Context, id, rawURL, formatID string) {
	a.updateJob(id, func(job *Job) { job.Status = "downloading" })
	output := filepath.Join(a.downloadDir, "%(title)s [%(id)s].%(ext)s")
	err := a.ytdlp.DownloadLocal(ctx, rawURL, formatID, output, func(progress float64) {
		a.updateJob(id, func(job *Job) { job.Progress = progress })
	})
	a.updateJob(id, func(job *Job) {
		job.FinishedAt = time.Now().Format(time.RFC3339)
		if err != nil {
			job.Status = "failed"
			job.Error = err.Error()
			if errors.Is(ctx.Err(), context.Canceled) {
				job.Status = "cancelled"
				job.Error = "다운로드를 취소했습니다."
			}
			return
		}
		job.Status = "complete"
		job.Progress = 100
	})
}

func (a *App) updateJob(id string, update func(*Job)) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.job != nil && a.job.ID == id {
		update(a.job)
	}
}

func (a *App) getJob(c *fiber.Ctx) error {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.job == nil {
		return c.JSON(fiber.Map{"status": "idle"})
	}
	copy := *a.job
	return c.JSON(copy)
}

func (a *App) cancelDownload(c *fiber.Ctx) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cancel != nil && a.job != nil && (a.job.Status == "queued" || a.job.Status == "downloading") {
		a.cancel()
		return c.SendStatus(fiber.StatusNoContent)
	}
	return service.Error{Status: fiber.StatusConflict, Code: "NO_ACTIVE_DOWNLOAD", Message: "진행 중인 다운로드가 없습니다."}
}

func (a *App) errorHandler(c *fiber.Ctx, err error) error {
	var svcErr service.Error
	if errors.As(err, &svcErr) {
		return c.Status(svcErr.Status).JSON(model.ErrorResponse{Error: model.ErrorBody{Code: svcErr.Code, Message: svcErr.Message}})
	}
	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) {
		return c.Status(fiberErr.Code).JSON(model.ErrorResponse{Error: model.ErrorBody{Code: "HTTP_ERROR", Message: fiberErr.Message}})
	}
	return c.Status(fiber.StatusInternalServerError).JSON(model.ErrorResponse{Error: model.ErrorBody{Code: "INTERNAL_ERROR", Message: "내부 오류가 발생했습니다."}})
}

func randomToken(size int) (string, error) {
	buffer := make([]byte, size)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}
