package router

import (
	"jk-api/config"
	v1 "jk-api/internal/router/v1"
	"jk-api/internal/service"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func SetupRoutes(app *fiber.App, db *gorm.DB, cfg *config.Config, cronSvc *service.GoldPriceCron) {
	api := app.Group("/api")
	v1.SetupV1Routes(api, db, cfg, cronSvc)
}
