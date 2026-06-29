package controller

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"jk-api/internal/middleware"
	"jk-api/internal/module/bill/usecase"
	"jk-api/internal/service"
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
		return middleware.GetStoreID(c), nil, nil
	default: // employee — locked to store + branch
		return middleware.GetStoreID(c), middleware.GetBranchID(c), nil
	}
}

func (ctrl *BillController) CreateBill(c *fiber.Ctx) error {
	var req usecase.CreateBillRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}
	if len(req.Items) == 0 {
		return response.BadRequest(c, "ต้องมีรายการอย่างน้อย 1 รายการ")
	}

	// Block creation outside sale hours.
	if status := service.GetSalesStatus(ctrl.db); !status.IsOpen {
		return response.BadRequest(c, fmt.Sprintf("ขณะนี้ปิดทำการ ไม่สามารถสร้างบิลได้ (เวลาทำการ %s - %s น.)", status.OpenTime, status.CloseTime))
	}

	// Stamp the gold-price round in effect now (for reporting).
	req.GoldRound, req.GoldPriceID = service.CurrentRound(ctrl.db)

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
	return response.Success(c, "Bill approved", bill)
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
	balance, history, err := ctrl.billUsecase.GetBillBalance(userID)
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}
	return response.Success(c, "ok", fiber.Map{"balance": balance, "history": history})
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
