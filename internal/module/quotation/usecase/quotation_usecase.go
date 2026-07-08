package usecase

import (
	"errors"
	"fmt"
	"time"

	"jk-api/internal/entity"
	billRepo "jk-api/internal/module/bill/repository"
	memberRepo "jk-api/internal/module/member/repository"
	notificationRepo "jk-api/internal/module/notification/repository"
	"jk-api/internal/module/quotation/repository"
)

type QuotationUsecase interface {
	CreateQuotation(req *CreateQuotationRequest) (*entity.Quotation, error)
	GetAllQuotations(storeID *uint, branchID *uint, createdBy *uint, status *int, page, limit int, search string) ([]entity.Quotation, int64, error)
	GetQuotationByID(id uint) (*entity.Quotation, error)
	UpdateQuotationStatus(id uint, req *UpdateStatusRequest, allowApproved bool) (*entity.Quotation, error)
	UpdateQuotation(id uint, req *UpdateQuotationRequest, allowApproved bool) (*entity.Quotation, error)
	DeleteQuotation(id uint) error
	AddImages(id uint, urls []string, imageType string) error
	// PreviewCreditReset/ResetMemberCredit bulk-refund the credit charged on a
	// member's approved-but-not-yet-refunded quotations (see ResetMemberCredit).
	PreviewCreditReset(memberID uint) (*CreditResetPreview, error)
	ResetMemberCredit(memberID uint, actingUserID uint) (*CreditResetResult, error)
}

type CreditResetItem struct {
	ID          uint      `json:"id"`
	Code        string    `json:"code"`
	TotalAmount float64   `json:"total_amount"`
	CreatedAt   time.Time `json:"created_at"`
}

type CreditResetPreview struct {
	Count  int               `json:"count"`
	Amount float64           `json:"amount"`
	Items  []CreditResetItem `json:"items"`
}

type CreditResetResult struct {
	Count   int     `json:"count"`
	Amount  float64 `json:"amount"`
	Balance float64 `json:"balance"`
}

type CreateQuotationRequest struct {
	// Store/Branch are set from JWT context in controller — NOT from payload
	StoreID  *uint  `json:"-"`
	BranchID *uint  `json:"-"`
	// CreatedByUserID and UsesCredits are set from JWT in controller.
	// UsesCredits is true when the creator's role holds credits.use — its
	// quotations deduct credits from the creator's member profile on creation.
	CreatedByUserID uint   `json:"-"`
	UsesCredits     bool   `json:"-"`
	// GoldRound/GoldPriceID record the gold-price round in effect at creation
	// (set from the latest gold price in the controller) for reporting.
	GoldRound       string `json:"-"`
	GoldPriceID     *uint  `json:"-"`
	// Store/Branch header snapshot — resolved from StoreID/BranchID in the
	// controller and copied onto the saved quotation, so reprinting later
	// always shows the header as it was on the day of issue.
	StoreName    string `json:"-"`
	StoreBranch  string `json:"-"`
	StoreAddress string `json:"-"`
	StorePhone   string `json:"-"`
	StoreTaxID   string `json:"-"`
	StoreTaxName string `json:"-"`
	StoreWebsite string `json:"-"`
	StoreLogo    string `json:"-"`
	// PayloadStoreID lets a master assign the quotation to a chosen store (master
	// has no store_id of their own). Ignored for employee (set from JWT).
	PayloadStoreID  *uint  `json:"store_id"`
	// PayloadBranchID picks which branch's receipt header to snapshot. Master and
	// owner choose it (defaulting to the store's main branch); employees are
	// locked to their JWT branch. The whole header is copied from this branch.
	PayloadBranchID *uint  `json:"branch_id"`
	// NoHeader opts out of the receipt-header snapshot — the quotation prints
	// without a header. Store/branch linkage (lists, reporting) is unaffected.
	NoHeader        bool   `json:"no_header"`
	MemberID        *uint  `json:"member_id"`
	Note            string `json:"note"`
	SignerName      string `json:"signer_name"`
	SignerPhone     string `json:"signer_phone"`
	PDPAConsent     bool   `json:"pdpa_consent"`
	// BillID / BillIDs link this quotation to the customer's bill(s) it is issued
	// for — those bills then advance to "รอตรวจบิล". BillIDs lets a master combine
	// all of a customer's pending bills into one quotation.
	BillID          *uint  `json:"bill_id"`
	BillIDs         []uint `json:"bill_ids"`
	// BillItemIDs are the customer bill items the master TICKED for this round.
	// Bills fully covered advance whole; partially covered bills are split so the
	// unticked items stay "รอออกบิล" for a later round. Empty = cover everything
	// (legacy whole-bill behaviour).
	BillItemIDs     []uint `json:"bill_item_ids"`
	Items           []CreateQuotationItemRequest `json:"items"`
}

