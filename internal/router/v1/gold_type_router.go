package v1

import (
	"jk-api/config"
	"jk-api/internal/middleware"
	goldTypeCtrl "jk-api/internal/module/gold_type/controller"
	goldTypeRepo "jk-api/internal/module/gold_type/repository"
	goldTypeUC "jk-api/internal/module/gold_type/usecase"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func SetupGoldTypeRoutes(v1 fiber.Router, db *gorm.DB, cfg *config.Config) {
	repo := goldTypeRepo.NewGoldTypeRepository(db)
	uc := goldTypeUC.NewGoldTypeUsecase(repo)
	ctrl := goldTypeCtrl.NewGoldTypeController(uc)

	gt := v1.Group("/gold-types", middleware.AuthMiddleware(cfg))
	{
		// All authenticated users can read (needed for quotation calculate page)
		gt.Get("/", ctrl.GetAll)
		gt.Get("/:id", ctrl.GetByID)
		// Only master can manage
		gt.Post("/", middleware.RequirePermission(db, "gold_types.create"), ctrl.Create)
		gt.Put("/:id", middleware.RequirePermission(db, "gold_types.update"), ctrl.Update)
		gt.Delete("/:id", middleware.RequirePermission(db, "gold_types.delete"), ctrl.Delete)
	}
}
