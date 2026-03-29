package v1

import (
	"jk-api/config"
	"jk-api/internal/middleware"
	roleCtrl "jk-api/internal/module/role/controller"
	roleRepo "jk-api/internal/module/role/repository"
	roleUC "jk-api/internal/module/role/usecase"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func SetupRoleRoutes(v1 fiber.Router, db *gorm.DB, cfg *config.Config) {
	repo := roleRepo.NewRoleRepository(db)
	uc := roleUC.NewRoleUsecase(repo)
	ctrl := roleCtrl.NewRoleController(uc)

	roles := v1.Group("/roles", middleware.AuthMiddleware(cfg))
	{
		roles.Get("/", middleware.RequirePermission(db, "roles.read"), ctrl.GetAllRoles)
		roles.Get("/:id", middleware.RequirePermission(db, "roles.read"), ctrl.GetRoleByID)
		roles.Post("/", middleware.RequirePermission(db, "roles.create"), ctrl.CreateRole)
		roles.Put("/:id", middleware.RequirePermission(db, "roles.update"), ctrl.UpdateRole)
		roles.Delete("/:id", middleware.RequirePermission(db, "roles.delete"), ctrl.DeleteRole)
		roles.Put("/:id/permissions", middleware.RequirePermission(db, "roles.update"), ctrl.SetRolePermissions)
	}

	// Permissions list
	permissions := v1.Group("/permissions", middleware.AuthMiddleware(cfg))
	{
		permissions.Get("/", middleware.RequirePermission(db, "roles.read"), ctrl.GetAllPermissions)
	}
}
