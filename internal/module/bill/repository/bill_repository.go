package repository

import (
	"fmt"

	"jk-api/internal/entity"

	"gorm.io/gorm"
)

// BillRepository manages customer bills. Bills are stored in the quotations table
// with is_bill = true; every query here is scoped to that flag so bills and staff
// quotations never leak into each other.
type BillRepository interface {
	Create(bill *entity.Quotation) error
	FindAll(storeID *uint, branchID *uint, createdBy *uint, status *int, page, limit int, search string) ([]entity.Quotation, int64, error)
	FindByID(id uint) (*entity.Quotation, error)
	// FindPendingByCreator returns a customer's open "รอออกบิล" bill, if any, so new
	// sells accumulate into it instead of creating separate bills.
	FindPendingByCreator(createdBy uint) (*entity.Quotation, error)
	AppendItems(billID uint, items []entity.QuotationItem) error
	Update(bill *entity.Quotation) error
	ReplaceItems(billID uint, items []entity.QuotationItem) error
	Delete(id uint) error
	GenerateCode() (string, error)
	AddImages(billID uint, urls []string) error
	CountUnfinished(storeID *uint, branchID *uint, createdBy *uint) (int64, error)
	// PartialDeliver accumulates processed_weight and processed_amount for a bill
	// when the master records a partial delivery without issuing the full quotation.
	PartialDeliver(billID uint, weight, amount float64) (*entity.Quotation, error)
	// LogDelivery appends one delivery-session record for audit/display.
	LogDelivery(billID uint, weight, amount float64, note string) error
	// GetDeliveryLogs returns all delivery-session records for a bill, oldest first.
	GetDeliveryLogs(billID uint) ([]entity.BillDeliveryLog, error)
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

func (r *billRepository) FindPendingByCreator(createdBy uint) (*entity.Quotation, error) {
	var bill entity.Quotation
	err := r.db.Where("is_bill = ? AND created_by = ? AND status = ?", true, createdBy, StatusPendingIssue).
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

func (r *billRepository) FindAll(storeID *uint, branchID *uint, createdBy *uint, status *int, page, limit int, search string) ([]entity.Quotation, int64, error) {
	var bills []entity.Quotation
	var total int64

	query := r.db.Model(&entity.Quotation{}).Where("quotations.is_bill = ?", true)
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
	// IssuedQuotation (code + total) lets the list show the issued quotation a bill
	// was rolled into; its items aren't needed until the detail view.
	err := query.Preload("Items").Preload("Images").Preload("Member").Preload("Creator").
		Preload("Store").Preload("Branch").Preload("IssuedQuotation").
		Offset(offset).Limit(limit).Order("quotations.id DESC").Find(&bills).Error
	return bills, total, err
}

func (r *billRepository) FindByID(id uint) (*entity.Quotation, error) {
	var bill entity.Quotation
	// IssuedQuotation (with its items/images) is the real bill the customer views —
	// they only have bills.read, not quotations.read.
	err := r.db.Preload("Items").Preload("Images").Preload("Member").Preload("Creator").
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
		if err := tx.Where("quotation_id = ?", billID).Delete(&entity.QuotationItem{}).Error; err != nil {
			return err
		}
		if len(items) > 0 {
			return tx.Create(&items).Error
		}
		return nil
	})
}

func (r *billRepository) Delete(id uint) error {
	return r.db.Where("is_bill = ?", true).Delete(&entity.Quotation{}, id).Error
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

func (r *billRepository) LogDelivery(billID uint, weight, amount float64, note string) error {
	return r.db.Create(&entity.BillDeliveryLog{
		BillID: billID,
		Weight: weight,
		Amount: amount,
		Note:   note,
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
// (status 10 = waiting to issue, 11 = waiting to review).
func (r *billRepository) CountUnfinished(storeID *uint, branchID *uint, createdBy *uint) (int64, error) {
	var count int64
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
	err := query.Count(&count).Error
	return count, err
}

// Bill status values (kept distinct from staff quotation statuses 0/1/2).
const (
	StatusPendingIssue  = 10 // รอออกบิล
	StatusPendingReview = 11 // รอตรวจบิล
	StatusCompleted     = 12 // สำเร็จ
	StatusCancelled     = 13 // ยกเลิก
)
