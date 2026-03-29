package middleware

import (
	"strings"

	"jk-api/config"
	jwtPkg "jk-api/pkg/jwt"
	"jk-api/pkg/response"

	"github.com/gofiber/fiber/v2"
)

// AuthMiddleware validates JWT token and sets user context
func AuthMiddleware(cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return response.Unauthorized(c, "Missing authorization header")
		}

		// Extract token from "Bearer <token>"
		tokenParts := strings.Split(authHeader, " ")
		if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
			return response.Unauthorized(c, "Invalid authorization format")
		}

		claims, err := jwtPkg.ParseToken(cfg.JWTSecret, tokenParts[1])
		if err != nil {
			return response.Unauthorized(c, "Invalid or expired token")
		}

		// Set user info in context locals
		c.Locals("user_id", claims.UserID)
		c.Locals("store_id", claims.StoreID)
		c.Locals("branch_id", claims.BranchID)
		c.Locals("role_id", claims.RoleID)
		c.Locals("role_name", claims.RoleName)

		return c.Next()
	}
}

// GetUserID extracts user_id from context
func GetUserID(c *fiber.Ctx) uint {
	if id, ok := c.Locals("user_id").(uint); ok {
		return id
	}
	return 0
}

// GetStoreID extracts store_id from context
func GetStoreID(c *fiber.Ctx) *uint {
	if id, ok := c.Locals("store_id").(*uint); ok {
		return id
	}
	return nil
}

// GetBranchID extracts branch_id from context
func GetBranchID(c *fiber.Ctx) *uint {
	if id, ok := c.Locals("branch_id").(*uint); ok {
		return id
	}
	return nil
}

// GetRoleName extracts role_name from context
func GetRoleName(c *fiber.Ctx) string {
	if name, ok := c.Locals("role_name").(string); ok {
		return name
	}
	return ""
}

// IsMaster checks if the current user is a master
func IsMaster(c *fiber.Ctx) bool {
	return GetRoleName(c) == "master"
}
