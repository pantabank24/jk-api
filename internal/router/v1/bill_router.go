package v1

import (
	"jk-api/config"
	"jk-api/internal/middleware"
	billCtrl "jk-api/internal/module/bill/controller"
	billRepo "jk-api/internal/module/bill/repository"
	billUC "jk-api/internal/module/bill/usecase"
	notifRepo "jk-api/internal/module/notification/repository"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func SetupBillRoutes(v1 fiber.Router, db *gorm.DB, cfg *config.Config) {
	bRepo := billRepo.NewBillRepository(db)
	bbRepo := billRepo.NewBillBalanceRepository(db)
	nRepo := notifRepo.NewNotificationRepository(db)
	uc := billUC.NewBillUsecase(bRepo, bbRepo, nRepo)
	ctrl := billCtrl.NewBillController(uc, db)

	bills := v1.Group("/bills", middleware.AuthMiddleware(cfg))
	{
		bills.Get("/unfinished-count", middleware.RequirePermission(db, "bills.read"), ctrl.GetUnfinishedCount)
		bills.Get("/balance",          middleware.RequirePermission(db, "bills.read"), ctrl.GetBillBalance)
		bills.Get("/",        middleware.RequirePermission(db, "bills.read"),    ctrl.GetAllBills)
		bills.Get("/:id",     middleware.RequirePermission(db, "bills.read"),    ctrl.GetBillByID)
		bills.Post("/",       middleware.RequirePermission(db, "bills.create"),  ctrl.CreateBill)
		bills.Patch("/:id",   middleware.RequirePermission(db, "bills.create"),  ctrl.UpdateBill)
		bills.Post("/:id/issue",           middleware.RequirePermission(db, "bills.issue"),   ctrl.IssueBill)
		bills.Post("/:id/approve",         middleware.RequirePermission(db, "bills.approve"), ctrl.ApproveBill)
		bills.Post("/:id/cancel",          middleware.RequirePermission(db, "bills.approve"), ctrl.CancelBill)
		bills.Post("/:id/revert",          middleware.RequirePermission(db, "bills.approve"), ctrl.RevertBill)
		bills.Get("/:id/delivery-logs",    middleware.RequirePermission(db, "bills.read"),    ctrl.GetDeliveryLogs)
		bills.Post("/:id/partial-deliver", middleware.RequirePermission(db, "bills.issue"),   ctrl.PartialDeliver)
		bills.Delete("/:id/items/:itemId", middleware.RequirePermission(db, "bills.issue"),   ctrl.RemoveBillItem)
		bills.Post("/clear",  middleware.RequirePermission(db, "bills.approve"), ctrl.ClearBills)
		bills.Delete("/:id",  middleware.RequirePermission(db, "bills.approve"), ctrl.DeleteBill)
		bills.Post("/:id/images", middleware.RequirePermission(db, "bills.create"), ctrl.UploadImages)
	}
}
