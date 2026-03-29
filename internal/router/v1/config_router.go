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
	ctrl := configCtrl.NewConfigController(repo, cronSvc)

	cfgGroup := v1.Group("/configs", middleware.AuthMiddleware(cfg))
	{
		cfgGroup.Get("/", middleware.RequirePermission(db, "config.read"), ctrl.GetAll)
		cfgGroup.Put("/", middleware.RequirePermission(db, "config.update"), ctrl.Update)
	}
}
