package controller

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"jk-api/internal/entity"
	"jk-api/internal/middleware"
	"jk-api/internal/module/bill/usecase"
	"jk-api/internal/service"
	"jk-api/pkg/linenotify"
	"jk-api/pkg/response"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type BillController struct {
	billUsecase usecase.BillUsecase
	db          *gorm.DB
}

func NewBillController(billUsecase usecase.BillUsecase, db *gorm.DB) *BillController {
	return &BillController{billUsecase: billUsecase, db: db}
}

// scope resolves which bills the caller may see based on their role.
// Returns (storeID, branchID, createdBy) filters.
func (ctrl *BillController) scope(c *fiber.Ctx) (*uint, *uint, *uint) {
	roleName := middleware.GetRoleName(c)
	switch roleName {
	case "master":
		var storeID, branchID, createdBy *uint
		if sid := c.Query("store_id"); sid != "" {
			if id, err := strconv.ParseUint(sid, 10, 32); err == nil {
				uid := uint(id)
				storeID = &uid
			}
		}
		if bid := c.Query("branch_id"); bid != "" {
			if id, err := strconv.ParseUint(bid, 10, 32); err == nil {
				uid := uint(id)
				branchID = &uid
			}
		}
		// Master may list a specific customer's bills (used to combine all of a
		// customer's pending bills when issuing).
		if cb := c.Query("created_by"); cb != "" {
			if id, err := strconv.ParseUint(cb, 10, 32); err == nil {
				uid := uint(id)
				createdBy = &uid
			}
		}
		return storeID, branchID, createdBy
	case "customer":
		// customers only see their own bills
		userID := middleware.GetUserID(c)
		return nil, nil, &userID
	case "owner":
		// Owners may narrow to one customer's bills (customer detail page);
		// still scoped to their own store.
		return middleware.GetStoreID(c), nil, queryCreatedBy(c)
	default: // employee — locked to store + branch
		return middleware.GetStoreID(c), middleware.GetBranchID(c), queryCreatedBy(c)
	}
}

// queryCreatedBy parses the optional created_by query param (a customer's user
// id). Safe for store-scoped roles because it only narrows their result set.
func queryCreatedBy(c *fiber.Ctx) *uint {
	if cb := c.Query("created_by"); cb != "" {
		if id, err := strconv.ParseUint(cb, 10, 32); err == nil {
			uid := uint(id)
			return &uid
		}
	}
	return nil
}

func (ctrl *BillController) CreateBill(c *fiber.Ctx) error {
	var req usecase.CreateBillRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}
	if len(req.Items) == 0 {
		return response.BadRequest(c, "ต้องมีรายการอย่างน้อย 1 รายการ")
	}

	// Block creation when bills_open config is false.
	var billsOpenCfg entity.SystemConfig
	if err := ctrl.db.Where("key = ?", "bills_open").First(&billsOpenCfg).Error; err == nil {
		if billsOpenCfg.Value == "false" {
			return response.BadRequest(c, "ขณะนี้ปิดรับซื้อ ไม่สามารถสร้างบิลได้")
		}
	}

	// Block creation when closed; otherwise pick the price source for the round.
	status := service.GetSalesStatus(ctrl.db)
	if !status.IsOpen {
		return response.BadRequest(c, "ขณะนี้ปิดทำการ ไม่สามารถสร้างบิลได้")
	}
	if status.PriceMode == service.PriceModeRealtime {
		// Lock a snapshot of the real-time price for this document.
		req.GoldRound, req.GoldPriceID = service.SnapshotRealtimeRound(ctrl.db)
	} else {
		// Stamp the association gold-price round in effect now (for reporting).
		req.GoldRound, req.GoldPriceID = service.CurrentRound(ctrl.db)
	}

	// Always derive store/branch from JWT (never from the payload), like employees.
	if storeID := middleware.GetStoreID(c); storeID != nil {
		req.StoreID = storeID
	}
	if branchID := middleware.GetBranchID(c); branchID != nil {
		req.BranchID = branchID
	}
	req.CreatedByUserID = middleware.GetUserID(c)

	bill, err := ctrl.billUsecase.CreateBill(&req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	middleware.SetActivityDescription(c, fmt.Sprintf("ลูกค้าสร้างบิลขาย %s", bill.Code))
	return response.Created(c, "Bill created", bill)
}

