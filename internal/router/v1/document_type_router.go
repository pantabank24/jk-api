package v1

import (
	"jk-api/config"
	"jk-api/internal/middleware"
	docTypeCtrl "jk-api/internal/module/document_type/controller"
	docTypeRepo "jk-api/internal/module/document_type/repository"
	docTypeUC "jk-api/internal/module/document_type/usecase"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func SetupDocumentTypeRoutes(v1 fiber.Router, db *gorm.DB, cfg *config.Config) {
	repo := docTypeRepo.NewDocumentTypeRepository(db)
	uc := docTypeUC.NewDocumentTypeUsecase(repo)
	ctrl := docTypeCtrl.NewDocumentTypeController(uc)

	t := v1.Group("/document-types", middleware.AuthMiddleware(cfg))
	{
		// Any authenticated user can read — the customer's own อัปโหลดเอกสาร form
		// needs the selector, and customers hold no back-office permissions.
		t.Get("/", ctrl.GetAll)
		t.Get("/:id", ctrl.GetByID)
		// Managing the list is master/owner only (see migration 000093).
		t.Post("/", middleware.RequirePermission(db, "document_types.create"), ctrl.Create)
		t.Put("/:id", middleware.RequirePermission(db, "document_types.update"), ctrl.Update)
		t.Delete("/:id", middleware.RequirePermission(db, "document_types.delete"), ctrl.Delete)
	}
}
