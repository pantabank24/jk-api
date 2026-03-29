package v1

import (
	"jk-api/config"
	"jk-api/internal/middleware"
	notifCtrl "jk-api/internal/module/notification/controller"
	notifRepo "jk-api/internal/module/notification/repository"
	notifUC "jk-api/internal/module/notification/usecase"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func SetupNotificationRoutes(v1 fiber.Router, db *gorm.DB, cfg *config.Config) {
	repo := notifRepo.NewNotificationRepository(db)
	uc := notifUC.NewNotificationUsecase(repo)
	ctrl := notifCtrl.NewNotificationController(uc)

	notifs := v1.Group("/notifications", middleware.AuthMiddleware(cfg))
	{
		notifs.Get("/", ctrl.GetNotifications)
		notifs.Get("/unread-count", ctrl.CountUnread)
		notifs.Put("/:id/read", ctrl.MarkRead)
		notifs.Put("/read-all", ctrl.MarkAllRead)
	}
}
