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

// parseBodyStoreID reads the optional store_id a master may send when creating or
// moving a customer. It is read separately from the usecase request, whose StoreID
// is json:"-" so a store-bound caller cannot smuggle another store in.
func parseBodyStoreID(c *fiber.Ctx) *uint {
	var body struct {
		StoreID *uint `json:"store_id"`
	}
	if err := c.BodyParser(&body); err != nil {
		return nil
	}
	return body.StoreID
}

// callerStoreScope returns the store a caller is confined to, or nil for master
// (who works across every store). Customers are a STORE-level record: every
// branch of a store shares one customer list, but another store must not see it.
// Branch is never part of this — only role `employee` even has a branch.
//
// The error case matters: a non-master whose account carries no store must NOT
// fall through to the master path, or an unassigned owner would silently see
// every store's customers.
func callerStoreScope(c *fiber.Ctx) (*uint, error) {
	if middleware.GetRoleName(c) == "master" {
		return nil, nil
	}
	storeID := middleware.GetStoreID(c)
	if storeID == nil {
		return nil, response.Forbidden(c, "บัญชีนี้ยังไม่ได้ผูกกับร้าน จึงดูข้อมูลลูกค้าไม่ได้")
	}
	return storeID, nil
}

// guardCustomerStore refuses a customer belonging to another store. The list is
// filtered already; this closes the door of reaching one straight by id.
func (ctrl *CustomerController) guardCustomerStore(c *fiber.Ctx, cust *entity.User) error {
	scope, errResp := callerStoreScope(c)
	if errResp != nil {
		return errResp
	}
	if scope == nil {
		return nil
	}
	if cust.StoreID == nil || *cust.StoreID != *scope {
		// 404 rather than 403 — a customer of another store should not be
		// confirmed to exist at all.
		return response.NotFound(c, "Customer not found")
	}
	return nil
}

func (ctrl *CustomerController) CreateCustomer(c *fiber.Ctx) error {
	var req usecase.CreateCustomerRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}
	// ร้านของลูกค้ามาจากผู้เรียก ไม่ใช่จาก body ที่ client ส่งมาดื้อ ๆ: staff ผูกกับ
	// ร้านของตัวเองเสมอ, master เลือกร้านได้ (ไม่ส่งมาและมีร้านเดียว usecase เติมให้)
	scope, errResp := callerStoreScope(c)
	if errResp != nil {
		return errResp
	}
	if scope != nil {
		req.StoreID = scope
	} else {
		req.StoreID = parseBodyStoreID(c)
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

	// ลูกค้าเป็นข้อมูลระดับ "ร้าน": ทุกสาขาในร้านเดียวกันเห็นชุดเดียวกัน แต่ร้านอื่น
	// ต้องไม่เห็น. ไม่กรองด้วย branch_id เด็ดขาด — มีแค่ role employee ที่มีสาขา
	// (requiresBranch ใน user usecase) ลูกค้าไม่มี การกรองสาขาจึงได้ 0 แถวเสมอ.
	storeID, errResp := callerStoreScope(c)
	if errResp != nil {
		return errResp
	}
	if storeID == nil {
		// master: ดูได้ทุกร้าน หรือเจาะร้านเดียวด้วย ?store_id=
		if v := c.Query("store_id"); v != "" {
			if id, err := strconv.ParseUint(v, 10, 32); err == nil {
				uid := uint(id)
				storeID = &uid
			}
		}
	}

	customers, total, err := ctrl.customerUsecase.GetAllCustomers(page, limit, storeID, search)
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}
	return response.Paginated(c, "Customers retrieved", customers, page, limit, total)
}

func (ctrl *CustomerController) GetCustomerByID(c *fiber.Ctx) error {
	customer, errResp := ctrl.loadCustomerParam(c)
	if errResp != nil {
		return errResp
	}
	return response.Success(c, "Customer retrieved", customer)
}

