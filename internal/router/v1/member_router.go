package v1

import (
	"jk-api/config"
	"jk-api/internal/middleware"
	memberCtrl "jk-api/internal/module/member/controller"
	memberRepo "jk-api/internal/module/member/repository"
	memberUC "jk-api/internal/module/member/usecase"
	notifRepo "jk-api/internal/module/notification/repository"
	roleRepo "jk-api/internal/module/role/repository"
	userRepo "jk-api/internal/module/user/repository"
	userUC "jk-api/internal/module/user/usecase"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func SetupMemberRoutes(v1 fiber.Router, db *gorm.DB, cfg *config.Config) {
	mRepo := memberRepo.NewMemberRepository(db)
	uRepo := userRepo.NewUserRepository(db)
	rRepo := roleRepo.NewRoleRepository(db)
	nRepo := notifRepo.NewNotificationRepository(db)

	uc := memberUC.NewMemberUsecase(mRepo, nRepo)
	uuc := userUC.NewUserUsecase(uRepo, rRepo)
	ctrl := memberCtrl.NewMemberController(uc, uuc)

	members := v1.Group("/members", middleware.AuthMiddleware(cfg))
	{
		members.Get("/", middleware.RequirePermission(db, "members.read"), ctrl.GetAllMembers)
		members.Get("/:id", middleware.RequirePermission(db, "members.read"), ctrl.GetMemberByID)
		members.Post("/", middleware.RequirePermission(db, "members.create"), ctrl.CreateMember)
		members.Put("/:id", middleware.RequirePermission(db, "members.update"), ctrl.UpdateMember)
		members.Delete("/:id", middleware.RequirePermission(db, "members.delete"), ctrl.DeleteMember)
		members.Post("/:id/credit", middleware.RequirePermission(db, "credits.update"), ctrl.AddCredit)
		members.Get("/:id/transactions", middleware.RequirePermission(db, "credits.read"), ctrl.GetCreditTransactions)
		members.Get("/credit-transactions/all", middleware.RequirePermission(db, "credits.read"), ctrl.GetAllCreditTransactions)
	}
}
