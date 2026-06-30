package v1

import (
	"jk-api/config"
	"jk-api/internal/middleware"
	salesCtrl "jk-api/internal/module/sales_schedule/controller"
	salesRepo "jk-api/internal/module/sales_schedule/repository"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func SetupSalesScheduleRoutes(v1 fiber.Router, db *gorm.DB, cfg *config.Config) {
	repo := salesRepo.NewSalesScheduleRepository(db)
	ctrl := salesCtrl.NewSalesScheduleController(repo)

	g := v1.Group("/sales-schedules", middleware.AuthMiddleware(cfg))
	{
		g.Get("/", middleware.RequirePermission(db, "config.read"), ctrl.GetAll)
		g.Put("/", middleware.RequirePermission(db, "config.update"), ctrl.Upsert)
		g.Delete("/:id", middleware.RequirePermission(db, "config.update"), ctrl.Delete)
	}
}
