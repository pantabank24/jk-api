package v1

import (
	"jk-api/config"
	"jk-api/internal/middleware"
	goldPriceCtrl "jk-api/internal/module/gold_price/controller"
	goldPriceRepo "jk-api/internal/module/gold_price/repository"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func SetupGoldPriceRoutes(v1 fiber.Router, db *gorm.DB, cfg *config.Config) {
	repo := goldPriceRepo.NewGoldPriceRepository(db)
	ctrl := goldPriceCtrl.NewGoldPriceController(repo)

	gp := v1.Group("/gold-prices", middleware.AuthMiddleware(cfg))
	{
		// All authenticated users can read current price
		gp.Get("/latest", ctrl.GetLatest)
		gp.Get("/history", middleware.RequirePermission(db, "gold_prices.read"), ctrl.GetHistory)
		// Manual fetch — master only
		gp.Post("/fetch", middleware.RequirePermission(db, "gold_prices.create"), ctrl.FetchAndSave)
	}
}
