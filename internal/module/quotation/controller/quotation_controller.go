package controller

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"jk-api/internal/entity"
	"jk-api/internal/middleware"
	"jk-api/internal/module/quotation/usecase"
	"jk-api/internal/service"
	"jk-api/pkg/response"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type QuotationController struct {
	quotationUsecase usecase.QuotationUsecase
	db               *gorm.DB
}

func NewQuotationController(quotationUsecase usecase.QuotationUsecase, db *gorm.DB) *QuotationController {
	return &QuotationController{quotationUsecase: quotationUsecase, db: db}
}

func visibleQuotationCode(q *entity.Quotation) string {
	if q != nil && q.DisplayCode != "" {
		return q.DisplayCode
	}
	if q == nil {
		return ""
	}
	return q.Code
}

// tagQuotation points the activity log at the customer the quotation was issued
// for, so issuing/approving/editing/deleting it shows up on that customer's
// timeline rather than only the issuer's. The customer is the owner of the bill
// the quotation was issued against — resolved forward via BillID, or backward
// via the bills that link to this quotation as their issued one (a master may
// combine several of a customer's bills into one quotation).
func (ctrl *QuotationController) tagQuotation(c *fiber.Ctx, q *entity.Quotation) {
	if q == nil {
		return
	}
	middleware.SetActivityRef(c, visibleQuotationCode(q))

	var owners []uint
	// created_by is nullable, and Pluck into []uint cannot represent NULL — both
	// lookups therefore exclude it rather than scanning a null into a zero id.
	if q.BillID != nil {
		ctrl.db.Model(&entity.Quotation{}).
			Where("id = ? AND created_by IS NOT NULL", *q.BillID).
			Pluck("created_by", &owners)
	}
	if len(owners) == 0 {
		ctrl.db.Model(&entity.Quotation{}).
			Where("issued_quotation_id = ? AND created_by IS NOT NULL", q.ID).
			Distinct().Pluck("created_by", &owners)
	}
	// Exactly one owner, or the row would claim a customer it doesn't belong to.
	if len(owners) == 1 {
		middleware.SetActivityTarget(c, owners[0])
	}
}

// beforeTotal reads a pre-edit total, or 0 when the document could not be read.
func beforeTotal(q *entity.Quotation) float64 {
	if q == nil {
		return 0
	}
	return q.TotalAmount
}

// quotationLines flattens a quotation's items into the log's evidence shape:
// the price, weight and amount that were printed on the document.
func quotationLines(q *entity.Quotation) []map[string]any {
	if q == nil {
		return nil
	}
	lines := make([]map[string]any, 0, len(q.Items))
	for _, it := range q.Items {
		lines = append(lines, map[string]any{
			"type_name": it.TypeName,
			"metal":     it.Metal,
			"price":     it.Price,
			"percent":   it.Percent,
			"plus":      it.Plus,
			"weight":    it.Weight,
			"per_gram":  it.PerGram,
			"total":     it.Total,
		})
	}
	return lines
}

// GetLatestSignature returns the most recent signature image a given customer
// signed on any of their quotations, so the issuer can reuse it instead of
// asking the customer to re-sign. Returns an empty image_url when none exists.
func (ctrl *QuotationController) GetLatestSignature(c *fiber.Ctx) error {
	createdBy, err := strconv.ParseUint(c.Query("created_by"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "created_by is required")
	}
	var urls []string
	ctrl.db.Table("quotation_images AS qi").
		Joins("JOIN quotations q ON q.id = qi.quotation_id").
		Where("q.created_by = ? AND qi.type = ? AND qi.deleted_at IS NULL AND q.deleted_at IS NULL", uint(createdBy), "signature").
		Order("qi.id DESC").
		Limit(1).
		Pluck("qi.image_url", &urls)
	url := ""
	if len(urls) > 0 {
		url = urls[0]
	}
	return response.Success(c, "Latest signature", fiber.Map{"image_url": url})
}

