package repository

import (
	"fmt"

	"jk-api/internal/entity"

	"gorm.io/gorm"
)

type StoreRepository interface {
	Create(store *entity.Store) error
	FindAll(page, limit int) ([]entity.Store, int64, error)
	FindByID(id uint) (*entity.Store, error)
	Update(store *entity.Store) error
	Delete(id uint) error
	GenerateCode() (string, error)
	// SetMain makes storeID the system's sole main store (unsets the others).
	SetMain(storeID uint) error
	Count() (int64, error)
}

type storeRepository struct {
	db *gorm.DB
}

func NewStoreRepository(db *gorm.DB) StoreRepository {
	return &storeRepository{db: db}
}

func (r *storeRepository) Create(store *entity.Store) error {
	return r.db.Create(store).Error
}

func (r *storeRepository) FindAll(page, limit int) ([]entity.Store, int64, error) {
	var stores []entity.Store
	var total int64

	r.db.Model(&entity.Store{}).Count(&total)
	offset := (page - 1) * limit
	err := r.db.Preload("Branches").Offset(offset).Limit(limit).Order("is_main DESC, id DESC").Find(&stores).Error
	return stores, total, err
}

func (r *storeRepository) FindByID(id uint) (*entity.Store, error) {
	var store entity.Store
	err := r.db.Preload("Branches").First(&store, id).Error
	if err != nil {
		return nil, err
	}
	return &store, nil
}

func (r *storeRepository) Update(store *entity.Store) error {
	return r.db.Save(store).Error
}

// Delete soft-deletes the store and cascades a soft-delete to its branches.
// Users/members under the store are left intact (recoverable).
func (r *storeRepository) Delete(id uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("store_id = ?", id).Delete(&entity.Branch{}).Error; err != nil {
			return err
		}
		return tx.Delete(&entity.Store{}, id).Error
	})
}

// SetMain makes storeID the system's sole main store. Runs in a transaction so
// there's never a moment with two mains (or none).
func (r *storeRepository) SetMain(storeID uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&entity.Store{}).
			Where("id <> ?", storeID).
			Update("is_main", false).Error; err != nil {
			return err
		}
		return tx.Model(&entity.Store{}).
			Where("id = ?", storeID).
			Update("is_main", true).Error
	})
}

func (r *storeRepository) Count() (int64, error) {
	var count int64
	err := r.db.Model(&entity.Store{}).Count(&count).Error
	return count, err
}

func (r *storeRepository) GenerateCode() (string, error) {
	var count int64
	r.db.Unscoped().Model(&entity.Store{}).Count(&count)
	return fmt.Sprintf("STR%04d", count+1), nil
}
