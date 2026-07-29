package usecase

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"jk-api/internal/entity"
	"jk-api/internal/module/bill/repository"
	notificationRepo "jk-api/internal/module/notification/repository"
)

type BillUsecase interface {
	CreateBill(req *CreateBillRequest) (*entity.Quotation, error)
	GetAllBills(f repository.BillFilter, page, limit int) ([]entity.Quotation, int64, error)
	// SummarizeBills totals the whole filtered set for the list's overview cards
	// (which must not change as the user pages through).
	SummarizeBills(f repository.BillFilter, groupIssued bool) (repository.BillSummary, error)
	GetBillByID(id uint) (*entity.Quotation, error)
	IssueBill(id uint, req *UpdateBillStatusRequest) (*entity.Quotation, error)
	ApproveBill(id uint, req *UpdateBillStatusRequest) (*entity.Quotation, error)
	CancelBill(id uint, req *UpdateBillStatusRequest) (*entity.Quotation, error)
	RevertBill(id uint) (*entity.Quotation, error)
	UpdateBill(id uint, req *UpdateBillRequest) (*entity.Quotation, error)
	// RemoveBillItem deletes one item from a pending bill; if it was the last item
	// the whole bill is deleted. The bool reports whether the bill was deleted.
	RemoveBillItem(billID, itemID uint) (*entity.Quotation, bool, error)
	DeleteBill(id uint) error
	AddImages(id uint, urls []string) error
	CountUnfinished(storeID *uint, branchID *uint, createdBy *uint) (repository.UnfinishedCounts, error)
	PartialDeliverBill(id uint, req *PartialDeliverRequest) (*entity.Quotation, error)
	GetBillBalance(userID uint) (repository.BalanceSummary, []entity.BillBalance, error)
	GetDeliveryLogs(billID uint) ([]entity.BillDeliveryLog, error)
	ClearBills(storeID *uint, billIDs []uint) (int64, error)
}

type CreateBillRequest struct {
	// Store/Branch are set from JWT context in the controller — NOT from payload.
	StoreID         *uint  `json:"-"`
	BranchID        *uint  `json:"-"`
	CreatedByUserID uint   `json:"-"`
	// CustomerID is only honoured when a staff member (master/owner/employee)
	// sells on behalf of a customer; the controller validates it and uses it as
	// the bill's CreatedBy. Ignored for the customer self-service flow.
	CustomerID      uint   `json:"customer_id"`
	// GoldRound/GoldPriceID record the gold-price round at creation (set in the
	// controller from the latest gold price) for reporting.
	GoldRound       string `json:"-"`
	GoldPriceID     *uint  `json:"-"`
	Note            string `json:"note"`
	Items           []CreateBillItemRequest `json:"items"`
}

type CreateBillItemRequest struct {
	TypeID   string  `json:"type_id"`
	TypeName string  `json:"type_name"`
	// Metal tags the item (gold|silver|platinum|palladium); empty = gold. It also
	// decides which bill the item lands in — bills are single-metal, so items of
	// different metals in one payload are split into a bill each.
	Metal    string  `json:"metal"`
	Plus     float64 `json:"plus"`
	Price    float64 `json:"price"`
	Percent  float64 `json:"percent"`
	Weight   float64 `json:"weight"`
	PerGram  float64 `json:"per_gram"`
	Total    float64 `json:"total"`
}

// billItemMetal normalises an item's metal, treating empty as gold (legacy payloads).
func billItemMetal(m string) string {
	if m == "" {
		return "gold"
	}
	return m
}

type UpdateBillStatusRequest struct {
	Note         string `json:"note"`
	RejectReason string `json:"reject_reason"`
}

type UpdateBillRequest struct {
	Note  string                  `json:"note"`
	Items []CreateBillItemRequest `json:"items"`
}

type PartialDeliverRequest struct {
	Weight float64         `json:"weight"`
	Amount float64         `json:"amount"`
	Items  json.RawMessage `json:"items"`
	// LogOnly records the round's items for display without adding to processed
	// weight/amount — used for the final batch, whose total the issued quotation
	// already accounts for (avoids double-counting).
	LogOnly bool `json:"log_only"`
}