func (ctrl *CustomerController) UpdateCustomer(c *fiber.Ctx) error {
	existing, errResp := ctrl.loadCustomerParam(c)
	if errResp != nil {
		return errResp
	}
	var req usecase.UpdateCustomerRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}
	// ย้ายลูกค้าข้ามร้านได้เฉพาะ master — staff แก้ลูกค้าของร้านตัวเองได้ แต่ย้ายออกไป
	// ร้านอื่น (ซึ่งเท่ากับทำให้ตัวเองมองไม่เห็นอีก) ไม่ได้
	if scope, _ := callerStoreScope(c); scope == nil {
		req.StoreID = parseBodyStoreID(c)
	}
	customer, err := ctrl.customerUsecase.UpdateCustomer(existing.ID, &req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.Success(c, "Customer updated", customer)
}

func (ctrl *CustomerController) DeleteCustomer(c *fiber.Ctx) error {
	cust, errResp := ctrl.loadCustomerParam(c)
	if errResp != nil {
		return errResp
	}
	if err := ctrl.customerUsecase.DeleteCustomer(cust.ID); err != nil {
		return response.NotFound(c, err.Error())
	}
	return response.Success(c, "Customer deleted", nil)
}

// UploadAvatar sets the customer's profile picture (multipart field "avatar").
func (ctrl *CustomerController) UploadAvatar(c *fiber.Ctx) error {
	cust, errResp := ctrl.loadCustomerParam(c)
	if errResp != nil {
		return errResp
	}
	id := cust.ID
	path, err := upload.SaveFile(c, "avatar", fmt.Sprintf("customers/%d", id))
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	customer, err := ctrl.customerUsecase.UpdateAvatar(id, path)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.Success(c, "Avatar uploaded", customer)
}

// customerIDParam reads :id off a staff-facing customer route and checks the
// customer exists.
func (ctrl *CustomerController) customerIDParam(c *fiber.Ctx) (uint, error) {
	cust, errResp := ctrl.loadCustomerParam(c)
	if errResp != nil {
		return 0, errResp
	}
	return cust.ID, nil
}

// loadCustomerParam reads :id off a staff-facing route, loads the customer and
// refuses one belonging to another store. Every staff entry point to a single
// customer goes through here, so the store boundary is enforced in one place.
func (ctrl *CustomerController) loadCustomerParam(c *fiber.Ctx) (*entity.User, error) {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return nil, response.BadRequest(c, "Invalid customer ID")
	}
	cust, err := ctrl.customerUsecase.GetCustomerByID(uint(id))
	if err != nil {
		return nil, response.NotFound(c, "Customer not found")
	}
	if errResp := ctrl.guardCustomerStore(c, cust); errResp != nil {
		return nil, errResp
	}
	return cust, nil
}

// UploadDocuments accepts multipart field "files" (multiple) plus an optional
// "document_type_id" naming what the batch is (บัตรประชาชน, เล่มบัญชี, ...), and
// stores the files under ./uploads/customers/{id}/. Images, PDF, DOCX and XLSX only.
func (ctrl *CustomerController) UploadDocuments(c *fiber.Ctx) error {
	id, errResp := ctrl.customerIDParam(c)
	if errResp != nil {
		return errResp
	}
	return ctrl.uploadDocumentsFor(c, id)
}

// UploadMyDocuments is the customer-facing twin of UploadDocuments: a logged-in
// customer uploads onto their own record, so the target is the token's user id
// rather than a path param (customers hold no customers.update permission).
func (ctrl *CustomerController) UploadMyDocuments(c *fiber.Ctx) error {
	return ctrl.uploadDocumentsFor(c, middleware.GetUserID(c))
}

