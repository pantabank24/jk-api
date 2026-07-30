package v1

import (
	"jk-api/config"
	"jk-api/internal/middleware"
	billRepo "jk-api/internal/module/bill/repository"
	billUC "jk-api/internal/module/bill/usecase"
	configRepo "jk-api/internal/module/config/repository"
	goldPriceRepo "jk-api/internal/module/gold_price/repository"
	logRepo "jk-api/internal/module/log/repository"
	metalPriceRepo "jk-api/internal/module/metal_price/repository"
	notifRepo "jk-api/internal/module/notification/repository"
	sellOrderRepo "jk-api/internal/module/sell_order/repository"
	"jk-api/internal/service"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func SetupV1Routes(api fiber.Router, db *gorm.DB, cfg *config.Config, cronSvc *service.GoldPriceCron, sellEngine *service.SellOrderEngine) {
	v1 := api.Group("/v1")

	// Shared log repository
	lRepo := logRepo.NewLogRepository(db)

	// Activity logger middleware
	v1.Use(middleware.ActivityLogger(lRepo))

	// Public routes
	SetupAuthRoutes(v1, db, cfg, lRepo)
	SetupPublicRoutes(v1, db, cfg)

	// Protected routes
	SetupUserRoutes(v1, db, cfg)
	SetupStoreRoutes(v1, db, cfg)
	SetupBranchRoutes(v1, db, cfg)
	SetupMemberRoutes(v1, db, cfg)
	SetupQuotationRoutes(v1, db, cfg)
	SetupBillRoutes(v1, db, cfg)
	SetupCustomerRoutes(v1, db, cfg)
	SetupRoleRoutes(v1, db, cfg)
	SetupLogRoutes(v1, db, cfg, lRepo)
	SetupDashboardRoutes(v1, db, cfg)

	// New modules
	SetupGoldTypeRoutes(v1, db, cfg)
	SetupGoldPriceRoutes(v1, db, cfg)
	SetupMetalPriceRoutes(v1, db, cfg)
	SetupConfigRoutes(v1, db, cfg, cronSvc, sellEngine)
	SetupSalesScheduleRoutes(v1, db, cfg)
	SetupCustomWeightScheduleRoutes(v1, db, cfg)
	SetupNotificationRoutes(v1, db, cfg)
	SetupNewsRoutes(v1, db, cfg)
	SetupLineRoutes(v1, db, cfg)
	SetupBankRoutes(v1, db, cfg)
	SetupSellOrderRoutes(v1, db, cfg, sellEngine)
}

// NewCronService creates and starts the gold price cron service.
func NewCronService(db *gorm.DB) *service.GoldPriceCron {
	priceRepo := goldPriceRepo.NewGoldPriceRepository(db)
	metalRepo := metalPriceRepo.NewMetalPriceRepository(db)
	cfgRepo := configRepo.NewConfigRepository(db)
	return service.NewGoldPriceCron(priceRepo, metalRepo, cfgRepo)
}

// NewSellOrderEngine wires the auto-sell engine. It fills orders by going through
// the same bill usecase the sell screens post to, so an automatic sale and a manual
// one produce the same bill in the same way.
func NewSellOrderEngine(db *gorm.DB) *service.SellOrderEngine {
	bRepo := billRepo.NewBillRepository(db)
	bbRepo := billRepo.NewBillBalanceRepository(db)
	nRepo := notifRepo.NewNotificationRepository(db)
	return service.NewSellOrderEngine(
		db,
		sellOrderRepo.NewSellOrderRepository(db),
		billUC.NewBillUsecase(bRepo, bbRepo, nRepo),
		nRepo,
	)
}
