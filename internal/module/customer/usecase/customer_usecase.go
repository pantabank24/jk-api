package usecase

import (
	"errors"
	"fmt"
	"os"
	"time"

	"jk-api/internal/entity"
	"jk-api/internal/module/customer/repository"
	notificationRepo "jk-api/internal/module/notification/repository"
	jwtPkg "jk-api/pkg/jwt"
)

type CustomerUsecase interface {
	CreateCustomer(req *CreateCustomerRequest) (*entity.User, error)
	GetAllCustomers(page, limit int, storeID *uint, search string) ([]entity.User, int64, error)
	GetCustomerByID(id uint) (*entity.User, error)
	UpdateCustomer(id uint, req *UpdateCustomerRequest) (*entity.User, error)
	UpdateAvatar(id uint, avatar string) (*entity.User, error)
	DeleteCustomer(id uint) error

	AddDocument(doc *entity.CustomerDocument) error
	GetDocuments(userID uint) ([]entity.CustomerDocument, error)
	GetDocumentByID(id uint) (*entity.CustomerDocument, error)
	DeleteDocument(id uint) error
	GetActiveDocumentType(id uint) (*entity.DocumentType, error)
	// ReplaceDocumentsOfType clears out what a customer already has of one type,
	// files included, so a high-priority type always holds exactly one current copy.
	ReplaceDocumentsOfType(userID, typeID uint) error
	// NotifyDocumentReview tells the shop's staff that a customer just put up a
	// high-priority document that needs checking.
	NotifyDocumentReview(customer *entity.User, typeName string)
	ApproveDocument(docID, reviewerID uint) (*entity.CustomerDocument, error)
	RejectDocument(docID, reviewerID uint, reason string) (*entity.CustomerDocument, error)
}

type CreateCustomerRequest struct {
	// ร้านที่ลูกค้าสังกัด — ตั้งโดย controller จาก role ของผู้เรียก (staff ใช้ร้านตัวเอง,
	// master เลือกเอง) ไม่ได้รับตรงจาก body ของ client
	StoreID         *uint  `json:"-"`
	Name            string `json:"name" validate:"required"`
	Email           string `json:"email" validate:"required,email"`
	Password        string `json:"password" validate:"required,min=6"`
	Phone           string `json:"phone"`
	StoreName       string `json:"store_name"`
	Address         string `json:"address"`
	TaxID           string `json:"tax_id"`
	BankID          *uint  `json:"bank_id"`
	BankAccountNo   string `json:"bank_account_no"`
	BankAccountName string `json:"bank_account_name"`
}

type UpdateCustomerRequest struct {
	// ย้ายลูกค้าข้ามร้าน — controller ยอมให้เฉพาะ master เท่านั้น
	StoreID   *uint   `json:"-"`
	Name      string  `json:"name"`
	Email     string  `json:"email" validate:"omitempty,email"`
	Password  string  `json:"password"`
	Phone     string  `json:"phone"`
	StoreName *string `json:"store_name"`
	Address   *string `json:"address"`
	TaxID     *string `json:"tax_id"`
	// Pointers so clearing a bank (bank_id: null) is distinguishable from "not sent".
	BankID          *uint   `json:"bank_id"`
	BankAccountNo   *string `json:"bank_account_no"`
	BankAccountName *string `json:"bank_account_name"`
	IsActive        *bool   `json:"is_active"`
}

// normalizeBankID maps "no bank" to a NULL bank_id. The form always sends a value,
// using 0 for "ไม่ระบุ" (an empty <Select> yields no id), so 0 must not be written
// as a real foreign key.
func normalizeBankID(id *uint) *uint {
	if id == nil || *id == 0 {
		return nil
	}
	return id
}

type customerUsecase struct {
	customerRepo repository.CustomerRepository
	notifRepo    notificationRepo.NotificationRepository
}

func NewCustomerUsecase(customerRepo repository.CustomerRepository, notifRepo notificationRepo.NotificationRepository) CustomerUsecase {
	return &customerUsecase{customerRepo: customerRepo, notifRepo: notifRepo}
}

