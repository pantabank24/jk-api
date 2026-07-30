package controller

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"jk-api/internal/entity"
	"jk-api/internal/middleware"
	"jk-api/internal/module/sell_order/repository"
	"jk-api/internal/module/sell_order/usecase"
	"jk-api/internal/service"
	"jk-api/pkg/response"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type SellOrderController struct {
	uc     usecase.SellOrderUsecase
	engine *service.SellOrderEngine
	db     *gorm.DB
}

func NewSellOrderController(uc usecase.SellOrderUsecase, engine *service.SellOrderEngine, db *gorm.DB) *SellOrderController {
	return &SellOrderController{uc: uc, engine: engine, db: db}
}

// GetStatus reports whether auto-sell is available right now plus the caps and the
// live price targets are measured against. Auth-only (no permission): every screen
// that mentions the feature needs it, exactly like /configs/sales-status.
func (ctrl *SellOrderController) GetStatus(c *fiber.Ctx) error {
	return response.Success(c, "Auto-sell status", service.GetAutoSellStatus(ctrl.db))
}

// scope decides which orders the caller may see. Customers are locked to their
// own; staff holding sell_orders.manage see everything and may narrow by customer.
func (ctrl *SellOrderController) scope(c *fiber.Ctx) *uint {
	if middleware.GetRoleName(c) == "customer" {
		userID := middleware.GetUserID(c)
		return &userID
	}
	if uid := c.Query("user_id"); uid != "" {
		if id, err := strconv.ParseUint(uid, 10, 32); err == nil {
			parsed := uint(id)
			return &parsed
		}
	}
	return nil
}

func (ctrl *SellOrderController) GetAll(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))

	f := repository.SellOrderFilter{
		UserID: ctrl.scope(c),
		Status: strings.TrimSpace(c.Query("status")),
		Search: strings.TrimSpace(c.Query("search")),
	}
	// A customer's own list must not be searchable across users.
	if middleware.GetRoleName(c) == "customer" {
		f.Search = ""
	}

	orders, total, err := ctrl.uc.List(f, page, limit)
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	return response.Paginated(c, "Sell orders retrieved", orders, page, limit, total)
}

// canReach reports whether the caller may act on an order. Customers only reach
// their own; sell_orders.manage reaches any.
func (ctrl *SellOrderController) canReach(c *fiber.Ctx, order *entity.SellOrder) bool {
	if middleware.GetRoleName(c) == "customer" {
		return order.UserID == middleware.GetUserID(c)
	}
	return middleware.HasPermission(ctrl.db, c, "sell_orders.manage")
}

func (ctrl *SellOrderController) GetByID(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid id")
	}
	order, err := ctrl.uc.GetByID(uint(id))
	if err != nil {
		return response.NotFound(c, "ไม่พบคำสั่งขายนี้")
	}
	if !ctrl.canReach(c, order) {
		return response.Forbidden(c, "ไม่มีสิทธิ์เข้าถึงคำสั่งขายนี้")
	}
	return response.Success(c, "Sell order retrieved", order)
}

type createSellOrderPayload struct {
	Weight      float64 `json:"weight"`
	TargetPrice float64 `json:"target_price"`
	Note        string  `json:"note"`
	// CustomerID is honoured only for a staff member placing an order on a
	// customer's behalf (sell_orders.manage); the self-service flow ignores it.
	CustomerID uint `json:"customer_id"`
}

func (ctrl *SellOrderController) Create(c *fiber.Ctx) error {
	var payload createSellOrderPayload
	if err := c.BodyParser(&payload); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	req := &usecase.CreateSellOrderRequest{
		Weight:      payload.Weight,
		TargetPrice: payload.TargetPrice,
		Note:        strings.TrimSpace(payload.Note),
		CreatedBy:   middleware.GetUserID(c),
		StoreID:     middleware.GetStoreID(c),
		BranchID:    middleware.GetBranchID(c),
	}

	// The order belongs to the customer it sells for; staff placing one must name
	// that customer, exactly as they must when selling on their behalf.
	if middleware.GetRoleName(c) == "customer" {
		req.UserID = middleware.GetUserID(c)
	} else {
		if payload.CustomerID == 0 {
			return response.BadRequest(c, "กรุณาเลือกลูกค้าก่อนตั้งคำสั่งขาย")
		}
		cust, err := ctrl.resolveCustomer(payload.CustomerID)
		if err != nil {
			return response.BadRequest(c, err.Error())
		}
		req.UserID = cust.ID
	}

	order, err := ctrl.uc.Create(req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	middleware.SetActivityDescription(c, fmt.Sprintf("ตั้งคำสั่งขายอัตโนมัติ %g บาท ที่ราคา %.2f",
		order.Weight, order.TargetPrice))
	middleware.SetActivityTarget(c, order.UserID)
	// price_at_create is the price on screen when the target was set — the pair
	// (target vs market) is what explains a later fill the customer disputes.
	middleware.SetActivityDetail(c, map[string]any{
		"kind":            "sell_order",
		"sell_order_id":   order.ID,
		"metal":           order.Metal,
		"type_name":       order.TypeName,
		"weight":          order.Weight,
		"target_price":    order.TargetPrice,
		"price_at_create": order.PriceAtCreate,
		"premium":         order.PremiumAtCreate,
		"spread":          order.SpreadAtCreate,
		"estimated":       order.EstimatedAmount(),
	})
	return response.Created(c, "ตั้งคำสั่งขายอัตโนมัติแล้ว", order)
}

func (ctrl *SellOrderController) resolveCustomer(customerID uint) (*entity.User, error) {
	var cust entity.User
	err := ctrl.db.
		Joins("JOIN roles ON roles.id = users.role_id").
		Where("users.id = ? AND roles.name = ?", customerID, "customer").
		First(&cust).Error
	if err != nil {
		return nil, errors.New("ไม่พบลูกค้าที่เลือก")
	}
	if !cust.IsActive {
		return nil, errors.New("ลูกค้ารายนี้ถูกปิดใช้งาน")
	}
	return &cust, nil
}

type cancelSellOrderPayload struct {
	Reason string `json:"reason"`
}

func (ctrl *SellOrderController) Cancel(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid id")
	}
	order, err := ctrl.uc.GetByID(uint(id))
	if err != nil {
		return response.NotFound(c, "ไม่พบคำสั่งขายนี้")
	}
	if !ctrl.canReach(c, order) {
		return response.Forbidden(c, "ไม่มีสิทธิ์ยกเลิกคำสั่งขายนี้")
	}

	var payload cancelSellOrderPayload
	_ = c.BodyParser(&payload) // body is optional

	byStaff := middleware.GetRoleName(c) != "customer"
	updated, err := ctrl.uc.Cancel(uint(id), byStaff, strings.TrimSpace(payload.Reason))
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	middleware.SetActivityDescription(c, fmt.Sprintf("ยกเลิกคำสั่งขายอัตโนมัติ #%d (%g บาท ที่ราคา %.2f)",
		updated.ID, updated.Weight, updated.TargetPrice))
	middleware.SetActivityTarget(c, updated.UserID)
	return response.Success(c, "ยกเลิกคำสั่งขายแล้ว", updated)
}

// RunNow triggers one matching pass immediately, so an admin changing the settings
// does not have to wait out the tick to see the effect. Staff only (route-gated).
func (ctrl *SellOrderController) RunNow(c *fiber.Ctx) error {
	ctrl.engine.Tick()
	return response.Success(c, "ตรวจราคาแล้ว", service.GetAutoSellStatus(ctrl.db))
}
