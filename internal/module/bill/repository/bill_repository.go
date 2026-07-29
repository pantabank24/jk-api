package repository

import (
	"encoding/json"
	"fmt"
	"time"

	"jk-api/internal/entity"

	"gorm.io/gorm"
)

// BillRepository manages customer bills. Bills are stored in the quotations table
// with is_bill = true; every query here is scoped to that flag so bills and staff
// quotations never leak into each other.
type BillRepository interface {
	Create(bill *entity.Quotation) error
	// FindAll lists one page of bills matching the filter.
	FindAll(f BillFilter, page, limit int) ([]entity.Quotation, int64, error)
	// Summarize aggregates EVERY bill matching the filter, not just the current
	// page — the list's overview cards must not change as the user pages through.
	Summarize(f BillFilter, groupIssued bool) (BillSummary, error)
	FindByID(id uint) (*entity.Quotation, error)
	// FindPendingByCreator returns a customer's open "รอออกบิล" bill for the given
	// metal, if any, so new sells accumulate into it instead of creating separate
	// bills. Bills are single-metal, so selling silver never lands in a gold bill.
	FindPendingByCreator(createdBy uint, metal string) (*entity.Quotation, error)
	AppendItems(billID uint, items []entity.QuotationItem) error
	Update(bill *entity.Quotation) error
	ReplaceItems(billID uint, items []entity.QuotationItem) error
	// RemoveItem hard-deletes one item from a bill and recomputes its total_amount.
	// Returns the number of items remaining (0 = the bill has no items left).
	RemoveItem(billID, itemID uint) (int, error)
	Delete(id uint) error
	// RevertIssuance moves an issued bill (and its combined group) back to
	// "รอออกบิล" (10), undoing the issuance side effects (balance ledger, delivery
	// logs, and the issued quotation) so the master can re-issue cleanly.
	RevertIssuance(id uint) error
	GenerateCode() (string, error)
	AddImages(billID uint, urls []string) error
	CountUnfinished(storeID *uint, branchID *uint, createdBy *uint) (UnfinishedCounts, error)
	// PartialDeliver accumulates processed_weight and processed_amount for a bill
	// when the master records a partial delivery without issuing the full quotation.
	PartialDeliver(billID uint, weight, amount float64) (*entity.Quotation, error)
	// LogDelivery appends one delivery-session record for audit/display.
	LogDelivery(billID uint, weight, amount float64, note string, items json.RawMessage) error
	// GetDeliveryLogs returns all delivery-session records for a bill, oldest first.
	GetDeliveryLogs(billID uint) ([]entity.BillDeliveryLog, error)
	// ClearBills moves สำเร็จ (12) bills to เคลียร์บิลแล้ว (14) and settles their
	// debt/credit ledger rows so the customer's balance/average restart fresh.
	// billIDs empty = all completed bills in scope; a non-empty selection is
	// expanded to whole issue-groups (bills sharing issued_quotation_id).
	ClearBills(storeID *uint, billIDs []uint) (int64, error)
}

// BillFilter is the shared "which bills" question behind the list and its
// overview, so a paged list and its totals can never drift apart.
type BillFilter struct {
	StoreID   *uint
	BranchID  *uint
	CreatedBy *uint
	Status    *int
	// ExcludeStatuses drops statuses from the result — the customer's รายการขาย
	// hides สำเร็จ/เคลียร์แล้ว (those live in บิลทั้งหมด). Ignored when Status is set.
	ExcludeStatuses []int
	Metal           *string
	Search          string
}

// BillSummary is the list's overview strip, aggregated over every matching bill.
type BillSummary struct {
	// Count of displayed rows: issue-groups for staff, individual bills for the
	// customer's own list (mirrors how each list renders).
	Count int64 `json:"count"`
	// RawAmount is what the customers submitted (Σ total_amount).
	RawAmount float64 `json:"raw_amount"`
	// Weight/Amount are the submitted items — same basis as RawAmount, so
	// Amount ÷ Weight is the average price of the sales RawAmount reports.
	Weight float64 `json:"weight"`
	Amount float64 `json:"amount"`
	// PendingClearWeight is the สำเร็จ (12) slice of Weight — waiting to be cleared.
	PendingClearWeight float64 `json:"pending_clear_weight"`
}

// UnfinishedCounts is the sidebar badge split: one number per list page plus the
// combined total (kept for callers that only care that anything is outstanding).
type UnfinishedCounts struct {
	Total  int64 `json:"count"`
	Gold   int64 `json:"gold"`
	Silver int64 `json:"silver"`
}

