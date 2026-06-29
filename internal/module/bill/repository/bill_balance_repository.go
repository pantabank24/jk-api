package repository

import (
	"jk-api/internal/entity"

	"gorm.io/gorm"
)

type BillBalanceRepository interface {
	Record(userID uint, storeID *uint, quotationID *uint, amount float64, description string) error
	GetBalance(userID uint) (float64, error)
	GetHistory(userID uint, limit int) ([]entity.BillBalance, error)
}

type billBalanceRepository struct {
	db *gorm.DB
}

func NewBillBalanceRepository(db *gorm.DB) BillBalanceRepository {
	return &billBalanceRepository{db: db}
}

func (r *billBalanceRepository) Record(userID uint, storeID *uint, quotationID *uint, amount float64, description string) error {
	return r.db.Create(&entity.BillBalance{
		UserID:      userID,
		StoreID:     storeID,
		QuotationID: quotationID,
		Amount:      amount,
		Description: description,
	}).Error
}

func (r *billBalanceRepository) GetBalance(userID uint) (float64, error) {
	var total float64
	err := r.db.Model(&entity.BillBalance{}).
		Where("user_id = ?", userID).
		Select("COALESCE(SUM(amount), 0)").
		Scan(&total).Error
	return total, err
}

func (r *billBalanceRepository) GetHistory(userID uint, limit int) ([]entity.BillBalance, error) {
	var records []entity.BillBalance
	err := r.db.Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).
		Find(&records).Error
	return records, err
}
