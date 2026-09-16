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

		// Fiber's strings point into its pooled request buffer, and the row below is
		// written from another goroutine after that buffer has gone to the next
		// request — which is how logs got a method of "OPT" and paths spliced from
		// two requests. Assigning a string does not copy it; strings.Clone does.
		path := strings.Clone(c.Path())
		if path == "/health" || strings.HasPrefix(path, "/uploads") {
			return err
		}

		durationMs := time.Since(start).Milliseconds()

		method      := strings.Clone(c.Method())
		statusCode  := c.Response().StatusCode()
		ip          := strings.Clone(c.IP())
		userAgent   := strings.Clone(c.Get("User-Agent"))
		description := strings.Clone(GetActivityDescription(c))
		targetUser  := GetActivityTarget(c)
		refCode     := strings.Clone(GetActivityRef(c))
		detail      := GetActivityDetail(c)

		var userIDPtr *uint
		if uid := GetUserID(c); uid != 0 {
			uid := uid
			userIDPtr = &uid
		}

		log := &entity.ActivityLog{
			UserID:       userIDPtr,
			TargetUserID: targetUser,
			Method:       method,
			Path:         path,
			Description:  description,
			RefCode:      refCode,
			Detail:       detail,
			StatusCode:   statusCode,
			IP:           ip,
			UserAgent:    userAgent,
			DurationMs:   durationMs,
		}

		go func() { _ = repo.CreateActivityLog(log) }()

		return err
	}
}
