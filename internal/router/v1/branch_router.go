package v1

import (
	"jk-api/config"
	"jk-api/internal/middleware"
	branchCtrl "jk-api/internal/module/branch/controller"
	branchRepo "jk-api/internal/module/branch/repository"
	branchUC "jk-api/internal/module/branch/usecase"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func SetupBranchRoutes(v1 fiber.Router, db *gorm.DB, cfg *config.Config) {
	repo := branchRepo.NewBranchRepository(db)
	uc := branchUC.NewBranchUsecase(repo)
	ctrl := branchCtrl.NewBranchController(uc)

	branches := v1.Group("/stores/:storeId/branches", middleware.AuthMiddleware(cfg))
	{
		branches.Get("/", middleware.RequirePermission(db, "branches.read"), ctrl.GetAllBranches)
		branches.Get("/:id", middleware.RequirePermission(db, "branches.read"), ctrl.GetBranchByID)
		branches.Post("/", middleware.RequirePermission(db, "branches.create"), ctrl.CreateBranch)
		branches.Put("/:id", middleware.RequirePermission(db, "branches.update"), ctrl.UpdateBranch)
		branches.Delete("/:id", middleware.RequirePermission(db, "branches.delete"), ctrl.DeleteBranch)
	}
}
