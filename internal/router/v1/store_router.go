package v1

import (
	"jk-api/config"
	"jk-api/internal/middleware"
	storeCtrl "jk-api/internal/module/store/controller"
	storeRepo "jk-api/internal/module/store/repository"
	storeUC "jk-api/internal/module/store/usecase"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func SetupStoreRoutes(v1 fiber.Router, db *gorm.DB, cfg *config.Config) {
	repo := storeRepo.NewStoreRepository(db)
	uc := storeUC.NewStoreUsecase(repo)
	ctrl := storeCtrl.NewStoreController(uc)

	stores := v1.Group("/stores", middleware.AuthMiddleware(cfg))
	{
		stores.Get("/", middleware.RequirePermission(db, "stores.read"), ctrl.GetAllStores)
		stores.Get("/:id", middleware.RequirePermission(db, "stores.read"), ctrl.GetStoreByID)
		stores.Post("/", middleware.RequirePermission(db, "stores.create"), ctrl.CreateStore)
		stores.Put("/:id", middleware.RequirePermission(db, "stores.update"), ctrl.UpdateStore)
		stores.Delete("/:id", middleware.RequirePermission(db, "stores.delete"), ctrl.DeleteStore)
		stores.Post("/:id/logo", middleware.RequirePermission(db, "stores.update"), ctrl.UploadLogo)
	}
}
