package controller

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"jk-api/internal/entity"
	"jk-api/internal/middleware"
	"jk-api/internal/module/bill/repository"
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
		// Owners may narrow to one customer's bills (customer detail page);
		// still scoped to their own store.
		return middleware.GetStoreID(c), nil, queryCreatedBy(c)
	default: // employee — locked to store + branch
		return middleware.GetStoreID(c), middleware.GetBranchID(c), queryCreatedBy(c)
	}
}

// queryMetal parses the optional metal query param used by the split list pages
// (รายการขายทอง / รายการขายเงิน). Absent = every metal, as before.
func queryMetal(c *fiber.Ctx) *string {
	if m := strings.TrimSpace(c.Query("metal")); m != "" {
		return &m
	}
	return nil
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

	// Two flows share this endpoint:
	//   - customer self-service: gated by the customer-facing switches (bills_open
	//     + store hours); the bill is created for the caller.
	//   - staff on-behalf (master/owner/employee with bills.sell): pick a customer
	//     and sell for them, bypassing the customer-facing switches entirely.
	staffSelling := middleware.GetRoleName(c) != "customer"
	var sellCustomer *entity.User

	status := service.GetSalesStatus(ctrl.db)

	if staffSelling {
		if req.CustomerID == 0 {
			return response.BadRequest(c, "กรุณาเลือกลูกค้าก่อนทำการขาย")
		}
		cust, err := ctrl.resolveSellCustomer(req.CustomerID)
		if err != nil {
			return response.BadRequest(c, err.Error())
		}
		sellCustomer = cust
	} else {
		// Gold and silver open independently, so validate each metal against its
		// own schedule (a customer may sell silver after gold hours, and vice versa).
		hasGold, hasSilver := false, false
		for _, it := range req.Items {
			if it.Metal == "" || it.Metal == "gold" {
				hasGold = true
			} else {
				hasSilver = true
			}
		}

		if hasGold {
			// Block gold when bills_open is false or the store is closed.
			var billsOpenCfg entity.SystemConfig
			if err := ctrl.db.Where("key = ?", "bills_open").First(&billsOpenCfg).Error; err == nil {
				if billsOpenCfg.Value == "false" {
					return response.BadRequest(c, "ขณะนี้ปิดรับซื้อทอง ไม่สามารถสร้างบิลได้")
				}
			}
			if !status.IsOpen {
				return response.BadRequest(c, "ขณะนี้ปิดทำการ (ทอง) ไม่สามารถสร้างบิลได้")
			}
		}
		if hasSilver {
			// Silver follows its own schedule (enable + close-shop + daily cutoff).
			if !service.GetSilverSellStatus(ctrl.db).IsOpen {
				return response.BadRequest(c, "ขณะนี้ปิดรับซื้อเงิน ไม่สามารถสร้างบิลได้")
			}
		}
	}

	// Stamp the gold-price round for the document. Real-time only applies during
	// the real-time window; otherwise (incl. staff selling after hours) the latest
	// association round is used.
	if status.PriceMode == service.PriceModeRealtime {
		// Lock a snapshot of the real-time price for this document.
		req.GoldRound, req.GoldPriceID = service.SnapshotRealtimeRound(ctrl.db)
	} else {
		// Stamp the association gold-price round in effect now (for reporting).
		req.GoldRound, req.GoldPriceID = service.CurrentRound(ctrl.db)
	}

	// Store/branch record where the sale happened (the caller's), never from the
	// payload. For staff selling, CreatedBy is the chosen customer so the bill
	// shows up in their account; for the self-service flow it is the caller.
	if storeID := middleware.GetStoreID(c); storeID != nil {
		req.StoreID = storeID
	}
	if branchID := middleware.GetBranchID(c); branchID != nil {
		req.BranchID = branchID
	}
	if sellCustomer != nil {
		req.CreatedByUserID = sellCustomer.ID
	} else {
		req.CreatedByUserID = middleware.GetUserID(c)
	}

	bill, err := ctrl.billUsecase.CreateBill(&req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	if sellCustomer != nil {
		middleware.SetActivityDescription(c, fmt.Sprintf("ขายแทนลูกค้า %s (บิล %s)", sellCustomer.Name, bill.Code))
	} else {
		middleware.SetActivityDescription(c, fmt.Sprintf("ลูกค้าสร้างบิลขาย %s", bill.Code))
	}
	return response.Created(c, "Bill created", bill)
}

// resolveSellCustomer validates that the given id belongs to an active customer.
// Customers are a global pool (no store binding), so any staff member holding
// bills.sell may sell on behalf of any of them.
func (ctrl *BillController) resolveSellCustomer(customerID uint) (*entity.User, error) {
	var cust entity.User
	err := ctrl.db.
		Joins("JOIN roles ON roles.id = users.role_id").
		Where("users.id = ? AND roles.name = ?", customerID, "customer").
		First(&cust).Error
	if err != nil {
		return nil, errors.New("ไม่พบลูกค้าที่เลือก")
	}
	if !cust.IsActive {
		return nil, errors.New("ลูกค้ารายนี้ถูกปิดใช้งาน ไม่สามารถขายแทนได้")
	}
	return &cust, nil
}

// ListSellCustomers returns the customer pool for the staff "sell on behalf"
// picker. Unlike /customers it is not store-scoped (customers have no store
// binding), so owners/employees can find any active customer to sell for.
func (ctrl *BillController) ListSellCustomers(c *fiber.Ctx) error {
	search := strings.TrimSpace(c.Query("search"))
	q := ctrl.db.Model(&entity.User{}).
		Joins("JOIN roles ON roles.id = users.role_id").
		Where("roles.name = ? AND users.is_active = ?", "customer", true)
	if search != "" {
		like := "%" + search + "%"
		q = q.Where("users.name ILIKE ? OR users.email ILIKE ? OR users.phone ILIKE ?", like, like, like)
	}
	var customers []entity.User
	if err := q.Order("users.name ASC").Limit(50).Find(&customers).Error; err != nil {
		return response.InternalServerError(c, err.Error())
	}
	return response.Success(c, "ok", customers)
}

// billFilter reads the query params shared by the list and its summary, so a page
// of results and the totals shown above it always describe the same set.
func (ctrl *BillController) billFilter(c *fiber.Ctx) repository.BillFilter {
	storeID, branchID, createdBy := ctrl.scope(c)
	f := repository.BillFilter{
		StoreID:   storeID,
		BranchID:  branchID,
		CreatedBy: createdBy,
		Metal:     queryMetal(c),
		Search:    c.Query("search", ""),
	}
	if s := c.Query("status"); s != "" {
		st, _ := strconv.Atoi(s)
		f.Status = &st
	}
	// exclude_status=12,14 — the customer's รายการขาย hides finished bills, which
	// has to happen server-side or a page of results arrives half-empty.
	for _, part := range strings.Split(c.Query("exclude_status"), ",") {
		if st, err := strconv.Atoi(strings.TrimSpace(part)); err == nil {
			f.ExcludeStatuses = append(f.ExcludeStatuses, st)
		}
	}
	return f
}

func (ctrl *BillController) GetAllBills(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "10"))

	bills, total, err := ctrl.billUsecase.GetAllBills(ctrl.billFilter(c), page, limit)
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}
	return response.Paginated(c, "Bills retrieved", bills, page, limit, total)
}

