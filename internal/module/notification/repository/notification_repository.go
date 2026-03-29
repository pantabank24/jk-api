package repository

import (
	"jk-api/internal/entity"
	"gorm.io/gorm"
)

type NotificationRepository interface {
	Create(n *entity.Notification) error
	FindByUserID(userID uint, page, limit int) ([]entity.Notification, int64, error)
	MarkRead(id uint, userID uint) error
	MarkAllRead(userID uint) error
	CountUnread(userID uint) (int64, error)
}

type notificationRepository struct {
	db *gorm.DB
}

func NewNotificationRepository(db *gorm.DB) NotificationRepository {
	return &notificationRepository{db: db}
}

func (r *notificationRepository) Create(n *entity.Notification) error {
	return r.db.Create(n).Error
}

func (r *notificationRepository) FindByUserID(userID uint, page, limit int) ([]entity.Notification, int64, error) {
	var notifications []entity.Notification
	var total int64
	offset := (page - 1) * limit
	r.db.Model(&entity.Notification{}).Where("user_id = ?", userID).Count(&total)
	err := r.db.Where("user_id = ?", userID).Order("id DESC").Offset(offset).Limit(limit).Find(&notifications).Error
	return notifications, total, err
}

func (r *notificationRepository) MarkRead(id uint, userID uint) error {
	return r.db.Model(&entity.Notification{}).
		Where("id = ? AND user_id = ?", id, userID).
		Update("is_read", true).Error
}

func (r *notificationRepository) MarkAllRead(userID uint) error {
	return r.db.Model(&entity.Notification{}).
		Where("user_id = ? AND is_read = ?", userID, false).
		Update("is_read", true).Error
}

func (r *notificationRepository) CountUnread(userID uint) (int64, error) {
	var count int64
	err := r.db.Model(&entity.Notification{}).Where("user_id = ? AND is_read = ?", userID, false).Count(&count).Error
	return count, err
}
