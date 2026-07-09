package middleware

import (
	"sync"

	"downloader-2607/internal/model"

	"github.com/gofiber/fiber/v2"
)

func ConcurrentLimit(limit int) fiber.Handler {
	if limit < 1 {
		limit = 1
	}

	var mu sync.Mutex
	active := make(map[string]int)

	return func(c *fiber.Ctx) error {
		ip := c.IP()

		mu.Lock()
		if active[ip] >= limit {
			mu.Unlock()
			return c.Status(fiber.StatusTooManyRequests).JSON(model.ErrorResponse{
				Error: model.ErrorBody{Code: "RATE_LIMITED", Message: "too many concurrent downloads from this IP"},
			})
		}
		active[ip]++
		mu.Unlock()

		defer func() {
			mu.Lock()
			active[ip]--
			if active[ip] <= 0 {
				delete(active, ip)
			}
			mu.Unlock()
		}()

		return c.Next()
	}
}
