package repository

import (
	"fmt"
	"time"

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
	AddImages(quotationID uint, urls []string, imageType string) error
	// MarkBillIssued advances a customer bill (a quotation row with is_bill=true)
	// to "รอตรวจบิล" (status 11) and links it to the master-issued quotation.
	MarkBillIssued(billID, quotationID uint) error
	// FindBillsByIDs fetches bill rows by their IDs (used at issuance).
	FindBillsByIDs(ids []uint) ([]entity.Quotation, error)
	// SplitBillItems moves the given items of a pending bill into a brand-new bill
	// and recomputes both totals. Used when a quotation covers only part of a bill.
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
		query = query.Where("quotations.code ILIKE ?", "%"+search+"%")
	}

	query.Count(&total)
	offset := (page - 1) * limit
	err := query.Preload("Items").Preload("Images").Preload("Member").Preload("Member.User").Preload("Member.User.Bank").
		Preload("Creator").
		Offset(offset).Limit(limit).Order("quotations.id DESC").Find(&quotations).Error
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
	return &quotation, nil
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
		Updates(map[string]interface{}{"status": 11, "issued_quotation_id": quotationID}).Error
}

func (r *quotationRepository) FindBillsByIDs(ids []uint) ([]entity.Quotation, error) {
	var bills []entity.Quotation
	err := r.db.Preload("Items").Where("id IN ? AND is_bill = ?", ids, true).Find(&bills).Error
	return bills, err
}

// SplitBillItems moves the given items of a pending bill into a brand-new bill
// (same customer/store header, fresh BILL code) and recomputes both bills'
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
		// New bill code follows the bill module's BILL%04d scheme.
		var count int64
		tx.Unscoped().Model(&entity.Quotation{}).Where("is_bill = ?", true).Count(&count)

		newBill := entity.Quotation{
			StoreID:     bill.StoreID,
			BranchID:    bill.BranchID,
			MemberID:    bill.MemberID,
			CreatedBy:   bill.CreatedBy,
			Code:        fmt.Sprintf("BILL%04d", count+1),
			Status:      10, // รอออกบิล — caller marks it issued right after
			IsBill:      true,
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
	now := time.Now()
	buddhistYear := now.Year() + 543
	prefix := fmt.Sprintf("P%02d%02d", buddhistYear%100, int(now.Month()))

	var count int64
	r.db.Unscoped().Model(&entity.Quotation{}).
		Where("is_bill = ? AND code LIKE ?", false, prefix+"%").
		Count(&count)
	return fmt.Sprintf("%s%04d", prefix, count+1), nil
}
