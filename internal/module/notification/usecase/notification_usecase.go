package usecase

import (
	"jk-api/internal/entity"
	"jk-api/internal/module/notification/repository"
)

type NotificationUsecase interface {
	GetNotifications(userID uint, page, limit int) ([]entity.Notification, int64, error)
	MarkRead(id uint, userID uint) error
	MarkAllRead(userID uint) error
	CountUnread(userID uint) (int64, error)
}

type notificationUsecase struct {
	repo repository.NotificationRepository
}

func NewNotificationUsecase(repo repository.NotificationRepository) NotificationUsecase {
	return &notificationUsecase{repo: repo}
}

func (u *notificationUsecase) GetNotifications(userID uint, page, limit int) ([]entity.Notification, int64, error) {
	if page < 1 { page = 1 }
	if limit < 1 || limit > 50 { limit = 20 }
	return u.repo.FindByUserID(userID, page, limit)
}

func (u *notificationUsecase) MarkRead(id uint, userID uint) error {
	return u.repo.MarkRead(id, userID)
}

func (u *notificationUsecase) MarkAllRead(userID uint) error {
	return u.repo.MarkAllRead(userID)
}

func (u *notificationUsecase) CountUnread(userID uint) (int64, error) {
	return u.repo.CountUnread(userID)
}