type billRepository struct {
	db *gorm.DB
}

func NewBillRepository(db *gorm.DB) BillRepository {
	return &billRepository{db: db}
}

func (r *billRepository) Create(bill *entity.Quotation) error {
	bill.IsBill = true
	return r.db.Create(bill).Error
}

func (r *billRepository) FindPendingByCreator(createdBy uint, metal string) (*entity.Quotation, error) {
	var bill entity.Quotation
	err := r.db.Where("is_bill = ? AND created_by = ? AND status = ? AND metal = ?",
		true, createdBy, StatusPendingIssue, metal).
		Order("id DESC").First(&bill).Error
	if err != nil {
		return nil, err
	}
	return &bill, nil
}

func (r *billRepository) AppendItems(billID uint, items []entity.QuotationItem) error {
	if len(items) == 0 {
		return nil
	}
	for i := range items {
		items[i].QuotationID = billID
	}
	return r.db.Create(&items).Error
}

// scope applies a BillFilter to a fresh bills query. Every read that has to agree
// with the list (the list itself, its overview) starts here.
func (r *billRepository) scope(f BillFilter) *gorm.DB {
	query := r.db.Model(&entity.Quotation{}).Where("quotations.is_bill = ?", true)
	if f.StoreID != nil {
		query = query.Where("quotations.store_id = ?", *f.StoreID)
	}
	if f.BranchID != nil {
		query = query.Where("quotations.branch_id = ?", *f.BranchID)
	}
	if f.CreatedBy != nil {
		query = query.Where("quotations.created_by = ?", *f.CreatedBy)
	}
	if f.Status != nil {
		query = query.Where("quotations.status = ?", *f.Status)
	} else if len(f.ExcludeStatuses) > 0 {
		query = query.Where("quotations.status NOT IN ?", f.ExcludeStatuses)
	}
	if f.Metal != nil {
		query = query.Where("quotations.metal = ?", *f.Metal)
	}
	if f.Search != "" {
		query = query.Where("quotations.code ILIKE ?", "%"+f.Search+"%")
	}
	return query
}

func (r *billRepository) FindAll(f BillFilter, page, limit int) ([]entity.Quotation, int64, error) {
	var bills []entity.Quotation
	var total int64

	query := r.scope(f)
	query.Count(&total)
	offset := (page - 1) * limit
	// IssuedQuotation (code + total) lets the list show the issued quotation a bill
	// was rolled into; its items aren't needed until the detail view.
	err := query.Preload("Items").Preload("Images").Preload("Member").Preload("Creator").
		Preload("Store").Preload("Branch").Preload("IssuedQuotation").
		Offset(offset).Limit(limit).Order("quotations.id DESC").Find(&bills).Error
	return bills, total, err
}

// Summarize aggregates the whole filtered set for the overview cards.
// groupIssued mirrors the list: staff see bills issued together as ONE row, the
// customer's own list shows each bill separately.
func (r *billRepository) Summarize(f BillFilter, groupIssued bool) (BillSummary, error) {
	var sum BillSummary

	countExpr := "COUNT(*)"
	if groupIssued {
		countExpr = "COUNT(DISTINCT COALESCE(quotations.issued_quotation_id, quotations.id))"
	}
	var head struct {
		Count     int64
		RawAmount float64
	}
	if err := r.scope(f).
		Select(countExpr + " AS count, COALESCE(SUM(quotations.total_amount), 0) AS raw_amount").
		Scan(&head).Error; err != nil {
		return sum, err
	}
	sum.Count, sum.RawAmount = head.Count, head.RawAmount

	// Weight/amount come from what the customers submitted — the same basis as
	// RawAmount above, so ราคาเฉลี่ย (amount ÷ weight) describes the sales the
	// ยอดขายรวม card is showing. The master's re-assessed figures belong to the
	// individual bill, not to this strip.
	weigh := func(q *gorm.DB) (float64, float64, error) {
		items := r.db.Model(&entity.QuotationItem{}).
			Where("quotation_id IN (?)", q.Select("quotations.id"))
		// A single-metal list must not pick up the stray other-metal lines a legacy
		// mixed bill still carries.
		if f.Metal != nil {
			items = items.Where("COALESCE(metal, 'gold') = ?", *f.Metal)
		}
		var agg struct {
			Weight float64
			Amount float64
		}
		err := items.Select("COALESCE(SUM(weight), 0) AS weight, COALESCE(SUM(total), 0) AS amount").
			Scan(&agg).Error
		return agg.Weight, agg.Amount, err
	}

	weight, amount, err := weigh(r.scope(f))
	if err != nil {
		return sum, err
	}
	sum.Weight, sum.Amount = weight, amount

	// The สำเร็จ slice — what is sitting there waiting for เคลียร์บิล.
	pending, _, err := weigh(r.scope(f).Where("quotations.status = ?", StatusCompleted))
	if err != nil {
		return sum, err
	}
	sum.PendingClearWeight = pending

	return sum, nil
}

