package usecase

import (
	"jk-api/internal/entity"
	"jk-api/internal/module/log/repository"
)

type LogUsecase interface {
	GetLoginLogs(userID *uint, success *bool, page, limit int) ([]entity.LoginLog, int64, error)
	GetActivityLogs(userID *uint, method string, page, limit int) ([]entity.ActivityLog, int64, error)
	DeleteOldLoginLogs(days int) (int64, error)
	DeleteOldActivityLogs(days int) (int64, error)
}

type logUsecase struct {
	logRepo repository.LogRepository
}

func NewLogUsecase(logRepo repository.LogRepository) LogUsecase {
	return &logUsecase{logRepo: logRepo}
}

func (u *logUsecase) GetLoginLogs(userID *uint, success *bool, page, limit int) ([]entity.LoginLog, int64, error) {
	return u.logRepo.GetLoginLogs(userID, success, page, limit)
}

func (u *logUsecase) GetActivityLogs(userID *uint, method string, page, limit int) ([]entity.ActivityLog, int64, error) {
	return u.logRepo.GetActivityLogs(userID, method, page, limit)
}

func (u *logUsecase) DeleteOldLoginLogs(days int) (int64, error) {
	return u.logRepo.DeleteLoginLogsBefore(days)
}

func (u *logUsecase) DeleteOldActivityLogs(days int) (int64, error) {
	return u.logRepo.DeleteActivityLogsBefore(days)
}
