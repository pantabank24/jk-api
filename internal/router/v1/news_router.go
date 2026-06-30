package v1

import (
	"jk-api/config"
	"jk-api/internal/middleware"
	newsCtrl "jk-api/internal/module/news/controller"
	newsRepo "jk-api/internal/module/news/repository"
	newsUC "jk-api/internal/module/news/usecase"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func SetupNewsRoutes(v1 fiber.Router, db *gorm.DB, cfg *config.Config) {
	repo := newsRepo.NewNewsRepository(db)
	uc := newsUC.NewNewsUsecase(repo)
	ctrl := newsCtrl.NewNewsController(uc)

	news := v1.Group("/news", middleware.AuthMiddleware(cfg))
	{
		// Any authenticated user can see the news visible to their role (home page).
		news.Get("/visible", ctrl.GetVisibleNews)
		news.Get("/", middleware.RequirePermission(db, "news.read"), ctrl.GetAllNews)
		news.Get("/:id", middleware.RequirePermission(db, "news.read"), ctrl.GetNewsByID)
		news.Post("/", middleware.RequirePermission(db, "news.create"), ctrl.CreateNews)
		news.Put("/:id", middleware.RequirePermission(db, "news.update"), ctrl.UpdateNews)
		news.Delete("/:id", middleware.RequirePermission(db, "news.delete"), ctrl.DeleteNews)
		news.Post("/:id/image", middleware.RequirePermission(db, "news.update"), ctrl.UploadImage)
	}
}
