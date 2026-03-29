package repository

import (
	"fmt"

	"jk-api/internal/entity"

	"gorm.io/gorm"
)

type BranchRepository interface {
	Create(branch *entity.Branch) error
	FindAllByStoreID(storeID uint, page, limit int) ([]entity.Branch, int64, error)
	FindByID(id uint) (*entity.Branch, error)
	Update(branch *entity.Branch) error
	Delete(id uint) error
	GenerateCode() (string, error)
}

type branchRepository struct {
	db *gorm.DB
}

func NewBranchRepository(db *gorm.DB) BranchRepository {
	return &branchRepository{db: db}
}

func (r *branchRepository) Create(branch *entity.Branch) error {
	return r.db.Create(branch).Error
}

func (r *branchRepository) FindAllByStoreID(storeID uint, page, limit int) ([]entity.Branch, int64, error) {
	var branches []entity.Branch
	var total int64

	query := r.db.Model(&entity.Branch{}).Where("store_id = ?", storeID)
	query.Count(&total)

	offset := (page - 1) * limit
	err := query.Offset(offset).Limit(limit).Order("id DESC").Find(&branches).Error
	return branches, total, err
}

func (r *branchRepository) FindByID(id uint) (*entity.Branch, error) {
	var branch entity.Branch
	err := r.db.Preload("Store").First(&branch, id).Error
	if err != nil {
		return nil, err
	}
	return &branch, nil
}

func (r *branchRepository) Update(branch *entity.Branch) error {
	return r.db.Save(branch).Error
}

func (r *branchRepository) Delete(id uint) error {
	return r.db.Delete(&entity.Branch{}, id).Error
}

func (r *branchRepository) GenerateCode() (string, error) {
	var count int64
	r.db.Unscoped().Model(&entity.Branch{}).Count(&count)
	return fmt.Sprintf("BRN%04d", count+1), nil
}
