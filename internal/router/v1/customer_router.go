package v1

import (
	"jk-api/config"
	"jk-api/internal/middleware"
	customerCtrl "jk-api/internal/module/customer/controller"
	customerRepo "jk-api/internal/module/customer/repository"
	customerUC "jk-api/internal/module/customer/usecase"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func SetupCustomerRoutes(v1 fiber.Router, db *gorm.DB, cfg *config.Config) {
	cRepo := customerRepo.NewCustomerRepository(db)
	uc := customerUC.NewCustomerUsecase(cRepo)
	ctrl := customerCtrl.NewCustomerController(uc)

	customers := v1.Group("/customers", middleware.AuthMiddleware(cfg))
	{
		customers.Get("/",     middleware.RequirePermission(db, "customers.read"),   ctrl.GetAllCustomers)
		customers.Get("/:id",  middleware.RequirePermission(db, "customers.read"),   ctrl.GetCustomerByID)
		customers.Post("/",    middleware.RequirePermission(db, "customers.create"), ctrl.CreateCustomer)
		customers.Put("/:id",  middleware.RequirePermission(db, "customers.update"), ctrl.UpdateCustomer)
		customers.Delete("/:id", middleware.RequirePermission(db, "customers.delete"), ctrl.DeleteCustomer)
	}
}
