package v1

import (
	"jk-api/config"
	"jk-api/internal/middleware"
	reportCtrl "jk-api/internal/module/report/controller"
	reportRepo "jk-api/internal/module/report/repository"
	reportUC "jk-api/internal/module/report/usecase"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// รายงานยอดขาย. It summarises documents the caller can already read, and
// re-applies the same store/branch/own-documents scope the quotation list uses,
// so it rides on quotations.read rather than inventing a permission that would
// have to be handed out before anyone could see their own numbers.
func SetupReportRoutes(v1 fiber.Router, db *gorm.DB, cfg *config.Config) {
	repo := reportRepo.NewReportRepository(db)
	uc := reportUC.NewReportUsecase(repo)
	ctrl := reportCtrl.NewReportController(uc, db)

	r := v1.Group("/reports", middleware.AuthMiddleware(cfg))
	{
		r.Get("/sales", middleware.RequirePermission(db, "quotations.read"), ctrl.GetSales)
		r.Get("/sales/rows", middleware.RequirePermission(db, "quotations.read"), ctrl.GetSalesRows)
		r.Get("/sales/export", middleware.RequirePermission(db, "quotations.read"), ctrl.ExportSales)
	}
}
