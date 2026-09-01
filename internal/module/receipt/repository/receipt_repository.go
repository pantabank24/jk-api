package repository

import (
	"jk-api/internal/entity"

	"gorm.io/gorm"
)

// SettingsID is the fixed primary key of the singleton settings row (see the
// receipt_settings_singleton check constraint in migration 000095).
const SettingsID = 1

type ReceiptFilter struct {
	// Matches เลขที่, อ้างอิง or the customer name — one box on the list page.
	Search string
}

type ReceiptRepository interface {
	FindAll(f ReceiptFilter, page, limit int) ([]entity.AdminReceipt, int64, error)
	FindByID(id uint) (*entity.AdminReceipt, error)
	Create(r *entity.AdminReceipt) error
	// Update replaces the receipt and its item rows in one transaction — items are
	// positional, so an edit rewrites the whole table rather than diffing rows.
	Update(r *entity.AdminReceipt, items []entity.AdminReceiptItem) error
	Delete(id uint) error

	GetSettings() (*entity.ReceiptSettings, error)
	SaveSettings(s *entity.ReceiptSettings) error
}

type receiptRepository struct {
	db *gorm.DB
}

func NewReceiptRepository(db *gorm.DB) ReceiptRepository {
	return &receiptRepository{db: db}
}

func (r *receiptRepository) scope(f ReceiptFilter) *gorm.DB {
	q := r.db.Model(&entity.AdminReceipt{})
	if f.Search != "" {
		like := "%" + f.Search + "%"
		q = q.Where("code ILIKE ? OR reference ILIKE ? OR customer_name ILIKE ?", like, like, like)
	}
	return q
}

func (r *receiptRepository) FindAll(f ReceiptFilter, page, limit int) ([]entity.AdminReceipt, int64, error) {
	var receipts []entity.AdminReceipt
	var total int64

	q := r.scope(f)
	q.Count(&total)
	// Newest receipt date first. The code is typed by hand and carries no order.
	err := q.Preload("Items", func(db *gorm.DB) *gorm.DB {
		return db.Order("sort_order ASC, id ASC")
	}).Preload("Creator").
		Offset((page - 1) * limit).Limit(limit).
		Order("issued_date DESC, id DESC").Find(&receipts).Error
	return receipts, total, err
}

func (r *receiptRepository) FindByID(id uint) (*entity.AdminReceipt, error) {
	var receipt entity.AdminReceipt
	err := r.db.Preload("Items", func(db *gorm.DB) *gorm.DB {
		return db.Order("sort_order ASC, id ASC")
	}).Preload("Creator").First(&receipt, id).Error
	if err != nil {
		return nil, err
	}
	return &receipt, nil
}

func (r *receiptRepository) Create(receipt *entity.AdminReceipt) error {
	return r.db.Create(receipt).Error
}

func (r *receiptRepository) Update(receipt *entity.AdminReceipt, items []entity.AdminReceiptItem) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(receipt).Error; err != nil {
			return err
		}
		// Hard-delete the old lines (Unscoped): they are replaced wholesale on every
		// save, and soft-deleted rows would pile up invisibly behind each edit.
		if err := tx.Unscoped().Where("receipt_id = ?", receipt.ID).
			Delete(&entity.AdminReceiptItem{}).Error; err != nil {
			return err
		}
		if len(items) == 0 {
			return nil
		}
		for i := range items {
			items[i].ID = 0
			items[i].ReceiptID = receipt.ID
		}
		return tx.Create(&items).Error
	})
}

func (r *receiptRepository) Delete(id uint) error {
	return r.db.Delete(&entity.AdminReceipt{}, id).Error
}

func (r *receiptRepository) GetSettings() (*entity.ReceiptSettings, error) {
	var s entity.ReceiptSettings
	// The row is seeded by the migration; recreate it rather than 404 if it is ever
	// missing, so the settings modal always has something to open.
	if err := r.db.First(&s, SettingsID).Error; err != nil {
		s = entity.ReceiptSettings{ID: SettingsID, DocTitle: "ใบกำกับภาษี/ใบเสร็จรับเงิน"}
		if err := r.db.Create(&s).Error; err != nil {
			return nil, err
		}
	}
	return &s, nil
}

func (r *receiptRepository) SaveSettings(s *entity.ReceiptSettings) error {
	s.ID = SettingsID
	return r.db.Save(s).Error
}
