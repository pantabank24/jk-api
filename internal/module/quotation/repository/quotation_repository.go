package repository

import (
	"fmt"
	"strings"
	"time"

	"jk-api/internal/documentcode"
	"jk-api/internal/entity"

	"gorm.io/gorm"
)

type QuotationRepository interface {
	Create(quotation *entity.Quotation) error
	FindAll(storeID *uint, branchID *uint, createdBy *uint, status *int, page, limit int, search string) ([]entity.Quotation, int64, error)
	FindByID(id uint) (*entity.Quotation, error)
	Update(quotation *entity.Quotation) error
	ReplaceItems(quotationID uint, items []entity.QuotationItem) error
	Delete(id uint) error
	GenerateCode() (string, error)
	GenerateBillCode() (string, error)
	AddImages(quotationID uint, urls []string, imageType string) error
	FindPendingAfterMelt(storeID *uint, branchID *uint, createdBy *uint) ([]entity.Quotation, error)
	ClearPendingAfterMelt(ids []uint, storeID *uint, branchID *uint, createdBy *uint) (int64, error)
	// MarkBillIssued advances a customer bill (a quotation row with is_bill=true)
	// to "รอตรวจบิล" (status 11) and links it to the master-issued quotation.
	MarkBillIssued(billID, quotationID uint) error
	// FindBillsByIDs fetches bill rows by their IDs (used at issuance).
	FindBillsByIDs(ids []uint) ([]entity.Quotation, error)
	// SplitBillItems moves the given items into a new same-prefix bill and
	// recomputes both totals. Used when a quotation covers only part of a bill.
	SplitBillItems(billID uint, itemIDs []uint) (uint, error)
	// FindUnrefundedApprovedByCreator returns this creator's approved quotations
	// whose charged credit hasn't been refunded yet (used by the credit-reset action).
	FindUnrefundedApprovedByCreator(userID uint) ([]entity.Quotation, error)
	// MarkCreditsRefunded flags the given quotation IDs as credits_refunded.
	MarkCreditsRefunded(ids []uint) error
}

type quotationRepository struct {
	db *gorm.DB
}

func NewQuotationRepository(db *gorm.DB) QuotationRepository {
	return &quotationRepository{db: db}
}

func (r *quotationRepository) Create(quotation *entity.Quotation) error {
	return r.db.Create(quotation).Error
}

func (r *quotationRepository) FindAll(storeID *uint, branchID *uint, createdBy *uint, status *int, page, limit int, search string) ([]entity.Quotation, int64, error) {
	var quotations []entity.Quotation
	var total int64

	// Exclude customer bills — they live in the same table but are managed by the bill module.
	query := r.db.Model(&entity.Quotation{}).Where("quotations.is_bill = ?", false)
	if storeID != nil {
		query = query.Where("quotations.store_id = ?", *storeID)
	}
	if branchID != nil {
		query = query.Where("quotations.branch_id = ?", *branchID)
	}
	if createdBy != nil {
		query = query.Where("quotations.created_by = ?", *createdBy)
	}
	if status != nil {
		query = query.Where("quotations.status = ?", *status)
	}
	if search != "" {
		pattern := "%" + search + "%"
		query = query.Where(`quotations.code ILIKE ? OR EXISTS (
			SELECT 1 FROM quotations source_bill
			WHERE source_bill.is_bill = true
			  AND source_bill.issued_quotation_id = quotations.id
			  AND source_bill.code ILIKE ?
		)`, pattern, pattern)
	}

	query.Count(&total)
	offset := (page - 1) * limit
	err := query.Preload("Items").Preload("Images").Preload("Member").Preload("Member.User").Preload("Member.User.Bank").
		Preload("Creator").
		Offset(offset).Limit(limit).Order("quotations.id DESC").Find(&quotations).Error
	if err == nil {
		err = r.attachDisplayCodes(quotations)
	}
	return quotations, total, err
}

func (r *quotationRepository) FindByID(id uint) (*entity.Quotation, error) {
	var quotation entity.Quotation
	// Bank relations feed the payout details printed on the quotation (ชำระโดย เงินโอน).
	err := r.db.Preload("Items").Preload("Images").Preload("Member").Preload("Member.User").Preload("Member.User.Bank").
		Preload("Creator").Preload("Creator.Bank").
		Preload("Store").Preload("Branch").First(&quotation, id).Error
	if err != nil {
		return nil, err
	}
	if code, err := r.displayCodeFor(quotation.ID, quotation.BillID); err == nil {
		quotation.DisplayCode = code
	}
	return &quotation, nil
}