func (ctrl *QuotationController) CreateQuotation(c *fiber.Ctx) error {
	var req usecase.CreateQuotationRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}
	if len(req.Items) == 0 {
		return response.BadRequest(c, "ต้องมีรายการอย่างน้อย 1 รายการ")
	}

	// Block creation when closed, unless the caller holds sales.bypass (master always does).
	status := service.GetSalesStatus(ctrl.db)
	if !status.IsOpen && !middleware.HasPermission(ctrl.db, c, "sales.bypass") {
		return response.BadRequest(c, "ขณะนี้ปิดทำการ ไม่สามารถออกใบเสนอราคาได้")
	}
	if status.PriceMode == service.PriceModeRealtime {
		// Lock a snapshot of the real-time price for this document.
		req.GoldRound, req.GoldPriceID = service.SnapshotRealtimeRound(ctrl.db)
	} else {
		// Stamp the association gold-price round in effect now (for reporting).
		req.GoldRound, req.GoldPriceID = service.CurrentRound(ctrl.db)
	}

	// Always derive store/branch from JWT token based on role
	roleName := middleware.GetRoleName(c)
	switch roleName {
	case "owner":
		// Owner is tied to their store; they pick which branch's header to use.
		if storeID := middleware.GetStoreID(c); storeID != nil {
			req.StoreID = storeID
		}
		req.BranchID = req.PayloadBranchID
	case "employee":
		// Employee is locked to their assigned store AND branch.
		if storeID := middleware.GetStoreID(c); storeID != nil {
			req.StoreID = storeID
		}
		if branchID := middleware.GetBranchID(c); branchID != nil {
			req.BranchID = branchID
		}
	default:
		// master: not tied to a store — use the store + branch chosen in the payload.
		req.StoreID = req.PayloadStoreID
		req.BranchID = req.PayloadBranchID
	}

	// The receipt header now lives on the branch (each branch prints its own).
	// Fall back to the store's main branch when none was chosen, then snapshot the
	// header onto the quotation so reprints stay accurate even if the branch's
	// info changes later. Skipped entirely when the caller opted out (no_header) —
	// the document prints headerless; the store/branch links above still stand.
	if !req.NoHeader {
		if req.BranchID == nil && req.StoreID != nil {
			var main entity.Branch
			if err := ctrl.db.Where("store_id = ? AND is_main = ?", *req.StoreID, true).
				First(&main).Error; err == nil {
				req.BranchID = &main.ID
			}
		}
		if req.BranchID != nil {
			var branch entity.Branch
			if err := ctrl.db.First(&branch, *req.BranchID).Error; err == nil {
				req.StoreName = branch.HeaderName
				req.StoreBranch = branch.Name
				req.StoreAddress = branch.Address
				req.StorePhone = branch.Phone
				req.StoreTaxID = branch.TaxID
				req.StoreTaxName = branch.TaxName
				req.StoreWebsite = branch.Website
				req.StoreLogo = branch.Logo
				// Keep the store link consistent with the branch (master may have sent
				// only a branch id).
				if req.StoreID == nil {
					req.StoreID = &branch.StoreID
				}
			}
		}
	}

	// Quotations from roles holding credits.use deduct credits on creation.
	// Use strict lookup (no master shortcut) since credits.use is a constraint, not a privilege.
	req.UsesCredits = middleware.HasPermissionStrict(ctrl.db, c, "credits.use")
	req.CreatedByUserID = middleware.GetUserID(c)
	// Prefix belongs to the role that starts the document: only master/admin
	// creates P documents; owner/employee/customer-origin documents use BILL.
	req.UseBillCode = !middleware.IsMaster(c)

	quotation, err := ctrl.quotationUsecase.CreateQuotation(&req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	middleware.SetActivityDescription(c, fmt.Sprintf(
		"สร้างใบเสนอราคา %s ให้ %s (%d รายการ รวม %.2f บาท)",
		visibleQuotationCode(quotation), quotation.SignerName, len(quotation.Items), quotation.TotalAmount,
	))
	ctrl.tagQuotation(c, quotation)
	middleware.SetActivityDetail(c, map[string]any{
		"kind":         "issue_quotation",
		"code":         visibleQuotationCode(quotation),
		"quotation_id": quotation.ID,
		"total_amount": quotation.TotalAmount,
		"items":        quotationLines(quotation),
	})
	return response.Created(c, "Quotation created", quotation)
}

