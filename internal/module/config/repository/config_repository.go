package repository

import (
	"jk-api/internal/entity"

	"gorm.io/gorm"
)

type ConfigRepository interface {
	GetAll() ([]entity.SystemConfig, error)
	GetByKey(key string) (*entity.SystemConfig, error)
	Set(key, value string) error
}

type configRepository struct {
	db *gorm.DB
}

func NewConfigRepository(db *gorm.DB) ConfigRepository {
	return &configRepository{db: db}
}

func (r *configRepository) GetAll() ([]entity.SystemConfig, error) {
	var configs []entity.SystemConfig
	err := r.db.Order("key ASC").Find(&configs).Error
	return configs, err
}

func (r *configRepository) GetByKey(key string) (*entity.SystemConfig, error) {
	var cfg entity.SystemConfig
	err := r.db.Where("key = ?", key).First(&cfg).Error
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (r *configRepository) Set(key, value string) error {
	return r.db.Model(&entity.SystemConfig{}).
		Where("key = ?", key).
		Update("value", value).Error
}
