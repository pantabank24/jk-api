package v1

import (
	"jk-api/config"
	"jk-api/internal/middleware"
	logCtrl "jk-api/internal/module/log/controller"
	logUC "jk-api/internal/module/log/usecase"
	logRepo "jk-api/internal/module/log/repository"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func SetupLogRoutes(v1 fiber.Router, db *gorm.DB, cfg *config.Config, repo logRepo.LogRepository) {
	uc := logUC.NewLogUsecase(repo)
	ctrl := logCtrl.NewLogController(uc)

	logs := v1.Group("/logs", middleware.AuthMiddleware(cfg))
	{
		logs.Get("/login",    middleware.RequirePermission(db, "logs.read"), ctrl.GetLoginLogs)
		logs.Get("/activity", middleware.RequirePermission(db, "logs.read"), ctrl.GetActivityLogs)
		logs.Delete("/login",    middleware.RequirePermission(db, "logs.delete"), ctrl.DeleteOldLoginLogs)
		logs.Delete("/activity", middleware.RequirePermission(db, "logs.delete"), ctrl.DeleteOldActivityLogs)
	}
}
