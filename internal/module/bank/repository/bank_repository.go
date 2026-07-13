package repository

import (
	"jk-api/internal/entity"

	"gorm.io/gorm"
)

type BankRepository interface {
	FindAll() ([]entity.Bank, error)
	FindByID(id uint) (*entity.Bank, error)
	Create(b *entity.Bank) error
	Update(b *entity.Bank) error
	Delete(id uint) error
	// CountUsers reports how many customers still point at this bank, so a delete
	// that would blank their account details can be refused instead.
	CountUsers(id uint) (int64, error)
}

type bankRepository struct {
	db *gorm.DB
}

func NewBankRepository(db *gorm.DB) BankRepository {
	return &bankRepository{db: db}
}

func (r *bankRepository) FindAll() ([]entity.Bank, error) {
	var banks []entity.Bank
	err := r.db.Order("sort_order ASC, id ASC").Find(&banks).Error
	return banks, err
}

func (r *bankRepository) FindByID(id uint) (*entity.Bank, error) {
	var b entity.Bank
	if err := r.db.First(&b, id).Error; err != nil {
		return nil, err
	}
	return &b, nil
}

func (r *bankRepository) Create(b *entity.Bank) error {
	return r.db.Create(b).Error
}

func (r *bankRepository) Update(b *entity.Bank) error {
	return r.db.Save(b).Error
}

func (r *bankRepository) Delete(id uint) error {
	return r.db.Delete(&entity.Bank{}, id).Error
}

func (r *bankRepository) CountUsers(id uint) (int64, error) {
	var n int64
	err := r.db.Model(&entity.User{}).Where("bank_id = ?", id).Count(&n).Error
	return n, err
}
