package handler

import (
	"strings"

	"downloader-2607/internal/model"
	"downloader-2607/internal/service"

	"github.com/gofiber/fiber/v2"
)

func Metadata(ytdlp *service.YTDLP) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req model.MetadataRequest
		if err := c.BodyParser(&req); err != nil {
			return AppError{Status: fiber.StatusBadRequest, Code: "BAD_REQUEST", Message: "invalid JSON body"}
		}

		req.URL = strings.TrimSpace(req.URL)
		if req.URL == "" {
			return AppError{Status: fiber.StatusBadRequest, Code: "INVALID_URL", Message: "url is required"}
		}

		metadata, err := ytdlp.Metadata(c.Context(), req.URL)
		if err != nil {
			return err
		}

		return c.JSON(metadata)
	}
}
