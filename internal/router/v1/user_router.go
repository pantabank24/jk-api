package v1

import (
	"jk-api/config"
	"jk-api/internal/middleware"
	roleRepo "jk-api/internal/module/role/repository"
	"jk-api/internal/module/user/controller"
	"jk-api/internal/module/user/repository"
	"jk-api/internal/module/user/usecase"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func SetupUserRoutes(v1 fiber.Router, db *gorm.DB, cfg *config.Config) {
	// Initialize dependencies (manual DI)
	userRepo := repository.NewUserRepository(db)
	rRepo := roleRepo.NewRoleRepository(db)
	userUsecase := usecase.NewUserUsecase(userRepo, rRepo)
	userController := controller.NewUserController(userUsecase)

	// User routes group (all protected)
	users := v1.Group("/users", middleware.AuthMiddleware(cfg))
	{
		users.Post("/", middleware.RequirePermission(db, "users.create"), userController.CreateUser)
		users.Get("/", middleware.RequirePermission(db, "users.read"), userController.GetAllUsers)
		users.Get("/:id", middleware.RequirePermission(db, "users.read"), userController.GetUserByID)
		users.Put("/:id", middleware.RequirePermission(db, "users.update"), userController.UpdateUser)
		users.Delete("/:id", middleware.RequirePermission(db, "users.delete"), userController.DeleteUser)
	}
}
