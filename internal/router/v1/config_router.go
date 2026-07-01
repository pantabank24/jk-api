package v1

import (
	"jk-api/config"
	"jk-api/internal/middleware"
	configCtrl "jk-api/internal/module/config/controller"
	configRepo "jk-api/internal/module/config/repository"
	"jk-api/internal/service"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func SetupConfigRoutes(v1 fiber.Router, db *gorm.DB, cfg *config.Config, cronSvc *service.GoldPriceCron) {
	repo := configRepo.NewConfigRepository(db)
	ctrl := configCtrl.NewConfigController(repo, cronSvc, db)

	cfgGroup := v1.Group("/configs", middleware.AuthMiddleware(cfg))
	{
		// Auth-only (no config.read): any user can check whether sales are open.
		cfgGroup.Get("/sales-status", ctrl.GetSalesStatus)
		// Auth-only: any user can check whether typing the weight is allowed.
		cfgGroup.Get("/custom-weight-status", ctrl.GetCustomWeightStatus)
		// Auth-only: any user can check whether customer bill creation is open.
		cfgGroup.Get("/bills-status", ctrl.GetBillsOpenStatus)
		cfgGroup.Get("/", middleware.RequirePermission(db, "config.read"), ctrl.GetAll)
		cfgGroup.Put("/", middleware.RequirePermission(db, "config.update"), ctrl.Update)
	}
}
