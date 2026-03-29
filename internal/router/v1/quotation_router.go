package v1

import (
	"jk-api/config"
	"jk-api/internal/middleware"
	memberRepo "jk-api/internal/module/member/repository"
	notifRepo "jk-api/internal/module/notification/repository"
	quotationCtrl "jk-api/internal/module/quotation/controller"
	quotationRepo "jk-api/internal/module/quotation/repository"
	quotationUC "jk-api/internal/module/quotation/usecase"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func SetupQuotationRoutes(v1 fiber.Router, db *gorm.DB, cfg *config.Config) {
	qRepo := quotationRepo.NewQuotationRepository(db)
	mRepo := memberRepo.NewMemberRepository(db)
	nRepo := notifRepo.NewNotificationRepository(db)
	uc := quotationUC.NewQuotationUsecase(qRepo, mRepo, nRepo)
	ctrl := quotationCtrl.NewQuotationController(uc)

	quotations := v1.Group("/quotations", middleware.AuthMiddleware(cfg))
	{
		quotations.Get("/",       middleware.RequirePermission(db, "quotations.read"),   ctrl.GetAllQuotations)
		quotations.Get("/:id",    middleware.RequirePermission(db, "quotations.read"),   ctrl.GetQuotationByID)
		quotations.Post("/",      middleware.RequirePermission(db, "quotations.create"), ctrl.CreateQuotation)
		quotations.Put("/:id",    middleware.RequirePermission(db, "quotations.update"), ctrl.UpdateQuotationStatus)   // status change
		quotations.Patch("/:id",  middleware.RequirePermission(db, "quotations.update"), ctrl.UpdateQuotation)          // content edit
		quotations.Delete("/:id", middleware.RequirePermission(db, "quotations.delete"), ctrl.DeleteQuotation)
		quotations.Get("/:id/export", middleware.RequirePermission(db, "quotations.read"), ctrl.ExportQuotation)
		quotations.Post("/:id/images", middleware.RequirePermission(db, "quotations.create"), ctrl.UploadImages)
	}
}
