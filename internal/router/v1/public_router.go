package v1

import (
	"jk-api/config"
	"jk-api/internal/middleware"
	goldPriceCtrl "jk-api/internal/module/gold_price/controller"
	goldPriceRepo "jk-api/internal/module/gold_price/repository"
	metalPriceCtrl "jk-api/internal/module/metal_price/controller"
	metalPriceRepo "jk-api/internal/module/metal_price/repository"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// SetupPublicRoutes exposes read-only current prices to first-party frontends
// that have no logged-in user (jk-goldtrader). Guarded by X-API-Key instead of
// JWT. Read-only on purpose: no fetch/manual endpoints live here.
func SetupPublicRoutes(v1 fiber.Router, db *gorm.DB, cfg *config.Config) {
	goldCtrl := goldPriceCtrl.NewGoldPriceController(
		goldPriceRepo.NewGoldPriceRepository(db),
		cfg.GoldRealtimeURL,
	)
	metalCtrl := metalPriceCtrl.NewMetalPriceController(
		metalPriceRepo.NewMetalPriceRepository(db),
	)

	pub := v1.Group("/public", middleware.APIKeyMiddleware(cfg))
	{
		// ราคาสมาคม (auto) เท่านั้น — ไม่เอา manual override ของร้านใดร้านหนึ่ง
		// เพราะทุกหน้าร้าน (jk, chinracha, kk, pw, wachara, aelomthong) ใช้เส้นนี้ร่วมกัน
		pub.Get("/gold-prices/latest", goldCtrl.GetLatestAssociation)
		pub.Get("/metal-prices/latest", metalCtrl.GetLatest)
	}
}
