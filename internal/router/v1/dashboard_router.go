package v1

import (
	"jk-api/config"
	"jk-api/internal/middleware"
	dashboardCtrl "jk-api/internal/module/dashboard/controller"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func SetupDashboardRoutes(v1 fiber.Router, db *gorm.DB, cfg *config.Config) {
	ctrl := dashboardCtrl.NewDashboardController(db)

	dashboard := v1.Group("/dashboard", middleware.AuthMiddleware(cfg))
	{
		dashboard.Get("/stats", ctrl.GetStats)
	}
}
