package usecase

import (
	"errors"
	"fmt"

	"jk-api/internal/entity"
	memberRepo "jk-api/internal/module/member/repository"
	notificationRepo "jk-api/internal/module/notification/repository"
	"jk-api/internal/module/quotation/repository"
)

type QuotationUsecase interface {
	CreateQuotation(req *CreateQuotationRequest) (*entity.Quotation, error)
	GetAllQuotations(storeID *uint, branchID *uint, createdBy *uint, status *int, page, limit int, search string) ([]entity.Quotation, int64, error)
	GetQuotationByID(id uint) (*entity.Quotation, error)
	UpdateQuotationStatus(id uint, req *UpdateStatusRequest) (*entity.Quotation, error)
	UpdateQuotation(id uint, req *UpdateQuotationRequest) (*entity.Quotation, error)
	DeleteQuotation(id uint) error
	AddImages(id uint, urls []string) error
}

type CreateQuotationRequest struct {
	// Store/Branch are set from JWT context in controller — NOT from payload
	StoreID  *uint  `json:"-"`
	BranchID *uint  `json:"-"`
	// CreatedByUserID and AutoApprove are set from JWT in controller
	CreatedByUserID uint   `json:"-"`
	AutoApprove     bool   `json:"-"`
	MemberID        *uint  `json:"member_id"`
	Note            string `json:"note"`
	Items           []CreateQuotationItemRequest `json:"items"`
}

type CreateQuotationItemRequest struct {
	TypeID   string  `json:"type_id"`
	TypeName string  `json:"type_name"`
	Plus     float64 `json:"plus"`
	Price    float64 `json:"price"`
	Percent  float64 `json:"percent"`
	Weight   float64 `json:"weight"`
	PerGram  float64 `json:"per_gram"`
	Total    float64 `json:"total"`
}

type UpdateStatusRequest struct {
	Status       int    `json:"status"`
	Note         string `json:"note"`
	RejectReason string `json:"reject_reason"`
}

type UpdateQuotationRequest struct {
	MemberID *uint                       `json:"member_id"`
	Note     string                      `json:"note"`
	Items    []CreateQuotationItemRequest `json:"items"`
}

type quotationUsecase struct {
	quotationRepo repository.QuotationRepository
	memberRepo    memberRepo.MemberRepository
	notifRepo     notificationRepo.NotificationRepository
}

func NewQuotationUsecase(quotationRepo repository.QuotationRepository, memberRepo memberRepo.MemberRepository, notifRepo notificationRepo.NotificationRepository) QuotationUsecase {
	return &quotationUsecase{
		quotationRepo: quotationRepo,
		memberRepo:    memberRepo,
		notifRepo:     notifRepo,
	}
}

func (u *quotationUsecase) CreateQuotation(req *CreateQuotationRequest) (*entity.Quotation, error) {
	if len(req.Items) == 0 {
		return nil, errors.New("ต้องมีรายการอย่างน้อย 1 รายการ")
	}

	// Calculate total
	var totalAmount float64
	for _, item := range req.Items {
		totalAmount += item.Total
	}

	// Auto-link member from creator's profile + credit check
	if req.CreatedByUserID != 0 {
		member, err := u.memberRepo.FindByUserID(req.CreatedByUserID)
		if err == nil && member != nil {
			req.MemberID = &member.ID // always link member from token
			if member.Credits < totalAmount {
				return nil, errors.New("เครดิตไม่เพียงพอ กรุณาเติมเครดิตก่อนออกใบเสนอราคา")
			}
		}
	}

	code, err := u.quotationRepo.GenerateCode()
	if err != nil {
		return nil, err
	}

	var items []entity.QuotationItem
	for _, item := range req.Items {
		items = append(items, entity.QuotationItem{
			TypeID:   item.TypeID,
			TypeName: item.TypeName,
			Plus:     item.Plus,
			Price:    item.Price,
			Percent:  item.Percent,
			Weight:   item.Weight,
			PerGram:  item.PerGram,
			Total:    item.Total,
		})
	}

	createdBy := req.CreatedByUserID
	status := 0 // pending
	if req.AutoApprove {
		status = 1 // approved
	}

	quotation := &entity.Quotation{
		StoreID:     req.StoreID,
		BranchID:    req.BranchID,
		MemberID:    req.MemberID,
		CreatedBy:   &createdBy,
		Code:        code,
		Status:      status,
		Note:        req.Note,
		TotalAmount: totalAmount,
		Items:       items,
	}

	if err := u.quotationRepo.Create(quotation); err != nil {
		return nil, err
	}

	// Auto-approved: deduct credits immediately
	if req.AutoApprove && quotation.CreatedBy != nil {
		member, err := u.memberRepo.FindByUserID(*quotation.CreatedBy)
		if err == nil && member != nil {
			newBalance := member.Credits - quotation.TotalAmount
			if newBalance < 0 {
				newBalance = 0
			}
			member.Credits = newBalance
			_ = u.memberRepo.Update(member)
			_ = u.memberRepo.CreateCreditTransaction(&entity.CreditTransaction{
				MemberID:    member.ID,
				StoreID:     member.StoreID,
				BranchID:    member.BranchID,
				Action:      1, // withdraw
				Amount:      quotation.TotalAmount,
				Balance:     newBalance,
				Description: "หักเครดิตจากใบเสนอราคา " + quotation.Code,
				CreatedBy:   quotation.CreatedBy,
			})
			_ = u.notifRepo.Create(&entity.Notification{
				UserID: *quotation.CreatedBy,
				Type:   "quotation_approved",
				Title:  "ใบเสนอราคาได้รับการอนุมัติ",
				Body:   fmt.Sprintf("ใบเสนอราคา %s ได้รับการอนุมัติ หักเครดิต %.2f บาท คงเหลือ %.2f บาท", quotation.Code, quotation.TotalAmount, newBalance),
			})
		} else {
			_ = u.notifRepo.Create(&entity.Notification{
				UserID: *quotation.CreatedBy,
				Type:   "quotation_approved",
				Title:  "ใบเสนอราคาได้รับการอนุมัติ",
				Body:   fmt.Sprintf("ใบเสนอราคา %s ได้รับการอนุมัติแล้ว", quotation.Code),
			})
		}
	}

	return quotation, nil
}