func (ctrl *CustomerController) uploadDocumentsFor(c *fiber.Ctx, userID uint) error {
	if userID == 0 {
		return response.BadRequest(c, "Invalid customer ID")
	}

	form, err := c.MultipartForm()
	if err != nil {
		return response.BadRequest(c, "Invalid form data")
	}
	files := form.File["files"]
	if len(files) == 0 {
		return response.BadRequest(c, "ไม่พบไฟล์ที่อัปโหลด")
	}

	// One type per upload batch — the whole batch is labelled the same thing.
	var docTypeID *uint
	var docType *entity.DocumentType
	if raw := c.FormValue("document_type_id"); raw != "" {
		parsed, err := strconv.ParseUint(raw, 10, 32)
		if err != nil {
			return response.BadRequest(c, "ประเภทเอกสารไม่ถูกต้อง")
		}
		typeID := uint(parsed)
		docType, err = ctrl.customerUsecase.GetActiveDocumentType(typeID)
		if err != nil {
			return response.BadRequest(c, "ไม่พบประเภทเอกสารที่เลือก")
		}
		docTypeID = &typeID
	}

	// A high-priority type stands for one physical document, so it holds exactly
	// one file and a fresh upload replaces the copy already on file — that is the
	// "เปลี่ยนเอกสาร" path a customer gets in place of delete.
	highPriority := docType != nil && docType.IsHighPriority
	if highPriority && len(files) > 1 {
		return response.BadRequest(c, fmt.Sprintf("%s อัปโหลดได้ครั้งละ 1 ไฟล์", docType.Name))
	}

	// Validate every file before saving any, so a bad file rejects the batch.
	for _, file := range files {
		ext := strings.ToLower(filepath.Ext(file.Filename))
		if !allowedDocExts[ext] {
			return response.BadRequest(c, fmt.Sprintf("ไฟล์ %s ไม่รองรับ (รองรับ รูปภาพ, PDF, DOCX, XLSX)", file.Filename))
		}
	}

	dir := fmt.Sprintf("./uploads/customers/%d", userID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return response.InternalServerError(c, "Failed to create directory")
	}

	if highPriority {
		if err := ctrl.customerUsecase.ReplaceDocumentsOfType(userID, *docTypeID); err != nil {
			return response.InternalServerError(c, err.Error())
		}
	}

	uploadedBy := middleware.GetUserID(c)
	// Only a high-priority copy waits for review; everything else needs no check.
	approval := entity.ApprovalApproved
	if highPriority {
		approval = entity.ApprovalPending
	}
	var docs []entity.CustomerDocument
	for _, file := range files {
		ext := strings.ToLower(filepath.Ext(file.Filename))
		filename := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
		savePath := filepath.Join(dir, filename)
		if err := c.SaveFile(file, savePath); err != nil {
			return response.InternalServerError(c, "Failed to save file")
		}
		doc := entity.CustomerDocument{
			UserID:         userID,
			FileName:       file.Filename,
			FilePath:       fmt.Sprintf("/uploads/customers/%d/%s", userID, filename),
			FileExt:        strings.TrimPrefix(ext, "."),
			FileSize:       file.Size,
			DocumentTypeID: docTypeID,
			ApprovalStatus: approval,
			UploadedBy:     &uploadedBy,
		}
		if err := ctrl.customerUsecase.AddDocument(&doc); err != nil {
			return response.InternalServerError(c, err.Error())
		}
		doc.DocumentType = docType
		docs = append(docs, doc)
	}

	// Staff hear about it only when the customer sent it in themselves. A staff
	// member uploading on the customer's behalf is already looking at the file, so
	// notifying the whole shop about their own action would be noise — the copy
	// still waits for a second pair of eyes either way.
	if highPriority && uploadedBy == userID {
		if customer, err := ctrl.customerUsecase.GetCustomerByID(userID); err == nil {
			ctrl.customerUsecase.NotifyDocumentReview(customer, docType.Name)
		}
	}
	return response.Created(c, "Documents uploaded", docs)
}

func (ctrl *CustomerController) GetDocuments(c *fiber.Ctx) error {
	id, errResp := ctrl.customerIDParam(c)
	if errResp != nil {
		return errResp
	}
	return ctrl.getDocumentsFor(c, id)
}

// GetMyDocuments lists the logged-in customer's own documents.
func (ctrl *CustomerController) GetMyDocuments(c *fiber.Ctx) error {
	return ctrl.getDocumentsFor(c, middleware.GetUserID(c))
}