func (r *billRepository) FindByID(id uint) (*entity.Quotation, error) {
	var bill entity.Quotation
	// IssuedQuotation (with its items/images) is the real bill the customer views —
	// they only have bills.read, not quotations.read.
	// Creator.Bank feeds the payout details printed on the quotation (ชำระโดย เงินโอน).
	err := r.db.Preload("Items").Preload("Images").Preload("Member").Preload("Creator").Preload("Creator.Bank").
		Preload("Store").Preload("Branch").
		Preload("IssuedQuotation.Items").Preload("IssuedQuotation.Images").
		Where("is_bill = ?", true).First(&bill, id).Error
	if err != nil {
		return nil, err
	}
	return &bill, nil
}

func (r *billRepository) Update(bill *entity.Quotation) error {
	return r.db.Save(bill).Error
}

func (r *billRepository) ReplaceItems(billID uint, items []entity.QuotationItem) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Hard-delete old items on edit (Unscoped) — no soft-deleted clutter.
		if err := tx.Unscoped().Where("quotation_id = ?", billID).Delete(&entity.QuotationItem{}).Error; err != nil {
			return err
		}
		if len(items) > 0 {
			return tx.Create(&items).Error
		}
		return nil
	})
}

// RemoveItem hard-deletes one item (scoped to the bill) and recomputes the bill's
// total_amount from the surviving items. Returns the remaining item count so the
// caller can drop an emptied bill entirely.
func (r *billRepository) RemoveItem(billID, itemID uint) (int, error) {
	var remaining int64
	err := r.db.Transaction(func(tx *gorm.DB) error {
		// Read the item (its total/weight) and the bill's issued-quotation link
		// before deleting — needed to keep an issued bill's ledger in sync.
		var item entity.QuotationItem
		if err := tx.Where("id = ? AND quotation_id = ?", itemID, billID).First(&item).Error; err != nil {
			return err
		}
		var bill entity.Quotation
		if err := tx.Select("id", "issued_quotation_id").Where("id = ?", billID).First(&bill).Error; err != nil {
			return err
		}

		if err := tx.Unscoped().Where("id = ? AND quotation_id = ?", itemID, billID).
			Delete(&entity.QuotationItem{}).Error; err != nil {
			return err
		}
		if err := tx.Model(&entity.QuotationItem{}).Where("quotation_id = ?", billID).
			Count(&remaining).Error; err != nil {
			return err
		}
		if remaining == 0 {
			return nil // caller will delete the empty bill (drops its ledger too)
		}
		if err := tx.Model(&entity.Quotation{}).Where("id = ?", billID).
			Update("total_amount", gorm.Expr(
				"(SELECT COALESCE(SUM(total),0) FROM quotation_items WHERE quotation_id = ? AND deleted_at IS NULL)", billID,
			)).Error; err != nil {
			return err
		}

		// If the bill is already issued, keep its debt/credit ledger consistent:
		// the locked total drops by the item's total, so ขาด/เกิน (amount) rises by
		// it, and the reference weight/avg shrink. (No row → no-op.)
		if bill.IssuedQuotationID != nil {
			var bal entity.BillBalance
			if err := tx.Where("quotation_id = ?", *bill.IssuedQuotationID).First(&bal).Error; err == nil {
				lockedTotal := bal.AvgPrice*bal.Weight - item.Total
				newWeight := bal.Weight - item.Weight
				newAvg := 0.0
				if newWeight > 0 {
					newAvg = lockedTotal / newWeight
				}
				if err := tx.Model(&entity.BillBalance{}).Where("id = ?", bal.ID).
					Updates(map[string]interface{}{
						"amount":    bal.Amount + item.Total,
						"weight":    newWeight,
						"avg_price": newAvg,
					}).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
	return int(remaining), err
}

// Delete soft-deletes the bill and cascades a soft-delete to its items, images,
// debt/credit balances and delivery logs so the bill drops out of debt totals.
// Debt balances are keyed to the bill's issued quotation, so we clear both ids.
// Credit transactions are left intact (no refund) for history.
func (r *billRepository) Delete(id uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var bill entity.Quotation
		if err := tx.Where("is_bill = ?", true).First(&bill, id).Error; err != nil {
			return err
		}
		balanceIDs := []uint{id}
		if bill.IssuedQuotationID != nil {
			balanceIDs = append(balanceIDs, *bill.IssuedQuotationID)
		}
		if err := tx.Where("quotation_id = ?", id).Delete(&entity.QuotationItem{}).Error; err != nil {
			return err
		}
		if err := tx.Where("quotation_id = ?", id).Delete(&entity.QuotationImage{}).Error; err != nil {
			return err
		}
		if err := tx.Where("quotation_id IN ?", balanceIDs).Delete(&entity.BillBalance{}).Error; err != nil {
			return err
		}
		if err := tx.Where("bill_id = ?", id).Delete(&entity.BillDeliveryLog{}).Error; err != nil {
			return err
		}
		return tx.Where("is_bill = ?", true).Delete(&entity.Quotation{}, id).Error
	})
}

// RevertIssuance is the inverse of an issuance: it keeps the bill(s) but resets
// them to "รอออกบิล" (10) and clears everything the issuance created. Debt
// balances are keyed to the issued quotation; delivery logs and processed
// totals are per-bill. Mirrors Delete's cleanup but without removing the bills.
// Credit transactions are left intact (no refund), consistent with Delete.
func (r *billRepository) RevertIssuance(id uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var bill entity.Quotation
		if err := tx.Where("is_bill = ?", true).First(&bill, id).Error; err != nil {
			return err
		}

		// No issued quotation (bill was advanced via the plain "ออกบิล" button):
		// just move it back to รอออกบิล.
		if bill.IssuedQuotationID == nil {
			return tx.Model(&entity.Quotation{}).Where("id = ?", id).
				Update("status", StatusPendingIssue).Error
		}

		qid := *bill.IssuedQuotationID

		// All bills that were issued together share this quotation.
		var billIDs []uint
		if err := tx.Model(&entity.Quotation{}).
			Where("is_bill = ? AND issued_quotation_id = ?", true, qid).
			Pluck("id", &billIDs).Error; err != nil {
			return err
		}

		// Reset each bill and drop its delivery logs.
		if err := tx.Model(&entity.Quotation{}).Where("id IN ?", billIDs).
			Updates(map[string]interface{}{
				"status":              StatusPendingIssue,
				"issued_quotation_id": nil,
				"processed_weight":    0,
				"processed_amount":    0,
			}).Error; err != nil {
			return err
		}
		if err := tx.Where("bill_id IN ?", billIDs).Delete(&entity.BillDeliveryLog{}).Error; err != nil {
			return err
		}

		// Reverse the debt/credit ledger entry recorded for this issuance.
		if err := tx.Where("quotation_id = ?", qid).Delete(&entity.BillBalance{}).Error; err != nil {
			return err
		}

		// Soft-delete the issued quotation and its items/images.
		if err := tx.Where("quotation_id = ?", qid).Delete(&entity.QuotationItem{}).Error; err != nil {
			return err
		}
		if err := tx.Where("quotation_id = ?", qid).Delete(&entity.QuotationImage{}).Error; err != nil {
			return err
		}
		return tx.Delete(&entity.Quotation{}, qid).Error
	})
}

