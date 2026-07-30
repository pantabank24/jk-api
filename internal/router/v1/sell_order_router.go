package v1

import (
	"jk-api/config"
	"jk-api/internal/middleware"
	soCtrl "jk-api/internal/module/sell_order/controller"
	soRepo "jk-api/internal/module/sell_order/repository"
	soUC "jk-api/internal/module/sell_order/usecase"
	"jk-api/internal/service"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func SetupSellOrderRoutes(v1 fiber.Router, db *gorm.DB, cfg *config.Config, engine *service.SellOrderEngine) {
	repo := soRepo.NewSellOrderRepository(db)
	uc := soUC.NewSellOrderUsecase(repo, db)
	ctrl := soCtrl.NewSellOrderController(uc, engine, db)

	g := v1.Group("/sell-orders", middleware.AuthMiddleware(cfg))
	{
		// Auth-only: every screen that mentions auto-sell needs to know whether it
		// is on and what the live price is (mirrors /configs/sales-status).
		g.Get("/status", ctrl.GetStatus)
		g.Get("/", middleware.RequirePermission(db, "sell_orders.read"), ctrl.GetAll)
		g.Get("/:id", middleware.RequirePermission(db, "sell_orders.read"), ctrl.GetByID)
		// Customers place their own orders; staff need manage to place one for them.
		g.Post("/", middleware.RequireAnyPermission(db, "sell_orders.create", "sell_orders.manage"), ctrl.Create)
		// Registered before "/:id/cancel" — fiber matches routes in order.
		g.Post("/run-now", middleware.RequirePermission(db, "sell_orders.manage"), ctrl.RunNow)
		g.Post("/:id/cancel", middleware.RequireAnyPermission(db, "sell_orders.create", "sell_orders.manage"), ctrl.Cancel)
	}
}
