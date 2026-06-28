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
	nRepo := notifRepo.NewNotificationRepository(db)
	uc := billUC.NewBillUsecase(bRepo, nRepo)
	ctrl := billCtrl.NewBillController(uc, db)

	bills := v1.Group("/bills", middleware.AuthMiddleware(cfg))
	{
		bills.Get("/unfinished-count", middleware.RequirePermission(db, "bills.read"), ctrl.GetUnfinishedCount)
		bills.Get("/",        middleware.RequirePermission(db, "bills.read"),    ctrl.GetAllBills)
		bills.Get("/:id",     middleware.RequirePermission(db, "bills.read"),    ctrl.GetBillByID)
		bills.Post("/",       middleware.RequirePermission(db, "bills.create"),  ctrl.CreateBill)
		bills.Patch("/:id",   middleware.RequirePermission(db, "bills.create"),  ctrl.UpdateBill)          // edit while pending issue
		bills.Post("/:id/issue",   middleware.RequirePermission(db, "bills.issue"),   ctrl.IssueBill)     // รอออกบิล → รอตรวจบิล
		bills.Post("/:id/approve", middleware.RequirePermission(db, "bills.approve"), ctrl.ApproveBill)   // รอตรวจบิล → สำเร็จ
		bills.Post("/:id/cancel",  middleware.RequirePermission(db, "bills.approve"), ctrl.CancelBill)    // → ยกเลิก
		bills.Delete("/:id",  middleware.RequirePermission(db, "bills.approve"), ctrl.DeleteBill)
		bills.Post("/:id/images", middleware.RequirePermission(db, "bills.create"), ctrl.UploadImages)
	}
}