func (r *billRepository) GenerateCode() (string, error) {
	var count int64
	r.db.Unscoped().Model(&entity.Quotation{}).Where("is_bill = ?", true).Count(&count)
	return fmt.Sprintf("BILL%04d", count+1), nil
}

func (r *billRepository) AddImages(billID uint, urls []string) error {
	var images []entity.QuotationImage
	for _, url := range urls {
		images = append(images, entity.QuotationImage{QuotationID: billID, ImageURL: url})
	}
	if len(images) == 0 {
		return nil
	}
	return r.db.Create(&images).Error
}

func (r *billRepository) LogDelivery(billID uint, weight, amount float64, note string, items json.RawMessage) error {
	if len(items) == 0 {
		items = json.RawMessage("[]")
	}
	return r.db.Create(&entity.BillDeliveryLog{
		BillID: billID,
		Weight: weight,
		Amount: amount,
		Note:   note,
		Items:  items,
	}).Error
}

func (r *billRepository) GetDeliveryLogs(billID uint) ([]entity.BillDeliveryLog, error) {
	var logs []entity.BillDeliveryLog
	err := r.db.Where("bill_id = ?", billID).Order("created_at ASC").Find(&logs).Error
	return logs, err
}

func (r *billRepository) PartialDeliver(billID uint, weight, amount float64) (*entity.Quotation, error) {
	err := r.db.Model(&entity.Quotation{}).
		Where("id = ? AND is_bill = ?", billID, true).
		Updates(map[string]interface{}{
			"processed_weight": gorm.Expr("processed_weight + ?", weight),
			"processed_amount": gorm.Expr("processed_amount + ?", amount),
		}).Error
	if err != nil {
		return nil, err
	}
	return r.FindByID(billID)
}

