package repository

import (
	"time"

	"jk-api/internal/entity"

	"gorm.io/gorm"
)

type GoldPriceRepository interface {
	Create(gp *entity.GoldPrice) error
	GetLatest() (*entity.GoldPrice, error)
	GetHistory(limit int) ([]entity.GoldPrice, error)
}

type goldPriceRepository struct {
	db *gorm.DB
}

func NewGoldPriceRepository(db *gorm.DB) GoldPriceRepository {
	return &goldPriceRepository{db: db}
}

func (r *goldPriceRepository) Create(gp *entity.GoldPrice) error {
	return r.db.Create(gp).Error
}

// GetLatest returns the active manual override (a manual row whose window covers
// now), otherwise the latest auto-fetched price; once a manual window passes it
// falls back to auto automatically.
func (r *goldPriceRepository) GetLatest() (*entity.GoldPrice, error) {
	now := time.Now()
	var gp entity.GoldPrice

	if err := r.db.Where("source = ? AND valid_from <= ? AND valid_until >= ?", "manual", now, now).
		Order("id DESC").First(&gp).Error; err == nil {
		return &gp, nil
	}
	if err := r.db.Where("source = ?", "auto").Order("id DESC").First(&gp).Error; err == nil {
		return &gp, nil
	}
	// Ultimate fallback: any latest row.
	if err := r.db.Order("id DESC").First(&gp).Error; err != nil {
		return nil, err
	}
	return &gp, nil
}

func (r *goldPriceRepository) GetHistory(limit int) ([]entity.GoldPrice, error) {
	var prices []entity.GoldPrice
	err := r.db.Order("id DESC").Limit(limit).Find(&prices).Error
	return prices, err
}
