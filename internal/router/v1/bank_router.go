package v1

import (
	"jk-api/config"
	"jk-api/internal/middleware"
	bankCtrl "jk-api/internal/module/bank/controller"
	bankRepo "jk-api/internal/module/bank/repository"
	bankUC "jk-api/internal/module/bank/usecase"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func SetupBankRoutes(v1 fiber.Router, db *gorm.DB, cfg *config.Config) {
	repo := bankRepo.NewBankRepository(db)
	uc := bankUC.NewBankUsecase(repo)
	ctrl := bankCtrl.NewBankController(uc)

	b := v1.Group("/banks", middleware.AuthMiddleware(cfg))
	{
		// Any authenticated user can read — the customer form needs the selector.
		b.Get("/", ctrl.GetAll)
		b.Get("/:id", ctrl.GetByID)
		// Managing the list is master/owner only (see migration 000078).
		b.Post("/", middleware.RequirePermission(db, "banks.create"), ctrl.Create)
		b.Put("/:id", middleware.RequirePermission(db, "banks.update"), ctrl.Update)
		b.Delete("/:id", middleware.RequirePermission(db, "banks.delete"), ctrl.Delete)
	}
}
