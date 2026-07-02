package controller

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"jk-api/internal/entity"
	"jk-api/internal/middleware"
	"jk-api/internal/module/customer/usecase"
	"jk-api/pkg/response"
	"jk-api/pkg/upload"

	"github.com/gofiber/fiber/v2"
)

// Allowed customer-document extensions: images, PDF, Word, Excel.
var allowedDocExts = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".webp": true, ".gif": true,
	".pdf": true, ".docx": true, ".xlsx": true,
}

type CustomerController struct {
	customerUsecase usecase.CustomerUsecase
}

func NewCustomerController(customerUsecase usecase.CustomerUsecase) *CustomerController {
	return &CustomerController{customerUsecase: customerUsecase}
}

func (ctrl *CustomerController) CreateCustomer(c *fiber.Ctx) error {
	var req usecase.CreateCustomerRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}
	customer, err := ctrl.customerUsecase.CreateCustomer(&req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.Created(c, "Customer created", customer)
}

func (ctrl *CustomerController) GetAllCustomers(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "10"))
	search := c.Query("search", "")

	var storeID, branchID *uint
	roleName := middleware.GetRoleName(c)
	switch roleName {
	case "master":
		if v := c.Query("store_id"); v != "" {
			if id, err := strconv.ParseUint(v, 10, 32); err == nil {
				uid := uint(id)
				storeID = &uid
			}
		}
		if v := c.Query("branch_id"); v != "" {
			if id, err := strconv.ParseUint(v, 10, 32); err == nil {
				uid := uint(id)
				branchID = &uid
			}
		}
	case "owner":
		storeID = middleware.GetStoreID(c)
		if v := c.Query("branch_id"); v != "" {
			if id, err := strconv.ParseUint(v, 10, 32); err == nil {
				uid := uint(id)
				branchID = &uid
			}
		}
	default: // employee
		storeID = middleware.GetStoreID(c)
		branchID = middleware.GetBranchID(c)
	}

	customers, total, err := ctrl.customerUsecase.GetAllCustomers(page, limit, storeID, branchID, search)
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}
	return response.Paginated(c, "Customers retrieved", customers, page, limit, total)
}

func (ctrl *CustomerController) GetCustomerByID(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid customer ID")
	}
	customer, err := ctrl.customerUsecase.GetCustomerByID(uint(id))
	if err != nil {
		return response.NotFound(c, "Customer not found")
	}
	return response.Success(c, "Customer retrieved", customer)
}

func (ctrl *CustomerController) UpdateCustomer(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid customer ID")
	}
	var req usecase.UpdateCustomerRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}
	customer, err := ctrl.customerUsecase.UpdateCustomer(uint(id), &req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.Success(c, "Customer updated", customer)
}

func (ctrl *CustomerController) DeleteCustomer(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid customer ID")
	}
	if err := ctrl.customerUsecase.DeleteCustomer(uint(id)); err != nil {
		return response.NotFound(c, err.Error())
	}
	return response.Success(c, "Customer deleted", nil)
}

// UploadAvatar sets the customer's profile picture (multipart field "avatar").
func (ctrl *CustomerController) UploadAvatar(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid customer ID")
	}
	if _, err := ctrl.customerUsecase.GetCustomerByID(uint(id)); err != nil {
		return response.NotFound(c, "Customer not found")
	}
	path, err := upload.SaveFile(c, "avatar", fmt.Sprintf("customers/%d", id))
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	customer, err := ctrl.customerUsecase.UpdateAvatar(uint(id), path)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.Success(c, "Avatar uploaded", customer)
}

// UploadDocuments accepts multipart field "files" (multiple) and stores them
// under ./uploads/customers/{id}/. Images, PDF, DOCX and XLSX only.
func (ctrl *CustomerController) UploadDocuments(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid customer ID")
	}
	if _, err := ctrl.customerUsecase.GetCustomerByID(uint(id)); err != nil {
		return response.NotFound(c, "Customer not found")
	}

	form, err := c.MultipartForm()
	if err != nil {
		return response.BadRequest(c, "Invalid form data")
	}
	files := form.File["files"]
	if len(files) == 0 {
		return response.BadRequest(c, "ไม่พบไฟล์ที่อัปโหลด")
	}
	// Validate every file before saving any, so a bad file rejects the batch.
	for _, file := range files {
		ext := strings.ToLower(filepath.Ext(file.Filename))
		if !allowedDocExts[ext] {
			return response.BadRequest(c, fmt.Sprintf("ไฟล์ %s ไม่รองรับ (รองรับ รูปภาพ, PDF, DOCX, XLSX)", file.Filename))
		}
	}

	dir := fmt.Sprintf("./uploads/customers/%d", id)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return response.InternalServerError(c, "Failed to create directory")
	}

	uploadedBy := middleware.GetUserID(c)
	var docs []entity.CustomerDocument
	for _, file := range files {
		ext := strings.ToLower(filepath.Ext(file.Filename))
		filename := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
		savePath := filepath.Join(dir, filename)
		if err := c.SaveFile(file, savePath); err != nil {
			return response.InternalServerError(c, "Failed to save file")
		}
		doc := entity.CustomerDocument{
			UserID:     uint(id),
			FileName:   file.Filename,
			FilePath:   fmt.Sprintf("/uploads/customers/%d/%s", id, filename),
			FileExt:    strings.TrimPrefix(ext, "."),
			FileSize:   file.Size,
			UploadedBy: &uploadedBy,
		}
		if err := ctrl.customerUsecase.AddDocument(&doc); err != nil {
			return response.InternalServerError(c, err.Error())
		}
		docs = append(docs, doc)
	}
	return response.Created(c, "Documents uploaded", docs)
}

func (ctrl *CustomerController) GetDocuments(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid customer ID")
	}
	docs, err := ctrl.customerUsecase.GetDocuments(uint(id))
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}
	return response.Success(c, "Documents retrieved", docs)
}

func (ctrl *CustomerController) DeleteDocument(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid customer ID")
	}
	docID, err := strconv.ParseUint(c.Params("docId"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid document ID")
	}
	doc, err := ctrl.customerUsecase.GetDocumentByID(uint(docID))
	if err != nil || doc.UserID != uint(id) {
		return response.NotFound(c, "Document not found")
	}
	if err := ctrl.customerUsecase.DeleteDocument(uint(docID)); err != nil {
		return response.InternalServerError(c, err.Error())
	}
	// Best-effort file removal; the DB row is the source of truth.
	_ = os.Remove("." + doc.FilePath)
	return response.Success(c, "Document deleted", nil)
}