func (ctrl *BillController) GetAllBills(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "10"))
	search := c.Query("search", "")

	storeID, branchID, createdBy := ctrl.scope(c)

	var status *int
	if s := c.Query("status"); s != "" {
		st, _ := strconv.Atoi(s)
		status = &st
	}

	bills, total, err := ctrl.billUsecase.GetAllBills(storeID, branchID, createdBy, status, page, limit, search)
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}
	return response.Paginated(c, "Bills retrieved", bills, page, limit, total)
}

func (ctrl *BillController) GetBillByID(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid bill ID")
	}
	bill, err := ctrl.billUsecase.GetBillByID(uint(id))
	if err != nil {
		return response.NotFound(c, "Bill not found")
	}
	return response.Success(c, "Bill retrieved", bill)
}

func (ctrl *BillController) GetUnfinishedCount(c *fiber.Ctx) error {
	storeID, branchID, createdBy := ctrl.scope(c)
	count, err := ctrl.billUsecase.CountUnfinished(storeID, branchID, createdBy)
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}
	return response.Success(c, "ok", fiber.Map{"count": count})
}

func (ctrl *BillController) IssueBill(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid bill ID")
	}
	var req usecase.UpdateBillStatusRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}
	bill, err := ctrl.billUsecase.IssueBill(uint(id), &req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	middleware.SetActivityDescription(c, fmt.Sprintf("ออกบิล %s ให้ลูกค้า", bill.Code))
	return response.Success(c, "Bill issued", bill)
}

func (ctrl *BillController) ApproveBill(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid bill ID")
	}
	var req usecase.UpdateBillStatusRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}
	bill, err := ctrl.billUsecase.ApproveBill(uint(id), &req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	middleware.SetActivityDescription(c, fmt.Sprintf("อนุมัติปิดบิล %s", bill.Code))
	go ctrl.maybeSendLineNotify(bill.StoreID)
	return response.Success(c, "Bill approved", bill)
}

func (ctrl *BillController) RemoveBillItem(c *fiber.Ctx) error {
	billID, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid bill ID")
	}
	itemID, err := strconv.ParseUint(c.Params("itemId"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid item ID")
	}
	bill, deleted, err := ctrl.billUsecase.RemoveBillItem(uint(billID), uint(itemID))
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	if deleted {
		middleware.SetActivityDescription(c, fmt.Sprintf("ลบรายการสุดท้ายและลบบิล #%d", billID))
		return response.Success(c, "Bill item removed; bill deleted", fiber.Map{"deleted": true})
	}
	middleware.SetActivityDescription(c, fmt.Sprintf("ลบรายการในบิล %s", bill.Code))
	return response.Success(c, "Bill item removed", fiber.Map{"deleted": false, "bill": bill})
}

func (ctrl *BillController) RevertBill(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid bill ID")
	}
	bill, err := ctrl.billUsecase.RevertBill(uint(id))
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	middleware.SetActivityDescription(c, fmt.Sprintf("ดึงบิล %s กลับไปแก้ไข", bill.Code))
	return response.Success(c, "Bill reverted", bill)
}

func (ctrl *BillController) maybeSendLineNotify(storeID *uint) {
	var enabledCfg entity.SystemConfig
	if err := ctrl.db.Where("key = ?", "line_notify_enabled").First(&enabledCfg).Error; err != nil || enabledCfg.Value != "true" {
		return
	}
	var targetCfg entity.SystemConfig
	if err := ctrl.db.Where("key = ?", "line_notify_target_id").First(&targetCfg).Error; err != nil || targetCfg.Value == "" {
		return
	}
	var thresholdCfg entity.SystemConfig
	if err := ctrl.db.Where("key = ?", "line_bill_notify_threshold").First(&thresholdCfg).Error; err != nil {
		return
	}
	threshold, _ := strconv.Atoi(thresholdCfg.Value)
	if threshold <= 0 {
		return
	}
	query := ctrl.db.Model(&entity.Quotation{}).Where("is_bill = ? AND status = ?", true, 12)
	if storeID != nil {
		query = query.Where("store_id = ?", *storeID)
	}
	var count int64
	query.Count(&count)
	if count >= int64(threshold) {
		msg := fmt.Sprintf("🔔 แจ้งเตือน: มีบิลสำเร็จที่ยังไม่เคลียร์ %d บิล (ถึงเกณฑ์ %d บิล)", count, threshold)
		_ = linenotify.SendText(targetCfg.Value, msg)
	}
}