func (u *quotationUsecase) GetAllQuotations(storeID *uint, branchID *uint, createdBy *uint, status *int, page, limit int, search string) ([]entity.Quotation, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}
	return u.quotationRepo.FindAll(storeID, branchID, createdBy, status, page, limit, search)
}

func (u *quotationUsecase) GetQuotationByID(id uint) (*entity.Quotation, error) {
	return u.quotationRepo.FindByID(id)
}

func (u *quotationUsecase) UpdateQuotationStatus(id uint, req *UpdateStatusRequest) (*entity.Quotation, error) {
	quotation, err := u.quotationRepo.FindByID(id)
	if err != nil {
		return nil, errors.New("quotation not found")
	}

	prevStatus := quotation.Status
	quotation.Status = req.Status
	if req.Note != "" {
		quotation.Note = req.Note
	}
	if req.RejectReason != "" {
		quotation.RejectReason = req.RejectReason
	}

	if err := u.quotationRepo.Update(quotation); err != nil {
		return nil, err
	}

	// On approval (pending → approved): deduct credits from creator's member profile
	if prevStatus == 0 && req.Status == 1 && quotation.CreatedBy != nil {
		member, err := u.memberRepo.FindByUserID(*quotation.CreatedBy)
		if err == nil && member != nil {
			newBalance := member.Credits - quotation.TotalAmount
			if newBalance < 0 {
				newBalance = 0
			}
			member.Credits = newBalance
			_ = u.memberRepo.Update(member)

			_ = u.memberRepo.CreateCreditTransaction(&entity.CreditTransaction{
				MemberID:    member.ID,
				StoreID:     member.StoreID,
				BranchID:    member.BranchID,
				Action:      1, // withdraw
				Amount:      quotation.TotalAmount,
				Balance:     newBalance,
				Description: "หักเครดิตจากใบเสนอราคา " + quotation.Code,
				CreatedBy:   quotation.CreatedBy,
			})
			_ = u.notifRepo.Create(&entity.Notification{
				UserID: *quotation.CreatedBy,
				Type:   "quotation_approved",
				Title:  "ใบเสนอราคาได้รับการอนุมัติ",
				Body:   fmt.Sprintf("ใบเสนอราคา %s ได้รับการอนุมัติ หักเครดิต %.2f บาท คงเหลือ %.2f บาท", quotation.Code, quotation.TotalAmount, newBalance),
			})
		} else if quotation.CreatedBy != nil {
			_ = u.notifRepo.Create(&entity.Notification{
				UserID: *quotation.CreatedBy,
				Type:   "quotation_approved",
				Title:  "ใบเสนอราคาได้รับการอนุมัติ",
				Body:   fmt.Sprintf("ใบเสนอราคา %s ได้รับการอนุมัติแล้ว", quotation.Code),
			})
		}
	}

	// On rejection: notify creator
	if req.Status == 2 && quotation.CreatedBy != nil {
		body := fmt.Sprintf("ใบเสนอราคา %s ถูกยกเลิก", quotation.Code)
		if req.RejectReason != "" {
			body += " เหตุผล: " + req.RejectReason
		}
		_ = u.notifRepo.Create(&entity.Notification{
			UserID: *quotation.CreatedBy,
			Type:   "quotation_rejected",
			Title:  "ใบเสนอราคาถูกยกเลิก",
			Body:   body,
		})
	}

	return quotation, nil
}

func (u *quotationUsecase) UpdateQuotation(id uint, req *UpdateQuotationRequest) (*entity.Quotation, error) {
	quotation, err := u.quotationRepo.FindByID(id)
	if err != nil {
		return nil, errors.New("quotation not found")
	}
	if quotation.Status != 0 {
		return nil, errors.New("ไม่สามารถแก้ไขใบเสนอราคาที่อนุมัติหรือยกเลิกแล้ว")
	}

	quotation.MemberID = req.MemberID
	quotation.Note = req.Note

	if len(req.Items) > 0 {
		var totalAmount float64
		var items []entity.QuotationItem
		for _, item := range req.Items {
			totalAmount += item.Total
			items = append(items, entity.QuotationItem{
				QuotationID: quotation.ID,
				TypeID:      item.TypeID,
				TypeName:    item.TypeName,
				Plus:        item.Plus,
				Price:       item.Price,
				Percent:     item.Percent,
				Weight:      item.Weight,
				PerGram:     item.PerGram,
				Total:       item.Total,
			})
		}
		quotation.TotalAmount = totalAmount
		if err := u.quotationRepo.ReplaceItems(quotation.ID, items); err != nil {
			return nil, err
		}
	}

	if err := u.quotationRepo.Update(quotation); err != nil {
		return nil, err
	}
	return u.quotationRepo.FindByID(id)
}

func (u *quotationUsecase) DeleteQuotation(id uint) error {
	_, err := u.quotationRepo.FindByID(id)
	if err != nil {
		return errors.New("quotation not found")
	}
	return u.quotationRepo.Delete(id)
}

func (u *quotationUsecase) AddImages(id uint, urls []string) error {
	return u.quotationRepo.AddImages(id, urls)
}
