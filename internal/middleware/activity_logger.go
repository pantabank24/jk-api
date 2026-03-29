package middleware

import (
	"strings"
	"time"

	"jk-api/internal/entity"
	logRepo "jk-api/internal/module/log/repository"

	"github.com/gofiber/fiber/v2"
)

// ActivityLogger records every API request asynchronously.
// Skips /health and /uploads/* paths.
func ActivityLogger(repo logRepo.LogRepository) fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()

		// Run the actual handler first
		err := c.Next()

		path := c.Path()
		if path == "/health" || strings.HasPrefix(path, "/uploads") {
			return err
		}

		durationMs := time.Since(start).Milliseconds()

		// Copy values before spawning goroutine (fiber context is pooled)
		method     := c.Method()
		statusCode := c.Response().StatusCode()
		ip         := c.IP()
		userAgent  := c.Get("User-Agent")

		var userIDPtr *uint
		if uid := GetUserID(c); uid != 0 {
			uid := uid
			userIDPtr = &uid
		}

		log := &entity.ActivityLog{
			UserID:     userIDPtr,
			Method:     method,
			Path:       path,
			StatusCode: statusCode,
			IP:         ip,
			UserAgent:  userAgent,
			DurationMs: durationMs,
		}

		go func() { _ = repo.CreateActivityLog(log) }()

		return err
	}
}
