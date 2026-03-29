package repository

import (
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

func (r *goldPriceRepository) GetLatest() (*entity.GoldPrice, error) {
	var gp entity.GoldPrice
	err := r.db.Order("id DESC").First(&gp).Error
	if err != nil {
		return nil, err
	}
	return &gp, nil
}

func (r *goldPriceRepository) GetHistory(limit int) ([]entity.GoldPrice, error) {
	var prices []entity.GoldPrice
	err := r.db.Order("id DESC").Limit(limit).Find(&prices).Error
	return prices, err
}
