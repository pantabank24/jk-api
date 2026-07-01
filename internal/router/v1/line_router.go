package v1

import (
	"jk-api/config"
	"jk-api/internal/middleware"
	lineCtrl "jk-api/internal/module/line"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func SetupLineRoutes(v1 fiber.Router, db *gorm.DB, cfg *config.Config) {
	ctrl := lineCtrl.NewLineController(db)

	// Public — verified by LINE signature inside the handler
	v1.Post("/line/webhook", ctrl.Webhook)

	// Authenticated
	line := v1.Group("/line", middleware.AuthMiddleware(cfg))
	{
		line.Get("/status",  middleware.RequirePermission(db, "config.read"),   ctrl.Status)
		line.Post("/unlink", middleware.RequirePermission(db, "config.update"), ctrl.Unlink)
	}
}
