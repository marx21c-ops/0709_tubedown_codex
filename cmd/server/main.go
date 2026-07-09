package main

import (
	"os"
	"strconv"
	"time"

	"downloader-2607/internal/handler"
	"downloader-2607/internal/middleware"
	"downloader-2607/internal/service"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	zerolog.TimeFieldFormat = time.RFC3339
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339})

	app := fiber.New(fiber.Config{
		AppName:      "downloader-2607",
		ErrorHandler: handler.ErrorHandler,
	})

	app.Use(middleware.RequestLogger())

	app.Get("/", handler.Home())

	ytdlp := service.NewYTDLP(service.Config{
		Binary:          env("YTDLP_BINARY", "yt-dlp"),
		MetadataTimeout: durationEnv("METADATA_TIMEOUT", 30*time.Second),
		DownloadTimeout: durationEnv("DOWNLOAD_TIMEOUT", 30*time.Minute),
		Proxy:           env("YTDLP_PROXY", ""),
		CookiesFile:     env("YTDLP_COOKIES_FILE", ""),
		JSRuntime:       env("YTDLP_JS_RUNTIME", "node"),
		Impersonate:     env("YTDLP_IMPERSONATE", "chrome"),
	})

	api := app.Group("/api/v1")
	api.Post("/metadata", handler.Metadata(ytdlp))

	downloadLimit := intEnv("RATE_LIMIT_PER_IP", 2)
	api.Get("/download", middleware.ConcurrentLimit(downloadLimit), handler.Download(ytdlp))

	app.Get("/healthz", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"ok": true})
	})

	port := env("PORT", "8080")
	log.Info().Str("port", port).Msg("server starting")
	if err := app.Listen(":" + port); err != nil {
		log.Fatal().Err(err).Msg("server stopped")
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func intEnv(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return fallback
	}
	return parsed
}

func durationEnv(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}
