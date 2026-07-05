package repository

import (
	"jk-api/internal/entity"

	"gorm.io/gorm"
)

type SalesScheduleRepository interface {
	GetAll() ([]entity.SalesSchedule, error)
	Create(s *entity.SalesSchedule) error
	Update(s *entity.SalesSchedule) error
	Delete(id uint) error
}

type salesScheduleRepository struct {
	db *gorm.DB
}

func NewSalesScheduleRepository(db *gorm.DB) SalesScheduleRepository {
	return &salesScheduleRepository{db: db}
}

func (r *salesScheduleRepository) GetAll() ([]entity.SalesSchedule, error) {
	var rules []entity.SalesSchedule
	// Weekday rules first (ordered Sun..Sat), then range rules by start.
	err := r.db.Order("scope ASC, weekday ASC, start_at ASC").Find(&rules).Error
	return rules, err
}

func (r *salesScheduleRepository) Create(s *entity.SalesSchedule) error {
	return r.db.Create(s).Error
}

func (r *salesScheduleRepository) Update(s *entity.SalesSchedule) error {
	return r.db.Model(&entity.SalesSchedule{}).Where("id = ?", s.ID).Updates(map[string]any{
		"weekday":              s.Weekday,
		"start_at":             s.StartAt,
		"end_at":               s.EndAt,
		"enabled":              s.Enabled,
		"open_time":            s.OpenTime,
		"close_time":           s.CloseTime,
		"realtime_after_hours": s.RealtimeAfterHours,
		"realtime_until":       s.RealtimeUntil,
		"note":                 s.Note,
	}).Error
}

func (r *salesScheduleRepository) Delete(id uint) error {
	return r.db.Delete(&entity.SalesSchedule{}, id).Error
}
