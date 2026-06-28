package v1

import (
	"jk-api/config"
	"jk-api/internal/middleware"
	metalPriceCtrl "jk-api/internal/module/metal_price/controller"
	metalPriceRepo "jk-api/internal/module/metal_price/repository"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func SetupMetalPriceRoutes(v1 fiber.Router, db *gorm.DB, cfg *config.Config) {
	repo := metalPriceRepo.NewMetalPriceRepository(db)
	ctrl := metalPriceCtrl.NewMetalPriceController(repo)

	mp := v1.Group("/metal-prices", middleware.AuthMiddleware(cfg))
	{
		// All authenticated users can read the current price
		mp.Get("/latest", ctrl.GetLatest)
		mp.Get("/history", middleware.RequirePermission(db, "metal_prices.read"), ctrl.GetHistory)
		// Manual fetch from the auto feed
		mp.Post("/fetch", middleware.RequirePermission(db, "metal_prices.create"), ctrl.FetchAndSave)
		// Manually enter a price (fallback when the feed is offline)
		mp.Post("/manual", middleware.RequirePermission(db, "metal_prices.create"), ctrl.SetManual)
	}
}
