package repository

import (
	"jk-api/internal/entity"

	"gorm.io/gorm"
)

type GoldTypeRepository interface {
	FindAll() ([]entity.GoldType, error)
	FindByID(id uint) (*entity.GoldType, error)
	Create(gt *entity.GoldType) error
	Update(gt *entity.GoldType) error
	Delete(id uint) error
}

type goldTypeRepository struct {
	db *gorm.DB
}

func NewGoldTypeRepository(db *gorm.DB) GoldTypeRepository {
	return &goldTypeRepository{db: db}
}

func (r *goldTypeRepository) FindAll() ([]entity.GoldType, error) {
	var types []entity.GoldType
	err := r.db.Order("sort_order ASC, id ASC").Find(&types).Error
	return types, err
}

func (r *goldTypeRepository) FindByID(id uint) (*entity.GoldType, error) {
	var gt entity.GoldType
	err := r.db.First(&gt, id).Error
	if err != nil {
		return nil, err
	}
	return &gt, nil
}

func (r *goldTypeRepository) Create(gt *entity.GoldType) error {
	return r.db.Create(gt).Error
}

func (r *goldTypeRepository) Update(gt *entity.GoldType) error {
	return r.db.Save(gt).Error
}

func (r *goldTypeRepository) Delete(id uint) error {
	return r.db.Delete(&entity.GoldType{}, id).Error
}