type billUsecase struct {
	billRepo        repository.BillRepository
	billBalanceRepo repository.BillBalanceRepository
	notifRepo       notificationRepo.NotificationRepository
}

func NewBillUsecase(billRepo repository.BillRepository, billBalanceRepo repository.BillBalanceRepository, notifRepo notificationRepo.NotificationRepository) BillUsecase {
	return &billUsecase{billRepo: billRepo, billBalanceRepo: billBalanceRepo, notifRepo: notifRepo}
}

func (u *billUsecase) CreateBill(req *CreateBillRequest) (*entity.Quotation, error) {
	if len(req.Items) == 0 {
		return nil, errors.New("ต้องมีรายการอย่างน้อย 1 รายการ")
	}

	// Bills are single-metal, so a payload carrying more than one metal is split
	// into one bill per metal (the sell screens post a single item at a time, so
	// in practice there is exactly one group).
	order, byMetal := groupItemsByMetal(req.Items)

	var first *entity.Quotation
	for _, metal := range order {
		bill, err := u.upsertPendingBill(req, metal, byMetal[metal])
		if err != nil {
			return nil, err
		}
		if first == nil {
			first = bill
		}
	}
	return first, nil
}

// groupItemsByMetal buckets the payload's items by their (normalised) metal and
// returns the metals in the order they first appear, so the response is the bill
// for the first item the customer sent.
func groupItemsByMetal(reqItems []CreateBillItemRequest) ([]string, map[string][]entity.QuotationItem) {
	order := []string{}
	byMetal := map[string][]entity.QuotationItem{}
	for _, item := range reqItems {
		metal := billItemMetal(item.Metal)
		if _, seen := byMetal[metal]; !seen {
			order = append(order, metal)
		}
		byMetal[metal] = append(byMetal[metal], entity.QuotationItem{
			TypeID:   item.TypeID,
			TypeName: item.TypeName,
			Metal:    metal,
			Plus:     item.Plus,
			Price:    item.Price,
			Percent:  item.Percent,
			Weight:   item.Weight,
			PerGram:  item.PerGram,
			Total:    item.Total,
		})
	}
	return order, byMetal
}

// upsertPendingBill appends the items to the customer's open "รอออกบิล" bill of
// the SAME metal, or starts a new bill when they have none. Selling silver while
// a gold bill is still pending therefore opens a separate silver bill.
func (u *billUsecase) upsertPendingBill(req *CreateBillRequest, metal string, items []entity.QuotationItem) (*entity.Quotation, error) {
	var totalAmount float64
	for _, item := range items {
		totalAmount += item.Total
	}

	if existing, err := u.billRepo.FindPendingByCreator(req.CreatedByUserID, metal); err == nil && existing != nil {
		if err := u.billRepo.AppendItems(existing.ID, items); err != nil {
			return nil, err
		}
		existing.TotalAmount += totalAmount
		if err := u.billRepo.Update(existing); err != nil {
			return nil, err
		}
		// The bill keeps its original code and created_at, so without this the
		// customer gets no sign that the sell landed (staff selling on their
		// behalf especially).
		if existing.CreatedBy != nil {
			_ = u.notifRepo.Create(&entity.Notification{
				UserID: *existing.CreatedBy,
				Type:   "bill_updated",
				Title:  "เพิ่มรายการขายแล้ว",
				Body: fmt.Sprintf("เพิ่ม %d รายการ (%s) เข้าบิล %s แล้ว ยอดรวม %s บาท",
					len(items), metalLabel(metal), existing.Code, formatAmount(existing.TotalAmount)),
			})
		}
		return u.billRepo.FindByID(existing.ID)
	}

	code, err := u.billRepo.GenerateCode()
	if err != nil {
		return nil, err
	}

	createdBy := req.CreatedByUserID
	bill := &entity.Quotation{
		StoreID:     req.StoreID,
		BranchID:    req.BranchID,
		CreatedBy:   &createdBy,
		Code:        code,
		Status:      repository.StatusPendingIssue, // รอออกบิล
		Metal:       metal,
		Note:        req.Note,
		TotalAmount: totalAmount,
		GoldRound:   req.GoldRound,
		GoldPriceID: req.GoldPriceID,
		IsBill:      true,
		Items:       items,
	}

	if err := u.billRepo.Create(bill); err != nil {
		return nil, err
	}

	if bill.CreatedBy != nil {
		_ = u.notifRepo.Create(&entity.Notification{
			UserID: *bill.CreatedBy,
			Type:   "bill_created",
			Title:  "สร้างบิลสำเร็จ",
			Body:   fmt.Sprintf("บิล%s %s ถูกสร้างแล้ว สถานะ: รอออกบิล", metalLabel(metal), bill.Code),
		})
	}

	return u.billRepo.FindByID(bill.ID)
}

