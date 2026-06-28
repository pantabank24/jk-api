package repository

import (
	"time"

	"jk-api/internal/entity"

	"gorm.io/gorm"
)

type MetalPriceRepository interface {
	Create(mp *entity.MetalPrice) error
	GetLatest(symbol string) (*entity.MetalPrice, error)
	GetHistory(symbol string, limit int) ([]entity.MetalPrice, error)
}

type metalPriceRepository struct {
	db *gorm.DB
}

func NewMetalPriceRepository(db *gorm.DB) MetalPriceRepository {
	return &metalPriceRepository{db: db}
}

func (r *metalPriceRepository) Create(mp *entity.MetalPrice) error {
	return r.db.Create(mp).Error
}

// GetLatest returns the active manual override (a manual row whose window covers
// now), otherwise the latest auto-fetched price for the symbol.
func (r *metalPriceRepository) GetLatest(symbol string) (*entity.MetalPrice, error) {
	now := time.Now()
	var mp entity.MetalPrice

	if err := r.db.Where("symbol = ? AND source = ? AND valid_from <= ? AND valid_until >= ?", symbol, "manual", now, now).
		Order("id DESC").First(&mp).Error; err == nil {
		return &mp, nil
	}
	if err := r.db.Where("symbol = ? AND source = ?", symbol, "auto").Order("id DESC").First(&mp).Error; err == nil {
		return &mp, nil
	}
	if err := r.db.Where("symbol = ?", symbol).Order("id DESC").First(&mp).Error; err != nil {
		return nil, err
	}
	return &mp, nil
}

func (r *metalPriceRepository) GetHistory(symbol string, limit int) ([]entity.MetalPrice, error) {
	var prices []entity.MetalPrice
	err := r.db.Where("symbol = ?", symbol).Order("id DESC").Limit(limit).Find(&prices).Error
	return prices, err
}