// pendingAfterMeltQuery is shared by listing and clearing so "เคลียร์ทั้งหมด"
// can never affect a quotation outside the caller's store/branch/user scope.
func pendingAfterMeltQuery(db *gorm.DB, storeID *uint, branchID *uint, createdBy *uint) *gorm.DB {
	query := db.Model(&entity.Quotation{}).
		Where("quotations.is_bill = ?", false).
		Where("quotations.status <> ?", 2).
		Where("quotations.after_melt_cleared_at IS NULL").
		Where(`NOT EXISTS (
			SELECT 1 FROM quotation_images qi
			WHERE qi.quotation_id = quotations.id
			  AND qi.type = ?
			  AND qi.deleted_at IS NULL
		)`, "after_melt")
	if storeID != nil {
		query = query.Where("quotations.store_id = ?", *storeID)
	}
	if branchID != nil {
		query = query.Where("quotations.branch_id = ?", *branchID)
	}
	if createdBy != nil {
		query = query.Where("quotations.created_by = ?", *createdBy)
	}
	return query
}

func (r *quotationRepository) FindPendingAfterMelt(storeID *uint, branchID *uint, createdBy *uint) ([]entity.Quotation, error) {
	var quotations []entity.Quotation
	err := pendingAfterMeltQuery(r.db, storeID, branchID, createdBy).
		Preload("Items").Preload("Images").Preload("Member").
		Preload("Creator").Preload("Store").Preload("Branch").
		Order("quotations.created_at ASC").Find(&quotations).Error
	if err == nil {
		err = r.attachDisplayCodes(quotations)
	}
	return quotations, err
}

func (r *quotationRepository) ClearPendingAfterMelt(ids []uint, storeID *uint, branchID *uint, createdBy *uint) (int64, error) {
	query := pendingAfterMeltQuery(r.db, storeID, branchID, createdBy)
	if len(ids) > 0 {
		query = query.Where("quotations.id IN ?", ids)
	}
	now := time.Now()
	result := query.Update("after_melt_cleared_at", now)
	return result.RowsAffected, result.Error
}

type displayCodeRow struct {
	QuotationID uint
	Code        string
}

// attachDisplayCodes makes a quotation issued from an existing bill keep the
// originating bill number in every list/preview. It also covers legacy rows
// whose bill_id was not populated by resolving the reverse issued link.
func (r *quotationRepository) attachDisplayCodes(quotations []entity.Quotation) error {
	if len(quotations) == 0 {
		return nil
	}
	ids := make([]uint, 0, len(quotations))
	for i := range quotations {
		ids = append(ids, quotations[i].ID)
	}

	var rows []displayCodeRow
	err := r.db.Table("quotations AS issued").
		Select("issued.id AS quotation_id, source.code AS code").
		Joins("JOIN quotations AS source ON source.id = issued.bill_id").
		Where("issued.id IN ?", ids).
		Scan(&rows).Error
	if err != nil {
		return err
	}

	var legacy []displayCodeRow
	err = r.db.Table("quotations AS source").
		Select("source.issued_quotation_id AS quotation_id, MIN(source.code) AS code").
		Where("source.is_bill = ? AND source.issued_quotation_id IN ?", true, ids).
		Group("source.issued_quotation_id").
		Scan(&legacy).Error
	if err != nil {
		return err
	}

	byID := make(map[uint]string, len(rows)+len(legacy))
	for _, row := range legacy {
		byID[row.QuotationID] = row.Code
	}
	// The explicit bill_id is canonical for new rows and wins over the legacy
	// reverse-link fallback when several bills were combined.
	for _, row := range rows {
		byID[row.QuotationID] = row.Code
	}
	for i := range quotations {
		quotations[i].DisplayCode = byID[quotations[i].ID]
	}
	return nil
}

func (r *quotationRepository) displayCodeFor(quotationID uint, billID *uint) (string, error) {
	if billID != nil {
		var code string
		if err := r.db.Model(&entity.Quotation{}).Where("id = ?", *billID).Pluck("code", &code).Error; err != nil {
			return "", err
		}
		return code, nil
	}
	var code string
	err := r.db.Model(&entity.Quotation{}).
		Where("is_bill = ? AND issued_quotation_id = ?", true, quotationID).
		Order("id ASC").Limit(1).Pluck("code", &code).Error
	return code, err
}

func (r *quotationRepository) AddImages(quotationID uint, urls []string, imageType string) error {
	var images []entity.QuotationImage
	for _, url := range urls {
		images = append(images, entity.QuotationImage{QuotationID: quotationID, ImageURL: url, Type: imageType})
	}
	if len(images) == 0 {
		return nil
	}
	return r.db.Create(&images).Error
}

func (r *quotationRepository) Update(quotation *entity.Quotation) error {
	// Explicitly omit has-many associations so GORM does not cascade-save the
	// preloaded Items/Images slices. Without this, db.Save() in GORM v1.25.x
	// upserts every slice element, re-inserting hard-deleted rows and producing
	// duplicate items after an edit that calls ReplaceItems then Save.
	return r.db.Omit("Items", "Images").Save(quotation).Error
}