func (u *customerUsecase) CreateCustomer(req *CreateCustomerRequest) (*entity.User, error) {
	if u.customerRepo.ExistsByEmail(req.Email) {
		return nil, errors.New("email already exists")
	}

	roleID, err := u.customerRepo.GetCustomerRoleID()
	if err != nil {
		return nil, err
	}

	hashed, err := jwtPkg.HashPassword(req.Password)
	if err != nil {
		return nil, errors.New("failed to hash password")
	}

	// ลูกค้าต้องสังกัดร้าน ไม่งั้นจะไม่โผล่ในลิสต์ของ owner/employee ร้านไหนเลย.
	// ร้านเดียวก็ไม่ต้องให้ระบุ — เดาให้ได้ตัวเดียวอยู่แล้ว
	storeID := req.StoreID
	if storeID == nil {
		sole, err := u.customerRepo.FindSoleStoreID()
		if err != nil {
			return nil, err
		}
		if sole == nil {
			return nil, errors.New("กรุณาระบุร้านของลูกค้า")
		}
		storeID = sole
	}

	user := &entity.User{
		StoreID:         storeID,
		Name:            req.Name,
		Email:           req.Email,
		Password:        hashed,
		Phone:           req.Phone,
		StoreName:       req.StoreName,
		Address:         req.Address,
		TaxID:           req.TaxID,
		BankID:          normalizeBankID(req.BankID),
		BankAccountNo:   req.BankAccountNo,
		BankAccountName: req.BankAccountName,
		RoleID:          &roleID,
		IsActive:        true,
	}
	if err := u.customerRepo.Create(user); err != nil {
		return nil, err
	}
	return u.customerRepo.FindByID(user.ID)
}

func (u *customerUsecase) GetAllCustomers(page, limit int, storeID *uint, search string) ([]entity.User, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}
	return u.customerRepo.FindAll(page, limit, storeID, search)
}

func (u *customerUsecase) GetCustomerByID(id uint) (*entity.User, error) {
	return u.customerRepo.FindByID(id)
}

func (u *customerUsecase) UpdateCustomer(id uint, req *UpdateCustomerRequest) (*entity.User, error) {
	user, err := u.customerRepo.FindByID(id)
	if err != nil {
		return nil, errors.New("customer not found")
	}

	if req.StoreID != nil {
		user.StoreID = req.StoreID
	}
	if req.Name != "" {
		user.Name = req.Name
	}
	if req.Email != "" && req.Email != user.Email {
		if u.customerRepo.ExistsByEmail(req.Email) {
			return nil, errors.New("email already exists")
		}
		user.Email = req.Email
	}
	if req.Password != "" {
		hashed, err := jwtPkg.HashPassword(req.Password)
		if err != nil {
			return nil, errors.New("failed to hash password")
		}
		user.Password = hashed
	}
	if req.Phone != "" {
		user.Phone = req.Phone
	}
	if req.StoreName != nil {
		user.StoreName = *req.StoreName
	}
	if req.Address != nil {
		user.Address = *req.Address
	}
	if req.TaxID != nil {
		user.TaxID = *req.TaxID
	}
	if req.BankID != nil {
		user.BankID = normalizeBankID(req.BankID) // 0 = ไม่ระบุ → clear it
	}
	if req.BankAccountNo != nil {
		user.BankAccountNo = *req.BankAccountNo
	}
	if req.BankAccountName != nil {
		user.BankAccountName = *req.BankAccountName
	}
	if req.IsActive != nil {
		user.IsActive = *req.IsActive
	}

	if err := u.customerRepo.Update(user); err != nil {
		return nil, err
	}
	return u.customerRepo.FindByID(id)
}

func (u *customerUsecase) UpdateAvatar(id uint, avatar string) (*entity.User, error) {
	user, err := u.customerRepo.FindByID(id)
	if err != nil {
		return nil, errors.New("customer not found")
	}
	user.Avatar = avatar
	if err := u.customerRepo.Update(user); err != nil {
		return nil, err
	}
	return u.customerRepo.FindByID(id)
}

func (u *customerUsecase) DeleteCustomer(id uint) error {
	if _, err := u.customerRepo.FindByID(id); err != nil {
		return errors.New("customer not found")
	}
	return u.customerRepo.Delete(id)
}

func (u *customerUsecase) AddDocument(doc *entity.CustomerDocument) error {
	return u.customerRepo.CreateDocument(doc)
}

func (u *customerUsecase) GetDocuments(userID uint) ([]entity.CustomerDocument, error) {
	return u.customerRepo.FindDocuments(userID)
}

