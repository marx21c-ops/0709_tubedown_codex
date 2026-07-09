package handler

import (
	"errors"

	"downloader-2607/internal/model"
	"downloader-2607/internal/service"

	"github.com/gofiber/fiber/v2"
)

type AppError struct {
	Status  int
	Code    string
	Message string
}

func (e AppError) Error() string {
	return e.Message
}

func ErrorHandler(c *fiber.Ctx, err error) error {
	var appErr AppError
	if errors.As(err, &appErr) {
		return c.Status(appErr.Status).JSON(model.ErrorResponse{
			Error: model.ErrorBody{Code: appErr.Code, Message: appErr.Message},
		})
	}

	var svcErr service.Error
	if errors.As(err, &svcErr) {
		return c.Status(svcErr.Status).JSON(model.ErrorResponse{
			Error: model.ErrorBody{Code: svcErr.Code, Message: svcErr.Message},
		})
	}

	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) {
		return c.Status(fiberErr.Code).JSON(model.ErrorResponse{
			Error: model.ErrorBody{Code: "HTTP_ERROR", Message: fiberErr.Message},
		})
	}

	return c.Status(fiber.StatusInternalServerError).JSON(model.ErrorResponse{
		Error: model.ErrorBody{Code: "INTERNAL_ERROR", Message: "internal server error"},
	})
}