func (ctrl *QuotationController) GetAllQuotations(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "10"))
	search := c.Query("search", "")

	var storeID, branchID, createdBy *uint
	var status *int

	if !middleware.IsMaster(c) {
		storeID = middleware.GetStoreID(c)
		roleName := middleware.GetRoleName(c)
		if roleName != "owner" {
			branchID = middleware.GetBranchID(c)
		}
		// Employee sees only their own quotations
		if roleName == "employee" {
			userID := middleware.GetUserID(c)
			createdBy = &userID
		}
	} else {
		if sid := c.Query("store_id"); sid != "" {
			id, _ := strconv.ParseUint(sid, 10, 32)
			uid := uint(id)
			storeID = &uid
		}
		if bid := c.Query("branch_id"); bid != "" {
			id, _ := strconv.ParseUint(bid, 10, 32)
			uid := uint(id)
			branchID = &uid
		}
	}

	if s := c.Query("status"); s != "" {
		st, _ := strconv.Atoi(s)
		status = &st
	}

	if cbId := c.Query("created_by"); cbId != "" {
		id, _ := strconv.ParseUint(cbId, 10, 32)
		uid := uint(id)
		createdBy = &uid
	}

	quotations, total, err := ctrl.quotationUsecase.GetAllQuotations(storeID, branchID, createdBy, status, page, limit, search)
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}
	return response.Paginated(c, "Quotations retrieved", quotations, page, limit, total)
}

func (ctrl *QuotationController) GetQuotationByID(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid quotation ID")
	}

	quotation, err := ctrl.quotationUsecase.GetQuotationByID(uint(id))
	if err != nil {
		return response.NotFound(c, "Quotation not found")
	}
	return response.Success(c, "Quotation retrieved", quotation)
}

func (ctrl *QuotationController) UpdateQuotationStatus(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid quotation ID")
	}

	var req usecase.UpdateStatusRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	// Authorization is governed entirely by the quotations.update permission on
	// this route — no extra role-name gate. Approving (status=1) is part of that
	// permission's scope ("แก้ไข/อนุมัติ/ยกเลิก"), and so is cancelling one that is
	// already approved: quotations are auto-approved on creation, so a master-only
	// rule there would leave an owner with no way to void a quotation but to delete
	// it — which used to strand the credit it charged.
	quotation, err := ctrl.quotationUsecase.UpdateQuotationStatus(uint(id), &req, true)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	switch req.Status {
	case 1:
		middleware.SetActivityDescription(c, fmt.Sprintf("อนุมัติใบเสนอราคา %s", visibleQuotationCode(quotation)))
	case 2:
		middleware.SetActivityDescription(c, fmt.Sprintf("ยกเลิกใบเสนอราคา %s (%s)", visibleQuotationCode(quotation), quotation.RejectReason))
	}
	ctrl.tagQuotation(c, quotation)
	return response.Success(c, "Quotation updated", quotation)
}

func (ctrl *QuotationController) UpdateQuotation(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid quotation ID")
	}

	var req usecase.UpdateQuotationRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	// Read the document as issued before overwriting it — an edit to an already
	// printed quotation is exactly the change a customer would later dispute.
	before, _ := ctrl.quotationUsecase.GetQuotationByID(uint(id))

	// Only master may edit a quotation that is already approved/cancelled.
	quotation, err := ctrl.quotationUsecase.UpdateQuotation(uint(id), &req, middleware.IsMaster(c))
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	middleware.SetActivityDescription(c, fmt.Sprintf(
		"แก้ไขใบเสนอราคา %s (รวม %.2f บาท)", visibleQuotationCode(quotation), quotation.TotalAmount,
	))
	ctrl.tagQuotation(c, quotation)
	middleware.SetActivityDetail(c, map[string]any{
		"kind":         "edit_quotation",
		"code":         visibleQuotationCode(quotation),
		"quotation_id": quotation.ID,
		"total_before": beforeTotal(before),
		"total_amount": quotation.TotalAmount,
		"before":       quotationLines(before),
		"after":        quotationLines(quotation),
	})
	return response.Success(c, "Quotation updated", quotation)
}

