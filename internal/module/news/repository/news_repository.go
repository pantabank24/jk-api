package repository

import (
	"jk-api/internal/entity"

	"gorm.io/gorm"
)

type NewsRepository interface {
	Create(news *entity.News) error
	FindAll(page, limit int) ([]entity.News, int64, error)
	// FindVisible returns news whose audience is in the given list (nil/empty = no filter, all audiences).
	FindVisible(audiences []string, page, limit int) ([]entity.News, int64, error)
	FindByID(id uint) (*entity.News, error)
	Update(news *entity.News) error
	Delete(id uint) error
}

type newsRepository struct {
	db *gorm.DB
}

func NewNewsRepository(db *gorm.DB) NewsRepository {
	return &newsRepository{db: db}
}

func (r *newsRepository) Create(news *entity.News) error {
	return r.db.Create(news).Error
}

func (r *newsRepository) FindAll(page, limit int) ([]entity.News, int64, error) {
	var news []entity.News
	var total int64

	r.db.Model(&entity.News{}).Count(&total)
	offset := (page - 1) * limit
	err := r.db.Preload("Creator").Offset(offset).Limit(limit).Order("id DESC").Find(&news).Error
	return news, total, err
}

func (r *newsRepository) FindVisible(audiences []string, page, limit int) ([]entity.News, int64, error) {
	var news []entity.News
	var total int64

	query := r.db.Model(&entity.News{})
	if len(audiences) > 0 {
		query = query.Where("audience IN ?", audiences)
	}

	query.Count(&total)
	offset := (page - 1) * limit
	err := query.Preload("Creator").Offset(offset).Limit(limit).Order("id DESC").Find(&news).Error
	return news, total, err
}

func (r *newsRepository) FindByID(id uint) (*entity.News, error) {
	var news entity.News
	err := r.db.Preload("Creator").First(&news, id).Error
	if err != nil {
		return nil, err
	}
	return &news, nil
}

func (r *newsRepository) Update(news *entity.News) error {
	return r.db.Save(news).Error
}

func (r *newsRepository) Delete(id uint) error {
	return r.db.Delete(&entity.News{}, id).Error
}