func (r *quotationRepository) ReplaceItems(quotationID uint, items []entity.QuotationItem) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Hard-delete the old items on edit (Unscoped) so replacing items doesn't
		// leave a trail of soft-deleted rows.
		if err := tx.Unscoped().Where("quotation_id = ?", quotationID).Delete(&entity.QuotationItem{}).Error; err != nil {
			return err
		}
		if len(items) > 0 {
			return tx.Create(&items).Error
		}
		return nil
	})
}

// Delete soft-deletes the quotation and cascades a soft-delete to its items and
// images. Credit transactions are intentionally left intact (no refund) so the
// member's history still shows what the credit was spent on.
func (r *quotationRepository) Delete(id uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("quotation_id = ?", id).Delete(&entity.QuotationItem{}).Error; err != nil {
			return err
		}
		if err := tx.Where("quotation_id = ?", id).Delete(&entity.QuotationImage{}).Error; err != nil {
			return err
		}
		return tx.Delete(&entity.Quotation{}, id).Error
	})
}

func (r *quotationRepository) MarkBillIssued(billID, quotationID uint) error {
	return r.db.Model(&entity.Quotation{}).
		Where("id = ? AND is_bill = ?", billID, true).
		Updates(map[string]interface{}{
			"status":              11,
			"issued_quotation_id": quotationID,
			"status_changed_at":   time.Now(),
		}).Error
}

func (r *quotationRepository) FindBillsByIDs(ids []uint) ([]entity.Quotation, error) {
	var bills []entity.Quotation
	err := r.db.Preload("Items").Where("id IN ? AND is_bill = ?", ids, true).Find(&bills).Error
	return bills, err
}

// SplitBillItems moves the given items of a pending bill into a brand-new bill
// (same customer/store header and origin prefix) and recomputes both bills'
// totals from their remaining items. The caller then marks the NEW bill issued,
// while the original keeps the leftover items and stays "รอออกบิล".
func (r *quotationRepository) SplitBillItems(billID uint, itemIDs []uint) (uint, error) {
	if len(itemIDs) == 0 {
		return 0, fmt.Errorf("no items to split")
	}
	var newID uint
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var bill entity.Quotation
		if err := tx.Where("id = ? AND is_bill = ?", billID, true).First(&bill).Error; err != nil {
			return err
		}
		// A split is still part of the same origin flow, so it keeps the source
		// prefix: Admin-originated P stays P; every other origin stays BILL.
		var code string
		var codeErr error
		if strings.HasPrefix(bill.Code, "P") {
			code, codeErr = documentcode.NextAdmin(tx)
		} else {
			code, codeErr = documentcode.NextBill(tx)
		}
		if codeErr != nil {
			return codeErr
		}

		newBill := entity.Quotation{
			StoreID:   bill.StoreID,
			BranchID:  bill.BranchID,
			MemberID:  bill.MemberID,
			CreatedBy: bill.CreatedBy,
			Code:      code,
			Status:    10, // รอออกบิล — caller marks it issued right after
			IsBill:    true,
			// Bills are single-metal; the split half must stay on the same list.
			Metal:       bill.Metal,
			GoldRound:   bill.GoldRound,
			GoldPriceID: bill.GoldPriceID,
		}
		if err := tx.Create(&newBill).Error; err != nil {
			return err
		}
		// Move the selected items across (guard on quotation_id so foreign item
		// ids can't be detached from another bill).
		if err := tx.Model(&entity.QuotationItem{}).
			Where("quotation_id = ? AND id IN ?", billID, itemIDs).
			Update("quotation_id", newBill.ID).Error; err != nil {
			return err
		}
		// Recompute both totals from the items each bill now holds.
		recompute := func(id uint) error {
			var total float64
			if err := tx.Model(&entity.QuotationItem{}).
				Where("quotation_id = ?", id).
				Select("COALESCE(SUM(total), 0)").Scan(&total).Error; err != nil {
				return err
			}
			return tx.Model(&entity.Quotation{}).Where("id = ?", id).
				Update("total_amount", total).Error
		}
		if err := recompute(billID); err != nil {
			return err
		}
		if err := recompute(newBill.ID); err != nil {
			return err
		}
		newID = newBill.ID
		return nil
	})
	return newID, err
}

func (r *quotationRepository) FindUnrefundedApprovedByCreator(userID uint) ([]entity.Quotation, error) {
	var quotations []entity.Quotation
	err := r.db.Where("created_by = ? AND status = ? AND credits_refunded = ? AND is_bill = ?", userID, 1, false, false).
		Order("id ASC").Find(&quotations).Error
	return quotations, err
}

func (r *quotationRepository) MarkCreditsRefunded(ids []uint) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.Model(&entity.Quotation{}).Where("id IN ?", ids).Update("credits_refunded", true).Error
}

func (r *quotationRepository) GenerateCode() (string, error) {
	return documentcode.NextAdmin(r.db)
}

func (r *quotationRepository) GenerateBillCode() (string, error) {
	return documentcode.NextBill(r.db)
}