// UpdatePaymentMethod records the ชำระโดย tick (cash | transfer | "") on an issued
// quotation. Separate from the content edit so it never rewrites items/member.
func (ctrl *QuotationController) UpdatePaymentMethod(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid quotation ID")
	}
	var req struct {
		PaymentMethod string `json:"payment_method"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}
	quotation, err := ctrl.quotationUsecase.SetPaymentMethod(uint(id), req.PaymentMethod)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	payLabel := map[string]string{"cash": "เงินสด", "transfer": "โอนเงิน"}[req.PaymentMethod]
	if payLabel == "" {
		payLabel = "ยังไม่ระบุ"
	}
	middleware.SetActivityDescription(c, fmt.Sprintf(
		"ระบุการชำระเงินใบเสนอราคา %s เป็น %s", visibleQuotationCode(quotation), payLabel,
	))
	ctrl.tagQuotation(c, quotation)
	return response.Success(c, "Payment method updated", quotation)
}

func (ctrl *QuotationController) DeleteQuotation(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid quotation ID")
	}

	// Fetch first to capture the code for the activity log (DeleteQuotation only returns an error).
	if quotation, err := ctrl.quotationUsecase.GetQuotationByID(uint(id)); err == nil {
		middleware.SetActivityDescription(c, fmt.Sprintf(
			"ลบใบเสนอราคา %s (รวม %.2f บาท)", visibleQuotationCode(quotation), quotation.TotalAmount,
		))
		ctrl.tagQuotation(c, quotation)
		// Keep the document's contents in the log; after this the quotation is gone.
		middleware.SetActivityDetail(c, map[string]any{
			"kind":         "delete_quotation",
			"code":         visibleQuotationCode(quotation),
			"quotation_id": quotation.ID,
			"total_amount": quotation.TotalAmount,
			"items":        quotationLines(quotation),
		})
	}

	if err := ctrl.quotationUsecase.DeleteQuotation(uint(id)); err != nil {
		return response.NotFound(c, err.Error())
	}
	return response.Success(c, "Quotation deleted", nil)
}

func (ctrl *QuotationController) PreviewCreditReset(c *fiber.Ctx) error {
	memberID, err := strconv.ParseUint(c.Params("memberId"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid member ID")
	}

	preview, err := ctrl.quotationUsecase.PreviewCreditReset(uint(memberID))
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.Success(c, "Preview retrieved", preview)
}

func (ctrl *QuotationController) ResetMemberCredit(c *fiber.Ctx) error {
	memberID, err := strconv.ParseUint(c.Params("memberId"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid member ID")
	}

	actingUserID := middleware.GetUserID(c)
	result, err := ctrl.quotationUsecase.ResetMemberCredit(uint(memberID), actingUserID)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	middleware.SetActivityDescription(c, fmt.Sprintf("รีเซ็ตเครดิตให้สมาชิก #%d จำนวน %d ใบ รวม %.2f บาท", memberID, result.Count, result.Amount))
	return response.Success(c, "Credit reset", result)
}

func (ctrl *QuotationController) UploadImages(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid quotation ID")
	}

	form, err := c.MultipartForm()
	if err != nil {
		return response.BadRequest(c, "Invalid form data")
	}

	files := form.File["images"]
	if len(files) == 0 {
		return response.BadRequest(c, "No images provided")
	}

	// Image category (before_melt / after_melt / signature). Optional.
	imageType := c.FormValue("type")
	if imageType == "" {
		imageType = c.Query("type")
	}

	dir := fmt.Sprintf("./uploads/quotations/%d", id)
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
		urls = append(urls, fmt.Sprintf("/uploads/quotations/%d/%s", id, filename))
	}

	if err := ctrl.quotationUsecase.AddImages(uint(id), urls, imageType); err != nil {
		return response.InternalServerError(c, err.Error())
	}
	return response.Success(c, "Images uploaded", urls)
}

func (ctrl *QuotationController) ExportQuotation(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid quotation ID")
	}

	quotation, err := ctrl.quotationUsecase.GetQuotationByID(uint(id))
	if err != nil {
		return response.NotFound(c, "Quotation not found")
	}

	c.Set("Content-Type", "text/csv; charset=utf-8")
	c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=quotation_%s.csv", visibleQuotationCode(quotation)))

	csv := "\xEF\xBB\xBF"
	csv += "ใบเสนอราคา," + visibleQuotationCode(quotation) + "\n"
	csv += "วันที่," + quotation.CreatedAt.Format("02/01/2006 15:04") + "\n"
	if quotation.Store != nil {
		csv += "ร้าน," + quotation.Store.Name + "\n"
	}
	if quotation.Branch != nil {
		csv += "สาขา," + quotation.Branch.Name + "\n"
	}
	if quotation.Member != nil {
		csv += "ลูกค้า," + quotation.Member.Fname + " " + quotation.Member.Lname + "\n"
	}
	csv += "\n"
	csv += "ประเภท,ราคา,เปอร์เซ็นต์,น้ำหนัก,ราคา/กรัม,บวกเพิ่ม,รวม\n"

	for _, item := range quotation.Items {
		csv += fmt.Sprintf("%s,%.2f,%.4f,%.4f,%.2f,%.2f,%.2f\n",
			item.TypeName, item.Price, item.Percent, item.Weight, item.PerGram, item.Plus, item.Total)
	}

	csv += fmt.Sprintf("\nรวมทั้งหมด,,,,,,%.2f\n", quotation.TotalAmount)

	return c.SendString(csv)
}