// uniformMetal tags a bill with its items' metal, but only when they all agree —
// a mixed set reads as gold, matching migration 85's backfill.
func uniformMetal(items []entity.QuotationItem) string {
	metal := "gold"
	for i, item := range items {
		if i == 0 {
			metal = billItemMetal(item.Metal)
			continue
		}
		if billItemMetal(item.Metal) != metal {
			return "gold"
		}
	}
	return metal
}

// metalLabel is the Thai word used in customer-facing notifications.
func metalLabel(metal string) string {
	switch metal {
	case "gold":
		return "ทอง"
	case "silver":
		return "เงิน"
	case "platinum":
		return "แพลทินัม"
	case "palladium":
		return "แพลเลเดียม"
	}
	return metal
}

func formatAmount(v float64) string {
	return fmt.Sprintf("%.2f", v)
}

func (u *billUsecase) GetAllBills(f repository.BillFilter, page, limit int) ([]entity.Quotation, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}
	return u.billRepo.FindAll(f, page, limit)
}

func (u *billUsecase) SummarizeBills(f repository.BillFilter, groupIssued bool) (repository.BillSummary, error) {
	return u.billRepo.Summarize(f, groupIssued)
}

func (u *billUsecase) GetBillByID(id uint) (*entity.Quotation, error) {
	return u.billRepo.FindByID(id)
}

// IssueBill moves a bill from รอออกบิล (10) to รอตรวจบิล (11). Master only (route-gated).
func (u *billUsecase) IssueBill(id uint, req *UpdateBillStatusRequest) (*entity.Quotation, error) {
	bill, err := u.billRepo.FindByID(id)
	if err != nil {
		return nil, errors.New("bill not found")
	}
	if bill.Status != repository.StatusPendingIssue {
		return nil, errors.New("ออกบิลได้เฉพาะบิลที่สถานะ 'รอออกบิล' เท่านั้น")
	}
	bill.Status = repository.StatusPendingReview
	if req.Note != "" {
		bill.Note = req.Note
	}
	if err := u.billRepo.Update(bill); err != nil {
		return nil, err
	}
	u.notify(bill, "bill_issued", "บิลถูกออกแล้ว", fmt.Sprintf("บิล %s ออกบิลแล้ว สถานะ: รอตรวจบิล", bill.Code))
	return bill, nil
}

// ApproveBill moves a bill from รอตรวจบิล (11) to สำเร็จ (12).
func (u *billUsecase) ApproveBill(id uint, req *UpdateBillStatusRequest) (*entity.Quotation, error) {
	bill, err := u.billRepo.FindByID(id)
	if err != nil {
		return nil, errors.New("bill not found")
	}
	if bill.Status != repository.StatusPendingReview {
		return nil, errors.New("อนุมัติได้เฉพาะบิลที่สถานะ 'รอตรวจบิล' เท่านั้น")
	}
	bill.Status = repository.StatusCompleted
	if req.Note != "" {
		bill.Note = req.Note
	}
	if err := u.billRepo.Update(bill); err != nil {
		return nil, err
	}
	u.notify(bill, "bill_completed", "บิลสำเร็จ", fmt.Sprintf("บิล %s เสร็จสมบูรณ์แล้ว", bill.Code))
	return bill, nil
}