type CreateQuotationItemRequest struct {
	TypeID   string  `json:"type_id"`
	TypeName string  `json:"type_name"`
	// Metal tags the item (gold|silver|platinum|palladium). Empty defaults to
	// gold — only gold items participate in the bill debt/credit balance.
	Metal    string  `json:"metal"`
	Plus     float64 `json:"plus"`
	Price    float64 `json:"price"`
	Percent  float64 `json:"percent"`
	Weight   float64 `json:"weight"`
	PerGram  float64 `json:"per_gram"`
	Total    float64 `json:"total"`
}

// itemMetal normalises an item's metal, treating empty as gold (legacy payloads).
func itemMetal(m string) string {
	if m == "" {
		return "gold"
	}
	return m
}

type UpdateStatusRequest struct {
	Status       int    `json:"status"`
	Note         string `json:"note"`
	RejectReason string `json:"reject_reason"`
	// RefundCredits, when true, refunds the credits charged on creation back to
	// the creator on cancellation (status=2). Only applies when the quotation was
	// approved/charged and the creator's role uses credits.
	RefundCredits bool `json:"refund_credits"`
}

type UpdateQuotationRequest struct {
	MemberID *uint                       `json:"member_id"`
	Note     string                      `json:"note"`
	Items    []CreateQuotationItemRequest `json:"items"`
	// AdjustCredits, when true, reconciles the creator's credit balance by the
	// change in total (charging more / refunding) and logs a credit transaction.
	// Only applies when a master edits and the creator's role uses credits.
	AdjustCredits bool `json:"adjust_credits"`
}

type quotationUsecase struct {
	quotationRepo   repository.QuotationRepository
	memberRepo      memberRepo.MemberRepository
	notifRepo       notificationRepo.NotificationRepository
	billBalanceRepo billRepo.BillBalanceRepository
}

func NewQuotationUsecase(quotationRepo repository.QuotationRepository, memberRepo memberRepo.MemberRepository, notifRepo notificationRepo.NotificationRepository, billBalanceRepo billRepo.BillBalanceRepository) QuotationUsecase {
	return &quotationUsecase{
		quotationRepo:   quotationRepo,
		memberRepo:      memberRepo,
		notifRepo:       notifRepo,
		billBalanceRepo: billBalanceRepo,
	}
}

