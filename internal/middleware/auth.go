package middleware

import (
	"encoding/json"
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

// SetActivityDescription lets a controller attach a human-readable summary of
// the business action it just performed (e.g. "อนุมัติใบเสนอราคา P2607001"),
// picked up by the ActivityLogger middleware when it persists the log row.
// Optional — routes that don't call this just log raw method+path as before.
func SetActivityDescription(c *fiber.Ctx, desc string) {
	c.Locals("activity_description", desc)
}

// GetActivityDescription reads back what SetActivityDescription stored, or ""
// if the route never set one.
func GetActivityDescription(c *fiber.Ctx) string {
	if d, ok := c.Locals("activity_description").(string); ok {
		return d
	}
	return ""
}

// SetActivityTarget records WHOM the action was about — the customer whose bill
// was touched — as opposed to the caller, who is logged automatically. Without
// it, every step a staff member performs on a customer's bill (ออกบิล, อนุมัติ,
// ยกเลิก, ลบ) lands only on the staff member's timeline and the customer's trail
// stops at their own click. Ignored when userID is 0.
func SetActivityTarget(c *fiber.Ctx, userID uint) {
	if userID == 0 {
		return
	}
	c.Locals("activity_target_user_id", userID)
}

// GetActivityTarget reads back SetActivityTarget, or nil if unset.
func GetActivityTarget(c *fiber.Ctx) *uint {
	if id, ok := c.Locals("activity_target_user_id").(uint); ok && id != 0 {
		return &id
	}
	return nil
}

// SetActivityRef records the document code the action touched (bill/quotation).
func SetActivityRef(c *fiber.Ctx, code string) {
	c.Locals("activity_ref_code", code)
}

// GetActivityRef reads back SetActivityRef, or "" if unset.
func GetActivityRef(c *fiber.Ctx) string {
	if code, ok := c.Locals("activity_ref_code").(string); ok {
		return code
	}
	return ""
}

// SetActivityDetail attaches a structured snapshot of the action (marshalled to
// JSON) to the log row — the per-item price/weight/total a customer clicked, for
// instance. Kept separate from the description so the frontend can render it as
// a table and so it survives later edits to the bill it describes. A value that
// fails to marshal is dropped rather than failing the request: the log is a
// side effect and must never break the action it records.
func SetActivityDetail(c *fiber.Ctx, v any) {
	if v == nil {
		return
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return
	}
	c.Locals("activity_detail", json.RawMessage(raw))
}

// GetActivityDetail reads back SetActivityDetail, or nil if unset.
func GetActivityDetail(c *fiber.Ctx) json.RawMessage {
	if d, ok := c.Locals("activity_detail").(json.RawMessage); ok {
		return d
	}
	return nil
}