// CancelBill cancels a bill (→ 13) with a reason. Allowed while not already completed.
func (u *billUsecase) CancelBill(id uint, req *UpdateBillStatusRequest) (*entity.Quotation, error) {
	bill, err := u.billRepo.FindByID(id)
	if err != nil {
		return nil, errors.New("bill not found")
	}
	if bill.Status == repository.StatusCompleted {
		return nil, errors.New("ไม่สามารถยกเลิกบิลที่เสร็จสมบูรณ์แล้ว")
	}
	if bill.Status == repository.StatusCancelled {
		return nil, errors.New("บิลถูกยกเลิกไปแล้ว")
	}
	bill.Status = repository.StatusCancelled
	if req.RejectReason != "" {
		bill.RejectReason = req.RejectReason
	}
	if err := u.billRepo.Update(bill); err != nil {
		return nil, err
	}
	body := fmt.Sprintf("บิล %s ถูกยกเลิก", bill.Code)
	if req.RejectReason != "" {
		body += " เหตุผล: " + req.RejectReason
	}
	u.notify(bill, "bill_cancelled", "บิลถูกยกเลิก", body)
	return bill, nil
}

// RevertBill pulls an issued bill back from รอตรวจบิล (11) to รอออกบิล (10) so the
// master can fix the quote they issued. The issuance side effects (balance ledger,
// delivery logs, issued quotation) are undone so a re-issue doesn't double-count.
func (u *billUsecase) RevertBill(id uint) (*entity.Quotation, error) {
	bill, err := u.billRepo.FindByID(id)
	if err != nil {
		return nil, errors.New("bill not found")
	}
	if bill.Status != repository.StatusPendingReview {
		return nil, errors.New("ย้อนกลับไปแก้ไขได้เฉพาะบิลที่สถานะ 'รอตรวจบิล' เท่านั้น")
	}
	if err := u.billRepo.RevertIssuance(id); err != nil {
		return nil, err
	}
	reverted, err := u.billRepo.FindByID(id)
	if err != nil {
		return nil, err
	}
	u.notify(reverted, "bill_reverted", "บิลถูกดึงกลับไปแก้ไข", fmt.Sprintf("บิล %s ถูกดึงกลับไปแก้ไข สถานะ: รอออกบิล", reverted.Code))
	return reverted, nil
}

// UpdateBill edits a bill's content while it is still รอออกบิล (10).
func (u *billUsecase) UpdateBill(id uint, req *UpdateBillRequest) (*entity.Quotation, error) {
	bill, err := u.billRepo.FindByID(id)
	if err != nil {
		return nil, errors.New("bill not found")
	}
	if bill.Status != repository.StatusPendingIssue {
		return nil, errors.New("แก้ไขบิลได้เฉพาะบิลที่สถานะ 'รอออกบิล' เท่านั้น")
	}

	bill.Note = req.Note
	if len(req.Items) > 0 {
		var totalAmount float64
		var items []entity.QuotationItem
		for _, item := range req.Items {
			totalAmount += item.Total
			items = append(items, entity.QuotationItem{
				QuotationID: bill.ID,
				TypeID:      item.TypeID,
				TypeName:    item.TypeName,
				Metal:       billItemMetal(item.Metal),
				Plus:        item.Plus,
				Price:       item.Price,
				Percent:     item.Percent,
				Weight:      item.Weight,
				PerGram:     item.PerGram,
				Total:       item.Total,
			})
		}
		bill.TotalAmount = totalAmount
		// Items were replaced wholesale, so re-derive which list the bill belongs
		// to (a mixed edit keeps it on the gold list — see migration 85).
		bill.Metal = uniformMetal(items)
		if err := u.billRepo.ReplaceItems(bill.ID, items); err != nil {
			return nil, err
		}
	}

	if err := u.billRepo.Update(bill); err != nil {
		return nil, err
	}
	return u.billRepo.FindByID(id)
}

// RemoveBillItem lets the master drop an item the customer submitted while the bill
// is still รอออกบิล (10). The bill's total_amount is recomputed; if no items remain
// the whole bill is deleted (returns deleted=true).
func (u *billUsecase) RemoveBillItem(billID, itemID uint) (*entity.Quotation, bool, error) {
	bill, err := u.billRepo.FindByID(billID)
	if err != nil {
		return nil, false, errors.New("bill not found")
	}
	// Editable while pending (10) or already issued (11). At 11 the repo also keeps
	// the debt/credit ledger in sync so removing an item is safe even if the master
	// doesn't re-issue afterwards.
	if bill.Status != repository.StatusPendingIssue && bill.Status != repository.StatusPendingReview {
		return nil, false, errors.New("แก้ไขรายการได้เฉพาะบิลที่ยังไม่ปิด")
	}
	remaining, err := u.billRepo.RemoveItem(billID, itemID)
	if err != nil {
		return nil, false, err
	}
	if remaining == 0 {
		if err := u.billRepo.Delete(billID); err != nil {
			return nil, false, err
		}
		return nil, true, nil
	}
	updated, err := u.billRepo.FindByID(billID)
	if err != nil {
		return nil, false, err
	}
	return updated, false, nil
}

