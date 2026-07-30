package repository

import (
	"jk-api/internal/entity"

	"gorm.io/gorm"
)

type LogRepository interface {
	CreateLoginLog(log *entity.LoginLog) error
	CreateActivityLog(log *entity.ActivityLog) error
	GetLoginLogs(userID *uint, success *bool, page, limit int) ([]entity.LoginLog, int64, error)
	GetActivityLogs(userID *uint, customerID *uint, method string, describedOnly bool, page, limit int) ([]entity.ActivityLog, int64, error)
	DeleteLoginLogsBefore(days int) (int64, error)
	DeleteActivityLogsBefore(days int) (int64, error)
}

type logRepository struct {
	db *gorm.DB
}

func NewLogRepository(db *gorm.DB) LogRepository {
	return &logRepository{db: db}
}

func (r *logRepository) CreateLoginLog(log *entity.LoginLog) error {
	return r.db.Create(log).Error
}

func (r *logRepository) CreateActivityLog(log *entity.ActivityLog) error {
	return r.db.Create(log).Error
}

func (r *logRepository) GetLoginLogs(userID *uint, success *bool, page, limit int) ([]entity.LoginLog, int64, error) {
	var logs []entity.LoginLog
	var total int64

	query := r.db.Model(&entity.LoginLog{}).Preload("User")
	if userID != nil {
		query = query.Where("user_id = ?", *userID)
	}
	if success != nil {
		query = query.Where("success = ?", *success)
	}

	query.Count(&total)
	offset := (page - 1) * limit
	err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&logs).Error
	return logs, total, err
}

func (r *logRepository) GetActivityLogs(userID *uint, customerID *uint, method string, describedOnly bool, page, limit int) ([]entity.ActivityLog, int64, error) {
	var logs []entity.ActivityLog
	var total int64

	query := r.db.Model(&entity.ActivityLog{}).Preload("User").Preload("TargetUser")
	if userID != nil {
		query = query.Where("user_id = ?", *userID)
	}
	// A customer's full trail is both what they did and what was done to them:
	// they click sell, then staff issue/approve/cancel the resulting bill under
	// their own user_id. Filtering on user_id alone would end the story at the click.
	// Parenthesised explicitly: without them the OR would reach across the
	// method / described filters below and return the customer's whole history
	// regardless of what was asked for.
	if customerID != nil {
		query = query.Where("(user_id = ? OR target_user_id = ?)", *customerID, *customerID)
	}
	if method != "" {
		query = query.Where("method = ?", method)
	}
	// Every request is logged, so an unfiltered list is mostly page loads. A
	// description is only set for business actions, which makes it the dividing
	// line between the audit trail and the noise. Filtered here rather than in
	// the client so that paging counts what the reader actually sees.
	if describedOnly {
		query = query.Where("description <> ''")
	}

	query.Count(&total)
	offset := (page - 1) * limit
	err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&logs).Error
	return logs, total, err
}

func (r *logRepository) DeleteLoginLogsBefore(days int) (int64, error) {
	result := r.db.Where("created_at < NOW() - INTERVAL '1 day' * ?", days).Delete(&entity.LoginLog{})
	return result.RowsAffected, result.Error
}

func (r *logRepository) DeleteActivityLogsBefore(days int) (int64, error) {
	result := r.db.Where("created_at < NOW() - INTERVAL '1 day' * ?", days).Delete(&entity.ActivityLog{})
	return result.RowsAffected, result.Error
}