// GetBillsSummary totals every bill matching the list's filters — the overview
// strip stays put while the user pages through the list underneath it.
func (ctrl *BillController) GetBillsSummary(c *fiber.Ctx) error {
	// Staff lists collapse bills issued together into one row; the customer's own
	// list shows each bill. The count must follow whichever the caller renders.
	groupIssued := middleware.GetRoleName(c) != "customer"
	summary, err := ctrl.billUsecase.SummarizeBills(ctrl.billFilter(c), groupIssued)
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}
	return response.Success(c, "ok", summary)
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

// GetUnfinishedCount feeds the sidebar badges: a combined total plus the per-metal
// split, one for each of the รายการขายทอง / รายการขายเงิน entries.
func (ctrl *BillController) GetUnfinishedCount(c *fiber.Ctx) error {
	storeID, branchID, createdBy := ctrl.scope(c)
	counts, err := ctrl.billUsecase.CountUnfinished(storeID, branchID, createdBy)
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}
	return response.Success(c, "ok", fiber.Map{
		"count":  counts.Total,
		"gold":   counts.Gold,
		"silver": counts.Silver,
	})
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
	// This bill just landed in the รอเคลียร์ pile — only its own metal's backlog
	// moved, so only that metal is evaluated for an alert.
	go service.SyncLineBacklogAlert(ctrl.db, bill.Metal, bill.StoreID, true)
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
	go ctrl.releaseLineLatch()
	return response.Success(c, "Bill reverted", bill)
}

// releaseLineLatch re-arms the LINE alert for both metals after an action that
// can only shrink the backlog (เคลียร์บิล / ยกเลิก / ดึงกลับไปแก้ไข / ลบบิล), so the
// next time the backlog climbs back to the threshold it alerts again. Never
// sends — only an approval can push a new alert.
func (ctrl *BillController) releaseLineLatch() {
	for _, metal := range service.LineMetals {
		service.SyncLineBacklogAlert(ctrl.db, metal, nil, false)
	}
}

// ClearBillsRequest optionally narrows the clear to specific bills; an empty
// (or absent) list keeps the old behaviour of clearing every completed bill.
type ClearBillsRequest struct {
	BillIDs []uint `json:"bill_ids"`
}

func (ctrl *BillController) ClearBills(c *fiber.Ctx) error {
	storeID, _, _ := ctrl.scope(c)
	var req ClearBillsRequest
	// BodyParser errors on an empty body — old clients POST with no payload.
	if len(c.Body()) > 0 {
		if err := c.BodyParser(&req); err != nil {
			return response.BadRequest(c, "Invalid request body")
		}
	}
	count, err := ctrl.billUsecase.ClearBills(storeID, req.BillIDs)
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}
	middleware.SetActivityDescription(c, fmt.Sprintf("เคลียร์บิลสำเร็จ %d บิล", count))
	go ctrl.releaseLineLatch()
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
	go ctrl.releaseLineLatch()
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
	go ctrl.releaseLineLatch()
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
