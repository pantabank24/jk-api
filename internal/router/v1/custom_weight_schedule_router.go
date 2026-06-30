package v1

import (
	"jk-api/config"
	"jk-api/internal/middleware"
	cwsCtrl "jk-api/internal/module/custom_weight_schedule/controller"
	cwsRepo "jk-api/internal/module/custom_weight_schedule/repository"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func SetupCustomWeightScheduleRoutes(v1 fiber.Router, db *gorm.DB, cfg *config.Config) {
	repo := cwsRepo.NewCustomWeightScheduleRepository(db)
	ctrl := cwsCtrl.NewCustomWeightScheduleController(repo)

	g := v1.Group("/custom-weight-schedules", middleware.AuthMiddleware(cfg))
	{
		g.Get("/", middleware.RequirePermission(db, "config.read"), ctrl.GetAll)
		g.Put("/", middleware.RequirePermission(db, "config.update"), ctrl.Upsert)
		g.Delete("/:id", middleware.RequirePermission(db, "config.update"), ctrl.Delete)
	}
}
