package middleware

import (
	"crypto/subtle"

	"jk-api/config"
	"jk-api/pkg/response"

	"github.com/gofiber/fiber/v2"
)

// APIKeyMiddleware guards the public read-only routes with a shared secret sent
// in the X-API-Key header. Used by first-party frontends (jk-goldtrader on
// Vercel) that have no logged-in user and therefore no JWT to present.
//
// Fails closed: if PUBLIC_API_KEY is not configured the routes stay shut rather
// than silently becoming open to the internet.
func APIKeyMiddleware(cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if cfg.PublicAPIKey == "" {
			return response.Unauthorized(c, "Public API is not configured")
		}

		key := c.Get("X-API-Key")
		if subtle.ConstantTimeCompare([]byte(key), []byte(cfg.PublicAPIKey)) != 1 {
			return response.Unauthorized(c, "Invalid API key")
		}

		return c.Next()
	}
}
