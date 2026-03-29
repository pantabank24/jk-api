package repository

import (
	"fmt"

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
	AddImages(quotationID uint, urls []string) error
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

	query := r.db.Model(&entity.Quotation{})
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
	err := query.Preload("Items").Preload("Images").Preload("Member").Preload("Creator").
		Offset(offset).Limit(limit).Order("quotations.id DESC").Find(&quotations).Error
	return quotations, total, err
}

func (r *quotationRepository) FindByID(id uint) (*entity.Quotation, error) {
	var quotation entity.Quotation
	err := r.db.Preload("Items").Preload("Images").Preload("Member").Preload("Creator").
		Preload("Store").Preload("Branch").First(&quotation, id).Error
	if err != nil {
		return nil, err
	}
	return &quotation, nil
}

func (r *quotationRepository) AddImages(quotationID uint, urls []string) error {
	var images []entity.QuotationImage
	for _, url := range urls {
		images = append(images, entity.QuotationImage{QuotationID: quotationID, ImageURL: url})
	}
	if len(images) == 0 {
		return nil
	}
	return r.db.Create(&images).Error
}

func (r *quotationRepository) Update(quotation *entity.Quotation) error {
	return r.db.Save(quotation).Error
}

func (r *quotationRepository) ReplaceItems(quotationID uint, items []entity.QuotationItem) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("quotation_id = ?", quotationID).Delete(&entity.QuotationItem{}).Error; err != nil {
			return err
		}
		if len(items) > 0 {
			return tx.Create(&items).Error
		}
		return nil
	})
}

func (r *quotationRepository) Delete(id uint) error {
	return r.db.Delete(&entity.Quotation{}, id).Error
}

func (r *quotationRepository) GenerateCode() (string, error) {
	var count int64
	r.db.Unscoped().Model(&entity.Quotation{}).Count(&count)
	return fmt.Sprintf("QUO%04d", count+1), nil
}
