package v1

import (
	"jk-api/config"
	"jk-api/internal/middleware"
	intakeCtrl "jk-api/internal/module/quotation_intake/controller"
	intakeRepo "jk-api/internal/module/quotation_intake/repository"
	intakeUC "jk-api/internal/module/quotation_intake/usecase"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// ใบเปิดงาน — the counter step taken before the metal is melted. It is the first
// half of issuing a quotation, so it rides on the quotations.* permissions rather
// than introducing its own: anyone who may issue a quotation may open a job.
func SetupQuotationIntakeRoutes(v1 fiber.Router, db *gorm.DB, cfg *config.Config) {
	repo := intakeRepo.NewIntakeRepository(db)
	uc := intakeUC.NewIntakeUsecase(repo)
	ctrl := intakeCtrl.NewIntakeController(uc, db)

	r := v1.Group("/quotation-intakes", middleware.AuthMiddleware(cfg))
	{
		// Reading takes either half of the pair: a role configured to issue
		// quotations but not to browse them still has to open the ใบเปิดงาน it is
		// issuing from.
		r.Get("/", middleware.RequireAnyPermission(db, "quotations.read", "quotations.create"), ctrl.GetAll)
		r.Get("/:id", middleware.RequireAnyPermission(db, "quotations.read", "quotations.create"), ctrl.GetByID)
		r.Post("/", middleware.RequirePermission(db, "quotations.create"), ctrl.Create)
		r.Patch("/:id", middleware.RequirePermission(db, "quotations.create"), ctrl.Update)
		r.Post("/:id/cancel", middleware.RequirePermission(db, "quotations.create"), ctrl.Cancel)
		r.Delete("/:id", middleware.RequirePermission(db, "quotations.delete"), ctrl.Delete)

		r.Post("/:id/images", middleware.RequirePermission(db, "quotations.create"), ctrl.UploadImages)
		r.Delete("/:id/images/:imageId", middleware.RequirePermission(db, "quotations.create"), ctrl.DeleteImage)
	}
}
