package middleware

import (
	"jk-api/internal/entity"
	"jk-api/pkg/response"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// RequirePermission checks if the current user's role has the required permission
func RequirePermission(db *gorm.DB, permissionCode string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		roleName := GetRoleName(c)

		// Master always has all permissions
		if roleName == "master" {
			return c.Next()
		}

		roleID, ok := c.Locals("role_id").(uint)
		if !ok || roleID == 0 {
			return response.Forbidden(c, "No role assigned")
		}

		// Check if role has the permission
		var count int64
		db.Model(&entity.RolePermission{}).
			Joins("JOIN permissions ON permissions.id = role_permissions.permission_id").
			Where("role_permissions.role_id = ? AND permissions.code = ?", roleID, permissionCode).
			Count(&count)

		if count == 0 {
			return response.Forbidden(c, "Insufficient permissions")
		}

		return c.Next()
	}
}

// ScopeByStore ensures non-master users can only access their own store's data
// It sets "scope_store_id" in locals for use by handlers
func ScopeByStore() fiber.Handler {
	return func(c *fiber.Ctx) error {
		if IsMaster(c) {
			// Master can access all stores, check query param for filtering
			if storeIDParam := c.Query("store_id"); storeIDParam != "" {
				c.Locals("scope_store_id", storeIDParam)
			}
			return c.Next()
		}

		storeID := GetStoreID(c)
		if storeID == nil {
			return response.Forbidden(c, "No store assigned")
		}

		c.Locals("scope_store_id", *storeID)
		return c.Next()
	}
}

// ScopeByBranch ensures branch/employee users can only access their own branch's data
func ScopeByBranch() fiber.Handler {
	return func(c *fiber.Ctx) error {
		roleName := GetRoleName(c)

		// Master and owner can access all branches in a store
		if roleName == "master" || roleName == "owner" {
			if branchIDParam := c.Query("branch_id"); branchIDParam != "" {
				c.Locals("scope_branch_id", branchIDParam)
			}
			return c.Next()
		}

		branchID := GetBranchID(c)
		if branchID == nil {
			return response.Forbidden(c, "No branch assigned")
		}

		c.Locals("scope_branch_id", *branchID)
		return c.Next()
	}
}
