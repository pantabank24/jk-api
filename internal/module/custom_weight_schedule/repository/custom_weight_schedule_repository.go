package repository

import (
	"jk-api/internal/entity"

	"gorm.io/gorm"
)

type CustomWeightScheduleRepository interface {
	GetAll() ([]entity.CustomWeightSchedule, error)
	Create(s *entity.CustomWeightSchedule) error
	Update(s *entity.CustomWeightSchedule) error
	Delete(id uint) error
}

type customWeightScheduleRepository struct {
	db *gorm.DB
}

func NewCustomWeightScheduleRepository(db *gorm.DB) CustomWeightScheduleRepository {
	return &customWeightScheduleRepository{db: db}
}

func (r *customWeightScheduleRepository) GetAll() ([]entity.CustomWeightSchedule, error) {
	var rules []entity.CustomWeightSchedule
	// Weekday rules first (ordered Sun..Sat), then range rules by start.
	err := r.db.Order("scope ASC, weekday ASC, start_at ASC").Find(&rules).Error
	return rules, err
}

func (r *customWeightScheduleRepository) Create(s *entity.CustomWeightSchedule) error {
	return r.db.Create(s).Error
}

func (r *customWeightScheduleRepository) Update(s *entity.CustomWeightSchedule) error {
	return r.db.Model(&entity.CustomWeightSchedule{}).Where("id = ?", s.ID).Updates(map[string]any{
		"start_at":   s.StartAt,
		"end_at":     s.EndAt,
		"enabled":    s.Enabled,
		"open_time":  s.OpenTime,
		"close_time": s.CloseTime,
		"note":       s.Note,
	}).Error
}

func (r *customWeightScheduleRepository) Delete(id uint) error {
	return r.db.Delete(&entity.CustomWeightSchedule{}, id).Error
}