// CountUnfinished counts bills that are not yet completed/cancelled
// (status 10 = waiting to issue, 11 = waiting to review), split per metal so the
// รายการขายทอง / รายการขายเงิน menu entries each get their own badge.
func (r *billRepository) CountUnfinished(storeID *uint, branchID *uint, createdBy *uint) (UnfinishedCounts, error) {
	var counts UnfinishedCounts
	query := r.db.Model(&entity.Quotation{}).
		Where("is_bill = ?", true).
		Where("status IN ?", []int{StatusPendingIssue, StatusPendingReview})
	if storeID != nil {
		query = query.Where("store_id = ?", *storeID)
	}
	if branchID != nil {
		query = query.Where("branch_id = ?", *branchID)
	}
	if createdBy != nil {
		query = query.Where("created_by = ?", *createdBy)
	}

	var rows []struct {
		Metal string
		Count int64
	}
	if err := query.Select("metal, COUNT(*) AS count").Group("metal").Scan(&rows).Error; err != nil {
		return counts, err
	}
	for _, row := range rows {
		counts.Total += row.Count
		// Legacy rows predating the metal column read as gold (see migration 85).
		if row.Metal == "" || row.Metal == "gold" {
			counts.Gold += row.Count
		} else {
			counts.Silver += row.Count
		}
	}
	return counts, nil
}

// Bill status values (kept distinct from staff quotation statuses 0/1/2).
const (
	StatusPendingIssue  = 10 // รอออกบิล
	StatusPendingReview = 11 // รอตรวจบิล
	StatusCompleted     = 12 // สำเร็จ
	StatusCancelled     = 13 // ยกเลิก
	StatusCleared       = 14 // เคลียร์บิลแล้ว
)

func (r *billRepository) ClearBills(storeID *uint, billIDs []uint) (int64, error) {
	var cleared int64
	err := r.db.Transaction(func(tx *gorm.DB) error {
		// Resolve the target bills (status 12, in scope). A partial selection is
		// expanded to whole issue-groups: bills issued together share one ledger
		// row, so they must settle together.
		q := tx.Model(&entity.Quotation{}).Where("is_bill = ? AND status = ?", true, StatusCompleted)
		if storeID != nil {
			q = q.Where("store_id = ?", *storeID)
		}
		if len(billIDs) > 0 {
			var qids []uint
			if err := tx.Model(&entity.Quotation{}).
				Where("is_bill = ? AND id IN ? AND issued_quotation_id IS NOT NULL", true, billIDs).
				Distinct().Pluck("issued_quotation_id", &qids).Error; err != nil {
				return err
			}
			if len(qids) > 0 {
				q = q.Where("id IN ? OR issued_quotation_id IN ?", billIDs, qids)
			} else {
				q = q.Where("id IN ?", billIDs)
			}
		}
		var targets []entity.Quotation
		if err := q.Select("id", "issued_quotation_id").Find(&targets).Error; err != nil {
			return err
		}
		if len(targets) == 0 {
			return nil
		}

		ids := make([]uint, 0, len(targets))
		// Ledger rows are keyed by the issued quotation's id; include the bill id
		// too, defensively, mirroring Delete's cleanup.
		ledgerIDs := make([]uint, 0, len(targets)*2)
		for _, b := range targets {
			ids = append(ids, b.ID)
			ledgerIDs = append(ledgerIDs, b.ID)
			if b.IssuedQuotationID != nil {
				ledgerIDs = append(ledgerIDs, *b.IssuedQuotationID)
			}
		}

		res := tx.Model(&entity.Quotation{}).Where("id IN ?", ids).Update("status", StatusCleared)
		if res.Error != nil {
			return res.Error
		}
		cleared = res.RowsAffected

		// Settle the cleared bills' debt/credit ledger rows: kept for history but
		// excluded from the balance/average from now on (see GetBalance).
		return tx.Model(&entity.BillBalance{}).
			Where("quotation_id IN ? AND settled_at IS NULL", ledgerIDs).
			Update("settled_at", time.Now()).Error
	})
	return cleared, err
}
