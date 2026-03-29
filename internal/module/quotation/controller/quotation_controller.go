package controller

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"jk-api/internal/middleware"
	"jk-api/internal/module/quotation/usecase"
	"jk-api/pkg/response"

	"github.com/gofiber/fiber/v2"
)

type QuotationController struct {
	quotationUsecase usecase.QuotationUsecase
}

func NewQuotationController(quotationUsecase usecase.QuotationUsecase) *QuotationController {
	return &QuotationController{quotationUsecase: quotationUsecase}
}

func (ctrl *QuotationController) CreateQuotation(c *fiber.Ctx) error {
	var req usecase.CreateQuotationRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}
	if len(req.Items) == 0 {
		return response.BadRequest(c, "ต้องมีรายการอย่างน้อย 1 รายการ")
	}

	// Always derive store/branch from JWT token based on role
	roleName := middleware.GetRoleName(c)
	switch roleName {
	case "owner":
		// Owner must use their own store; branch is optional
		if storeID := middleware.GetStoreID(c); storeID != nil {
			req.StoreID = storeID
		}
	case "branch", "employee":
		// Branch/Employee must use their assigned store AND branch
		if storeID := middleware.GetStoreID(c); storeID != nil {
			req.StoreID = storeID
		}
		if branchID := middleware.GetBranchID(c); branchID != nil {
			req.BranchID = branchID
		}
	// master: StoreID/BranchID remain nil (allowed)
	}

	// master and owner can approve — auto-approve their own quotations
	req.AutoApprove = roleName == "master" || roleName == "owner"
	req.CreatedByUserID = middleware.GetUserID(c)

	quotation, err := ctrl.quotationUsecase.CreateQuotation(&req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
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

	// Only owner/master can approve (status=1)
	if req.Status == 1 {
		roleName := middleware.GetRoleName(c)
		if roleName != "master" && roleName != "owner" {
			return response.Forbidden(c, "ไม่มีสิทธิ์อนุมัติใบเสนอราคา")
		}
	}

	quotation, err := ctrl.quotationUsecase.UpdateQuotationStatus(uint(id), &req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
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

	quotation, err := ctrl.quotationUsecase.UpdateQuotation(uint(id), &req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.Success(c, "Quotation updated", quotation)
}

func (ctrl *QuotationController) DeleteQuotation(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid quotation ID")
	}

	if err := ctrl.quotationUsecase.DeleteQuotation(uint(id)); err != nil {
		return response.NotFound(c, err.Error())
	}
	return response.Success(c, "Quotation deleted", nil)
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

	if err := ctrl.quotationUsecase.AddImages(uint(id), urls); err != nil {
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
	c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=quotation_%s.csv", quotation.Code))

	csv := "\xEF\xBB\xBF"
	csv += "ใบเสนอราคา," + quotation.Code + "\n"
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