func (ctrl *BillController) ClearBills(c *fiber.Ctx) error {
	storeID, _, _ := ctrl.scope(c)
	count, err := ctrl.billUsecase.ClearBills(storeID)
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}
	middleware.SetActivityDescription(c, fmt.Sprintf("เคลียร์บิลสำเร็จ %d บิล", count))
	return response.Success(c, "Bills cleared", fiber.Map{"cleared": count})
}

func (ctrl *BillController) CancelBill(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid bill ID")
	}
	var req usecase.UpdateBillStatusRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}
	bill, err := ctrl.billUsecase.CancelBill(uint(id), &req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	middleware.SetActivityDescription(c, fmt.Sprintf("ยกเลิกบิล %s", bill.Code))
	return response.Success(c, "Bill cancelled", bill)
}

func (ctrl *BillController) UpdateBill(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid bill ID")
	}
	var req usecase.UpdateBillRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}
	bill, err := ctrl.billUsecase.UpdateBill(uint(id), &req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.Success(c, "Bill updated", bill)
}

func (ctrl *BillController) DeleteBill(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid bill ID")
	}
	if bill, err := ctrl.billUsecase.GetBillByID(uint(id)); err == nil {
		middleware.SetActivityDescription(c, fmt.Sprintf("ลบบิล %s", bill.Code))
	}

	if err := ctrl.billUsecase.DeleteBill(uint(id)); err != nil {
		return response.NotFound(c, err.Error())
	}
	return response.Success(c, "Bill deleted", nil)
}

func (ctrl *BillController) GetDeliveryLogs(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid bill ID")
	}
	logs, err := ctrl.billUsecase.GetDeliveryLogs(uint(id))
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}
	return response.Success(c, "ok", logs)
}

func (ctrl *BillController) PartialDeliver(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid bill ID")
	}
	var req usecase.PartialDeliverRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}
	bill, err := ctrl.billUsecase.PartialDeliverBill(uint(id), &req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.Success(c, "Partial delivery recorded", bill)
}

func (ctrl *BillController) GetBillBalance(c *fiber.Ctx) error {
	var userID uint
	if id := c.Query("user_id"); id != "" {
		parsed, err := strconv.ParseUint(id, 10, 32)
		if err != nil {
			return response.BadRequest(c, "Invalid user_id")
		}
		userID = uint(parsed)
	} else {
		userID = middleware.GetUserID(c)
	}
	summary, history, err := ctrl.billUsecase.GetBillBalance(userID)
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}
	return response.Success(c, "ok", fiber.Map{
		"balance":      summary.Balance,
		"total_weight": summary.TotalWeight,
		"avg_price":    summary.AvgPrice,
		"history":      history,
	})
}

func (ctrl *BillController) UploadImages(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid bill ID")
	}
	form, err := c.MultipartForm()
	if err != nil {
		return response.BadRequest(c, "Invalid form data")
	}
	files := form.File["images"]
	if len(files) == 0 {
		return response.BadRequest(c, "No images provided")
	}
	dir := fmt.Sprintf("./uploads/bills/%d", id)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return response.InternalServerError(c, "Failed to create directory")
	}
	var urls []string
	for _, file := range files {
		ext := filepath.Ext(file.Filename)
		filename := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
		savePath := fmt.Sprintf("%s/%s", dir, filename)
		if err := c.SaveFile(file, savePath); err != nil {
			return response.InternalServerError(c, "Failed to save file")
		}
		urls = append(urls, fmt.Sprintf("/uploads/bills/%d/%s", id, filename))
	}
	if err := ctrl.billUsecase.AddImages(uint(id), urls); err != nil {
		return response.InternalServerError(c, err.Error())
	}
	return response.Success(c, "Images uploaded", urls)
}