func (u *quotationUsecase) CreateQuotation(req *CreateQuotationRequest) (*entity.Quotation, error) {
	if len(req.Items) == 0 {
		return nil, errors.New("ต้องมีรายการอย่างน้อย 1 รายการ")
	}
	if !req.PDPAConsent {
		return nil, errors.New("กรุณายอมรับเงื่อนไขการเก็บข้อมูลส่วนบุคคล (PDPA) ก่อนบันทึก")
	}

	// Calculate total
	var totalAmount float64
	for _, item := range req.Items {
		totalAmount += item.Total
	}

	// Auto-link the creator's member profile (credits are deducted from it).
	// Overdraw is allowed: no insufficient-credit block here — the balance may
	// go negative. The client shows a warning before creating in that case.
	var creditMember *entity.Member
	if req.CreatedByUserID != 0 {
		member, err := u.memberRepo.FindByUserID(req.CreatedByUserID)
		if err == nil && member != nil {
			req.MemberID = &member.ID // always link member from token
			creditMember = member
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
			Metal:    itemMetal(item.Metal),
			Plus:     item.Plus,
			Price:    item.Price,
			Percent:  item.Percent,
			Weight:   item.Weight,
			PerGram:  item.PerGram,
			Total:    item.Total,
		})
	}

	createdBy := req.CreatedByUserID
	quotation := &entity.Quotation{
		StoreID:     req.StoreID,
		BranchID:    req.BranchID,
		MemberID:    req.MemberID,
		CreatedBy:   &createdBy,
		Code:        code,
		Status:      1, // approved immediately on creation
		Note:        req.Note,
		TotalAmount: totalAmount,
		GoldRound:   req.GoldRound,
		GoldPriceID: req.GoldPriceID,
		SignerName:  req.SignerName,
		SignerPhone: req.SignerPhone,
		PDPAConsent: req.PDPAConsent,
		BillID:      req.BillID,
		StoreName:    req.StoreName,
		StoreBranch:  req.StoreBranch,
		StoreAddress: req.StoreAddress,
		StorePhone:   req.StorePhone,
		StoreTaxID:   req.StoreTaxID,
		StoreTaxName: req.StoreTaxName,
		StoreWebsite: req.StoreWebsite,
		StoreLogo:    req.StoreLogo,
		NoHeader:     req.NoHeader,
		Items:       items,
	}

	if err := u.quotationRepo.Create(quotation); err != nil {
		return nil, err
	}

	// Deduct credits immediately for credit-using creators. Overdraw is allowed:
	// the resulting balance may be negative.
	deducted := false
	if req.UsesCredits && creditMember != nil {
		newBalance := creditMember.Credits - totalAmount
		creditMember.Credits = newBalance
		_ = u.memberRepo.Update(creditMember)
		_ = u.memberRepo.CreateCreditTransaction(&entity.CreditTransaction{
			MemberID:    creditMember.ID,
			StoreID:     creditMember.StoreID,
			BranchID:    creditMember.BranchID,
			Action:      1, // withdraw
			Amount:      totalAmount,
			Balance:     newBalance,
			Description: "หักเครดิตจากใบเสนอราคา " + quotation.Code,
			CreatedBy:   &createdBy,
		})
		deducted = true
	}

	// Notify the creator that the quotation was approved.
	if quotation.CreatedBy != nil {
		body := fmt.Sprintf("ใบเสนอราคา %s ได้รับการอนุมัติแล้ว", quotation.Code)
		if deducted {
			body = fmt.Sprintf("ใบเสนอราคา %s อนุมัติแล้ว หักเครดิต %.2f บาท คงเหลือ %.2f บาท", quotation.Code, totalAmount, creditMember.Credits)
		}
		_ = u.notifRepo.Create(&entity.Notification{
			UserID: *quotation.CreatedBy,
			Type:   "quotation_approved",
			Title:  "ใบเสนอราคาได้รับการอนุมัติ",
			Body:   body,
		})
	}

	// If issued for customer bill(s), advance each to "รอตรวจบิล", link it to this
	// quotation, and notify the customer once so they can view the issued bill.
	billIDs := req.BillIDs
	if len(billIDs) == 0 && req.BillID != nil {
		billIDs = []uint{*req.BillID}
	}
	if len(billIDs) > 0 {
		// Per-item issuance: only the ticked bill items (req.BillItemIDs) are
		// covered by this quotation. Bills fully covered advance whole (exactly
		// the legacy behaviour); partially covered bills are SPLIT so the unticked
		// items stay "รอออกบิล" for a later round. Each round settles itself — no
		// debt/credit ledger is recorded any more (bill_balances is legacy data).
		ticked := make(map[uint]bool, len(req.BillItemIDs))
		for _, id := range req.BillItemIDs {
			ticked[id] = true
		}

		var notifiedUser *uint
		issuedBills := 0
		if bills, err := u.quotationRepo.FindBillsByIDs(billIDs); err == nil {
			for i := range bills {
				bill := &bills[i]
				if notifiedUser == nil {
					notifiedUser = bill.CreatedBy
				}
				// Empty BillItemIDs = legacy payload → cover every item.
				var selected []uint
				for _, item := range bill.Items {
					if len(ticked) == 0 || ticked[item.ID] {
						selected = append(selected, item.ID)
					}
				}
				if len(selected) == 0 {
					continue // none of this bill's items are in this round
				}
				if len(selected) == len(bill.Items) {
					_ = u.quotationRepo.MarkBillIssued(bill.ID, quotation.ID)
				} else if newBillID, err := u.quotationRepo.SplitBillItems(bill.ID, selected); err == nil {
					_ = u.quotationRepo.MarkBillIssued(newBillID, quotation.ID)
				}
				issuedBills++
			}
		}
		if notifiedUser != nil && issuedBills > 0 {
			_ = u.notifRepo.Create(&entity.Notification{
				UserID: *notifiedUser,
				Type:   "bill_issued",
				Title:  "บิลของคุณถูกออกแล้ว",
				Body:   fmt.Sprintf("ออกบิลแล้ว %d รายการ สามารถดูรายละเอียดได้", issuedBills),
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

func (u *quotationUsecase) UpdateQuotationStatus(id uint, req *UpdateStatusRequest, allowApproved bool) (*entity.Quotation, error) {
	quotation, err := u.quotationRepo.FindByID(id)
	if err != nil {
		return nil, errors.New("quotation not found")
	}

	prevStatus := quotation.Status
	// Changing an already-approved quotation (e.g. cancelling it) is master-only,
	// mirroring the edit rule — see the controller.
	if prevStatus == 1 && req.Status != 1 && !allowApproved {
		return nil, errors.New("ไม่มีสิทธิ์เปลี่ยนสถานะใบเสนอราคาที่อนุมัติแล้ว")
	}
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

	// On approval (pending → approved): deduct credits from creator's member profile.
	// New quotations are auto-approved on creation, so this only fires for any
	// legacy pending quotations. Overdraw is allowed: the balance may go negative.
	if prevStatus == 0 && req.Status == 1 && quotation.CreatedBy != nil {
		member, err := u.memberRepo.FindByUserID(*quotation.CreatedBy)
		if err == nil && member != nil {
			newBalance := member.Credits - quotation.TotalAmount
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

	// On cancellation (status=2): optionally refund the credits that were charged
	// on creation, then notify. Refund only when the quotation was approved/charged
	// (prevStatus==1) and the creator's role uses credits.
	if req.Status == 2 && quotation.CreatedBy != nil {
		refunded := false
		if prevStatus == 1 && req.RefundCredits && u.memberRepo.UserUsesCredits(*quotation.CreatedBy) {
			if member, err := u.memberRepo.FindByUserID(*quotation.CreatedBy); err == nil && member != nil {
				member.Credits += quotation.TotalAmount
				_ = u.memberRepo.Update(member)
				_ = u.memberRepo.CreateCreditTransaction(&entity.CreditTransaction{
					MemberID:    member.ID,
					StoreID:     member.StoreID,
					BranchID:    member.BranchID,
					Action:      0, // deposit (refund)
					Amount:      quotation.TotalAmount,
					Balance:     member.Credits,
					Description: "คืนเครดิตจากการยกเลิกใบเสนอราคา " + quotation.Code,
					CreatedBy:   quotation.CreatedBy,
				})
				refunded = true
			}
		}

		body := fmt.Sprintf("ใบเสนอราคา %s ถูกยกเลิก", quotation.Code)
		if req.RejectReason != "" {
			body += " เหตุผล: " + req.RejectReason
		}
		if refunded {
			body += fmt.Sprintf(" (คืนเครดิต %.2f บาท)", quotation.TotalAmount)
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

func (u *quotationUsecase) UpdateQuotation(id uint, req *UpdateQuotationRequest, allowApproved bool) (*entity.Quotation, error) {
	quotation, err := u.quotationRepo.FindByID(id)
	if err != nil {
		return nil, errors.New("quotation not found")
	}
	// Pending quotations are editable normally; approved/cancelled ones only when
	// the caller is allowed to (master) — see the controller.
	if quotation.Status != 0 && !allowApproved {
		return nil, errors.New("ไม่สามารถแก้ไขใบเสนอราคาที่อนุมัติหรือยกเลิกแล้ว")
	}

	oldTotal := quotation.TotalAmount
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
				Metal:       itemMetal(item.Metal),
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

	// Clear preloaded slices so db.Save cannot cascade-upsert them.
	quotation.Items = nil
	quotation.Images = nil

	if err := u.quotationRepo.Update(quotation); err != nil {
		return nil, err
	}

	// Reconcile the creator's credits by the change in total, if requested and the
	// creator's role uses credits. Positive delta charges more, negative refunds.
	// Overdraw is allowed (balance may go negative), consistent with creation.
	delta := quotation.TotalAmount - oldTotal
	if req.AdjustCredits && delta != 0 && quotation.CreatedBy != nil && u.memberRepo.UserUsesCredits(*quotation.CreatedBy) {
		if member, err := u.memberRepo.FindByUserID(*quotation.CreatedBy); err == nil && member != nil {
			member.Credits -= delta
			_ = u.memberRepo.Update(member)

			action := 1 // withdraw (charge more)
			if delta < 0 {
				action = 0 // deposit (refund)
			}
			amount := delta
			if amount < 0 {
				amount = -amount
			}
			_ = u.memberRepo.CreateCreditTransaction(&entity.CreditTransaction{
				MemberID:    member.ID,
				StoreID:     member.StoreID,
				BranchID:    member.BranchID,
				Action:      action,
				Amount:      amount,
				Balance:     member.Credits,
				Description: "ปรับเครดิตจากการแก้ไขใบเสนอราคา " + quotation.Code,
				CreatedBy:   quotation.CreatedBy,
			})
		}
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

func (u *quotationUsecase) AddImages(id uint, urls []string, imageType string) error {
	return u.quotationRepo.AddImages(id, urls, imageType)
}

func (u *quotationUsecase) PreviewCreditReset(memberID uint) (*CreditResetPreview, error) {
	member, err := u.memberRepo.FindByID(memberID)
	if err != nil {
		return nil, errors.New("member not found")
	}
	preview := &CreditResetPreview{}
	if member.UserID == nil {
		return preview, nil
	}
	quotations, err := u.quotationRepo.FindUnrefundedApprovedByCreator(*member.UserID)
	if err != nil {
		return nil, err
	}
	preview.Count = len(quotations)
	for _, q := range quotations {
		preview.Amount += q.TotalAmount
		preview.Items = append(preview.Items, CreditResetItem{ID: q.ID, Code: q.Code, TotalAmount: q.TotalAmount, CreatedAt: q.CreatedAt})
	}
	return preview, nil
}

// ResetMemberCredit bulk-refunds the credit charged on this member's approved
// quotations that haven't been refunded yet (status=1, credits_refunded=false),
// then flags them refunded so a repeat call doesn't double-credit. This covers
// quotations that stay "approved" forever — the only other refund paths are
// rejecting an approved quotation or a master's credit-adjusting edit.
func (u *quotationUsecase) ResetMemberCredit(memberID uint, actingUserID uint) (*CreditResetResult, error) {
	member, err := u.memberRepo.FindByID(memberID)
	if err != nil {
		return nil, errors.New("member not found")
	}
	if member.UserID == nil {
		return nil, errors.New("สมาชิกนี้ไม่มีบัญชีผู้ใช้ที่ผูกกับใบเสนอราคา")
	}
	if !u.memberRepo.UserUsesCredits(*member.UserID) {
		return nil, errors.New("สมาชิกนี้ไม่ได้ใช้ระบบเครดิต")
	}

	quotations, err := u.quotationRepo.FindUnrefundedApprovedByCreator(*member.UserID)
	if err != nil {
		return nil, err
	}
	if len(quotations) == 0 {
		return nil, errors.New("ไม่มีรายการที่ต้องคืนเครดิต")
	}

	var ids []uint
	var totalRefund float64
	balance := member.Credits
	for _, q := range quotations {
		balance += q.TotalAmount
		totalRefund += q.TotalAmount
		ids = append(ids, q.ID)
		_ = u.memberRepo.CreateCreditTransaction(&entity.CreditTransaction{
			MemberID:    member.ID,
			StoreID:     member.StoreID,
			BranchID:    member.BranchID,
			Action:      0, // deposit (refund)
			Amount:      q.TotalAmount,
			Balance:     balance,
			Description: "รีเซ็ตเครดิต — คืนยอดใบเสนอราคา " + q.Code,
			CreatedBy:   &actingUserID,
		})
	}
	member.Credits = balance
	if err := u.memberRepo.Update(member); err != nil {
		return nil, err
	}
	if err := u.quotationRepo.MarkCreditsRefunded(ids); err != nil {
		return nil, err
	}

	_ = u.notifRepo.Create(&entity.Notification{
		UserID: *member.UserID,
		Type:   "credit_reset",
		Title:  "รีเซ็ตเครดิต",
		Body:   fmt.Sprintf("คืนเครดิตจากใบเสนอราคา %d ใบ รวม %.2f บาท คงเหลือ %.2f บาท", len(quotations), totalRefund, member.Credits),
	})

	return &CreditResetResult{Count: len(quotations), Amount: totalRefund, Balance: member.Credits}, nil
}
