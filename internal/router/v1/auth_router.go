package v1

import (
	"time"

	"jk-api/config"
	"jk-api/internal/middleware"
	authCtrl "jk-api/internal/module/auth/controller"
	authRepo "jk-api/internal/module/auth/repository"
	authUC "jk-api/internal/module/auth/usecase"
	logRepo "jk-api/internal/module/log/repository"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func SetupAuthRoutes(v1 fiber.Router, db *gorm.DB, cfg *config.Config, lRepo logRepo.LogRepository) {
	repo := authRepo.NewAuthRepository(db)

	expiry, err := time.ParseDuration(cfg.JWTExpiresIn)
	if err != nil {
		expiry = 24 * time.Hour
	}

	uc := authUC.NewAuthUsecase(repo, cfg.JWTSecret, expiry)
	ctrl := authCtrl.NewAuthController(uc, lRepo)

	auth := v1.Group("/auth")
	{
		auth.Post("/login", ctrl.Login)

		// Protected routes
		protected := auth.Group("", middleware.AuthMiddleware(cfg))
		protected.Get("/me", ctrl.GetMe)
		protected.Post("/refresh", ctrl.RefreshToken)
	}
}
