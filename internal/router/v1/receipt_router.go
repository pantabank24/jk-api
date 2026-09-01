package v1

import (
	"jk-api/config"
	"jk-api/internal/middleware"
	receiptCtrl "jk-api/internal/module/receipt/controller"
	receiptRepo "jk-api/internal/module/receipt/repository"
	receiptUC "jk-api/internal/module/receipt/usecase"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// บันทึกใบเสร็จแอดมิน. Every route is permission-gated and no role holds these
// permissions out of the box (migration 000095), so this ships master-only.
func SetupReceiptRoutes(v1 fiber.Router, db *gorm.DB, cfg *config.Config) {
	repo := receiptRepo.NewReceiptRepository(db)
	uc := receiptUC.NewReceiptUsecase(repo)
	ctrl := receiptCtrl.NewReceiptController(uc)

	r := v1.Group("/receipts", middleware.AuthMiddleware(cfg))
	{
		// Registered before "/:id" — Fiber matches in order, so the other way round
		// would send "settings" into the id handler.
		r.Get("/settings", middleware.RequirePermission(db, "receipts.read"), ctrl.GetSettings)
		r.Put("/settings", middleware.RequirePermission(db, "receipts.update"), ctrl.UpdateSettings)
		r.Post("/settings/logo", middleware.RequirePermission(db, "receipts.update"), ctrl.UploadLogo)

		r.Get("/", middleware.RequirePermission(db, "receipts.read"), ctrl.GetAll)
		r.Get("/:id", middleware.RequirePermission(db, "receipts.read"), ctrl.GetByID)
		r.Post("/", middleware.RequirePermission(db, "receipts.create"), ctrl.Create)
		r.Put("/:id", middleware.RequirePermission(db, "receipts.update"), ctrl.Update)
		r.Delete("/:id", middleware.RequirePermission(db, "receipts.delete"), ctrl.Delete)
	}
}
