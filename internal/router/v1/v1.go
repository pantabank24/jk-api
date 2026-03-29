package v1

import (
	"jk-api/config"
	"jk-api/internal/middleware"
	configRepo "jk-api/internal/module/config/repository"
	goldPriceRepo "jk-api/internal/module/gold_price/repository"
	logRepo "jk-api/internal/module/log/repository"
	"jk-api/internal/service"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func SetupV1Routes(api fiber.Router, db *gorm.DB, cfg *config.Config, cronSvc *service.GoldPriceCron) {
	v1 := api.Group("/v1")

	// Shared log repository
	lRepo := logRepo.NewLogRepository(db)

	// Activity logger middleware
	v1.Use(middleware.ActivityLogger(lRepo))

	// Public routes
	SetupAuthRoutes(v1, db, cfg, lRepo)

	// Protected routes
	SetupUserRoutes(v1, db, cfg)
	SetupStoreRoutes(v1, db, cfg)
	SetupBranchRoutes(v1, db, cfg)
	SetupMemberRoutes(v1, db, cfg)
	SetupQuotationRoutes(v1, db, cfg)
	SetupRoleRoutes(v1, db, cfg)
	SetupLogRoutes(v1, db, cfg, lRepo)
	SetupDashboardRoutes(v1, db, cfg)

	// New modules
	SetupGoldTypeRoutes(v1, db, cfg)
	SetupGoldPriceRoutes(v1, db, cfg)
	SetupConfigRoutes(v1, db, cfg, cronSvc)
	SetupNotificationRoutes(v1, db, cfg)
}

// NewCronService creates and starts the gold price cron service.
func NewCronService(db *gorm.DB) *service.GoldPriceCron {
	priceRepo := goldPriceRepo.NewGoldPriceRepository(db)
	cfgRepo := configRepo.NewConfigRepository(db)
	return service.NewGoldPriceCron(priceRepo, cfgRepo)
}