func (u *billUsecase) DeleteBill(id uint) error {
	if _, err := u.billRepo.FindByID(id); err != nil {
		return errors.New("bill not found")
	}
	return u.billRepo.Delete(id)
}

func (u *billUsecase) AddImages(id uint, urls []string) error {
	return u.billRepo.AddImages(id, urls)
}

func (u *billUsecase) CountUnfinished(storeID *uint, branchID *uint, createdBy *uint) (repository.UnfinishedCounts, error) {
	return u.billRepo.CountUnfinished(storeID, branchID, createdBy)
}

func (u *billUsecase) PartialDeliverBill(id uint, req *PartialDeliverRequest) (*entity.Quotation, error) {
	if req.Weight < 0 || req.Amount < 0 {
		return nil, errors.New("weight and amount must not be negative")
	}
	// Weight/amount carry the GOLD portion only (the bill's processed aggregates
	// are gold). A round of non-gold metals arrives with both at zero — that's
	// valid as long as it still has items to log; reject only truly empty calls.
	itemsJSON := strings.TrimSpace(string(req.Items))
	hasItems := itemsJSON != "" && itemsJSON != "[]" && itemsJSON != "null"
	if req.Weight == 0 && req.Amount == 0 && !hasItems {
		return nil, errors.New("weight and amount must be greater than zero")
	}
	bill, err := u.billRepo.FindByID(id)
	if err != nil {
		return nil, errors.New("bill not found")
	}
	// Final batch: only record its items for display; the issued quotation already
	// covers this amount, so don't add it to processed weight/amount. Runs BEFORE
	// the status guard because by save time the bill has already moved past
	// "รอออกบิล" (the issued quotation was just created).
	if req.LogOnly {
		_ = u.billRepo.LogDelivery(id, req.Weight, req.Amount, "รอบสุดท้าย", req.Items)
		return bill, nil
	}
	if bill.Status != repository.StatusPendingIssue {
		return nil, errors.New("บันทึกส่งบางส่วนได้เฉพาะบิลที่สถานะ 'รอออกบิล' เท่านั้น")
	}
	// Non-gold-only round: nothing to add to the (gold) processed aggregates —
	// just log the items so they survive a reload and reach the final quotation.
	if req.Weight == 0 && req.Amount == 0 {
		_ = u.billRepo.LogDelivery(id, 0, 0, "รอส่งเพิ่ม", req.Items)
		return bill, nil
	}
	result, err := u.billRepo.PartialDeliver(id, req.Weight, req.Amount)
	if err != nil {
		return nil, err
	}
	_ = u.billRepo.LogDelivery(id, req.Weight, req.Amount, "รอส่งเพิ่ม", req.Items)
	return result, nil
}

func (u *billUsecase) GetDeliveryLogs(billID uint) ([]entity.BillDeliveryLog, error) {
	return u.billRepo.GetDeliveryLogs(billID)
}

func (u *billUsecase) GetBillBalance(userID uint) (repository.BalanceSummary, []entity.BillBalance, error) {
	summary, err := u.billBalanceRepo.GetBalance(userID)
	if err != nil {
		return repository.BalanceSummary{}, nil, err
	}
	history, err := u.billBalanceRepo.GetHistory(userID, 50)
	if err != nil {
		return repository.BalanceSummary{}, nil, err
	}
	return summary, history, nil
}

func (u *billUsecase) ClearBills(storeID *uint, billIDs []uint) (int64, error) {
	return u.billRepo.ClearBills(storeID, billIDs)
}

func (u *billUsecase) notify(bill *entity.Quotation, typ, title, body string) {
	if bill.CreatedBy == nil {
		return
	}
	_ = u.notifRepo.Create(&entity.Notification{
		UserID: *bill.CreatedBy,
		Type:   typ,
		Title:  title,
		Body:   body,
	})
}