func (u *customerUsecase) GetDocumentByID(id uint) (*entity.CustomerDocument, error) {
	return u.customerRepo.FindDocumentByID(id)
}

func (u *customerUsecase) DeleteDocument(id uint) error {
	return u.customerRepo.DeleteDocument(id)
}

func (u *customerUsecase) GetActiveDocumentType(id uint) (*entity.DocumentType, error) {
	return u.customerRepo.FindActiveDocumentType(id)
}

func (u *customerUsecase) ReplaceDocumentsOfType(userID, typeID uint) error {
	existing, err := u.customerRepo.FindDocumentsOfType(userID, typeID)
	if err != nil {
		return err
	}
	for _, doc := range existing {
		if err := u.customerRepo.DeleteDocument(doc.ID); err != nil {
			return err
		}
		// Best-effort file removal; the DB row is the source of truth.
		_ = os.Remove("." + doc.FilePath)
	}
	return nil
}

func (u *customerUsecase) NotifyDocumentReview(customer *entity.User, typeName string) {
	if customer == nil {
		return
	}
	// ตั้งใจส่งถึง master คนเดียวก่อน (ตัดสินใจ 2026-09-03) — ส่ง nil เพื่อไม่ให้
	// owner/employee ของร้านเริ่มได้รับเองเงียบ ๆ หลังลูกค้าถูกผูกเข้าร้านแล้ว.
	// ถ้าจะเปิดให้ร้านตรวจเอกสารเองเมื่อไหร่ เปลี่ยนกลับเป็น customer.StoreID
	reviewers, err := u.customerRepo.FindReviewerIDs(nil)
	if err != nil {
		return
	}
	for _, id := range reviewers {
		_ = u.notifRepo.Create(&entity.Notification{
			UserID: id,
			Type:   "document_review",
			Title:  "มีเอกสารรอตรวจสอบ",
			Body:   fmt.Sprintf("%s ส่ง%sเข้ามา กรุณาตรวจสอบและอนุมัติ", customer.Name, typeName),
		})
	}
}

func (u *customerUsecase) ApproveDocument(docID, reviewerID uint) (*entity.CustomerDocument, error) {
	return u.reviewDocument(docID, reviewerID, entity.ApprovalApproved, "")
}

func (u *customerUsecase) RejectDocument(docID, reviewerID uint, reason string) (*entity.CustomerDocument, error) {
	return u.reviewDocument(docID, reviewerID, entity.ApprovalRejected, reason)
}

func (u *customerUsecase) reviewDocument(docID, reviewerID uint, status, reason string) (*entity.CustomerDocument, error) {
	doc, err := u.customerRepo.FindDocumentByID(docID)
	if err != nil {
		return nil, errors.New("ไม่พบเอกสาร")
	}
	if doc.DocumentType == nil || !doc.DocumentType.IsHighPriority {
		return nil, errors.New("เอกสารนี้ไม่ต้องตรวจสอบ")
	}
	now := time.Now()
	doc.ApprovalStatus = status
	doc.ApprovedBy = &reviewerID
	doc.ApprovedAt = &now
	doc.RejectReason = reason
	if err := u.customerRepo.UpdateDocument(doc); err != nil {
		return nil, err
	}

	// The customer only ever sees the outcome here, so tell them either way —
	// a rejection that goes unannounced looks like nothing happened.
	typeName := doc.DocumentType.Name
	if status == entity.ApprovalApproved {
		_ = u.notifRepo.Create(&entity.Notification{
			UserID: doc.UserID,
			Type:   "document_approved",
			Title:  "เอกสารผ่านการตรวจสอบ",
			Body:   fmt.Sprintf("%sของคุณผ่านการตรวจสอบแล้ว", typeName),
		})
	} else {
		body := fmt.Sprintf("%sของคุณไม่ผ่านการตรวจสอบ กรุณาอัปโหลดใหม่", typeName)
		if reason != "" {
			body = fmt.Sprintf("%sของคุณไม่ผ่านการตรวจสอบ (%s) กรุณาอัปโหลดใหม่", typeName, reason)
		}
		_ = u.notifRepo.Create(&entity.Notification{
			UserID: doc.UserID,
			Type:   "document_rejected",
			Title:  "เอกสารไม่ผ่านการตรวจสอบ",
			Body:   body,
		})
	}
	return doc, nil
}
