package middleware

import (
	"jk-api/internal/entity"
	"jk-api/pkg/response"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// HasPermission checks whether the current request's role has a given permission code.
// Master always returns true. Does a single COUNT query for other roles.
func HasPermission(db *gorm.DB, c *fiber.Ctx, code string) bool {
	if GetRoleName(c) == "master" {
		return true
	}
	return HasPermissionStrict(db, c, code)
}

// HasPermissionStrict checks whether the role has the permission via a pure DB lookup,
// without granting master automatic access. Use this for permissions that represent
// constraints or required behaviors rather than privileges (e.g. credits.use).
func HasPermissionStrict(db *gorm.DB, c *fiber.Ctx, code string) bool {
	roleID, ok := c.Locals("role_id").(uint)
	if !ok || roleID == 0 {
		return false
	}
	var count int64
	db.Model(&entity.RolePermission{}).
		Joins("JOIN permissions ON permissions.id = role_permissions.permission_id").
		Where("role_permissions.role_id = ? AND permissions.code = ?", roleID, code).
		Count(&count)
	return count > 0
}

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

// RequireAnyPermission passes when the current user's role has AT LEAST ONE of
// the given permission codes. Master always passes. Used by routes that serve
// more than one role (e.g. bill creation: customers hold bills.create, staff
// hold bills.sell).
func RequireAnyPermission(db *gorm.DB, permissionCodes ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if GetRoleName(c) == "master" {
			return c.Next()
		}

		roleID, ok := c.Locals("role_id").(uint)
		if !ok || roleID == 0 {
			return response.Forbidden(c, "No role assigned")
		}

		var count int64
		db.Model(&entity.RolePermission{}).
			Joins("JOIN permissions ON permissions.id = role_permissions.permission_id").
			Where("role_permissions.role_id = ? AND permissions.code IN ?", roleID, permissionCodes).
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