func (ctrl *CustomerController) getDocumentsFor(c *fiber.Ctx, userID uint) error {
	docs, err := ctrl.customerUsecase.GetDocuments(userID)
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}
	return response.Success(c, "Documents retrieved", docs)
}

func (ctrl *CustomerController) DeleteDocument(c *fiber.Ctx) error {
	id, errResp := ctrl.customerIDParam(c)
	if errResp != nil {
		return errResp
	}
	return ctrl.deleteDocumentFor(c, id, true)
}

// DeleteMyDocument lets a customer remove a document off their own record. The
// owner check inside deleteDocumentFor is what keeps one customer out of another's
// files — the route itself is open to any authenticated user.
func (ctrl *CustomerController) DeleteMyDocument(c *fiber.Ctx) error {
	return ctrl.deleteDocumentFor(c, middleware.GetUserID(c), false)
}

// allowHighPriority separates the two callers: staff may remove anything, while a
// customer may not delete their own identity papers — re-uploading the type
// replaces the copy instead, which keeps a document on file at all times.
func (ctrl *CustomerController) deleteDocumentFor(c *fiber.Ctx, userID uint, allowHighPriority bool) error {
	docID, err := strconv.ParseUint(c.Params("docId"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid document ID")
	}
	doc, err := ctrl.customerUsecase.GetDocumentByID(uint(docID))
	if err != nil || userID == 0 || doc.UserID != userID {
		return response.NotFound(c, "Document not found")
	}
	if !allowHighPriority && doc.DocumentType != nil && doc.DocumentType.IsHighPriority {
		return response.BadRequest(c, fmt.Sprintf("%s เป็นเอกสารสำคัญ ลบไม่ได้ — อัปโหลดใหม่เพื่อเปลี่ยนแทน", doc.DocumentType.Name))
	}
	if err := ctrl.customerUsecase.DeleteDocument(uint(docID)); err != nil {
		return response.InternalServerError(c, err.Error())
	}
	// Best-effort file removal; the DB row is the source of truth.
	_ = os.Remove("." + doc.FilePath)
	return response.Success(c, "Document deleted", nil)
}

// ApproveDocument marks a high-priority document as checked and correct, which is
// what turns the customer's verify badge blue.
func (ctrl *CustomerController) ApproveDocument(c *fiber.Ctx) error {
	doc, errResp := ctrl.reviewTarget(c)
	if errResp != nil {
		return errResp
	}
	updated, err := ctrl.customerUsecase.ApproveDocument(doc.ID, middleware.GetUserID(c))
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.Success(c, "Document approved", updated)
}

// RejectDocument turns a document down with an optional reason, which is passed on
// to the customer so they know what to re-upload.
func (ctrl *CustomerController) RejectDocument(c *fiber.Ctx) error {
	doc, errResp := ctrl.reviewTarget(c)
	if errResp != nil {
		return errResp
	}
	var body struct {
		Reason string `json:"reason"`
	}
	_ = c.BodyParser(&body)
	updated, err := ctrl.customerUsecase.RejectDocument(doc.ID, middleware.GetUserID(c), body.Reason)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.Success(c, "Document rejected", updated)
}

// reviewTarget resolves :id/:docId and checks the document really belongs to that
// customer, so a review cannot be aimed at someone else's file by guessing an id.
func (ctrl *CustomerController) reviewTarget(c *fiber.Ctx) (*entity.CustomerDocument, error) {
	cust, errResp := ctrl.loadCustomerParam(c)
	if errResp != nil {
		return nil, errResp
	}
	id := cust.ID
	docID, err := strconv.ParseUint(c.Params("docId"), 10, 32)
	if err != nil {
		return nil, response.BadRequest(c, "Invalid document ID")
	}
	doc, err := ctrl.customerUsecase.GetDocumentByID(uint(docID))
	if err != nil || doc.UserID != id {
		return nil, response.NotFound(c, "Document not found")
	}
	return doc, nil
}
