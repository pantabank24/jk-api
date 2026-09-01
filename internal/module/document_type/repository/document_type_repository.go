package repository

import (
	"jk-api/internal/entity"

	"gorm.io/gorm"
)

type DocumentTypeRepository interface {
	FindAll() ([]entity.DocumentType, error)
	FindByID(id uint) (*entity.DocumentType, error)
	Create(t *entity.DocumentType) error
	Update(t *entity.DocumentType) error
	Delete(id uint) error
	// CountDocuments reports how many customer documents still carry this type, so a
	// delete that would strip their label can be refused instead.
	CountDocuments(id uint) (int64, error)
}

type documentTypeRepository struct {
	db *gorm.DB
}

func NewDocumentTypeRepository(db *gorm.DB) DocumentTypeRepository {
	return &documentTypeRepository{db: db}
}

func (r *documentTypeRepository) FindAll() ([]entity.DocumentType, error) {
	var types []entity.DocumentType
	err := r.db.Order("sort_order ASC, id ASC").Find(&types).Error
	return types, err
}

func (r *documentTypeRepository) FindByID(id uint) (*entity.DocumentType, error) {
	var t entity.DocumentType
	if err := r.db.First(&t, id).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *documentTypeRepository) Create(t *entity.DocumentType) error {
	return r.db.Create(t).Error
}

func (r *documentTypeRepository) Update(t *entity.DocumentType) error {
	return r.db.Save(t).Error
}

func (r *documentTypeRepository) Delete(id uint) error {
	return r.db.Delete(&entity.DocumentType{}, id).Error
}

func (r *documentTypeRepository) CountDocuments(id uint) (int64, error) {
	var n int64
	err := r.db.Model(&entity.CustomerDocument{}).Where("document_type_id = ?", id).Count(&n).Error
	return n, err
}
