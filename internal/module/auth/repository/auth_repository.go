package repository

import (
	"jk-api/internal/entity"

	"gorm.io/gorm"
)

type AuthRepository interface {
	FindByEmail(email string) (*entity.User, error)
	FindByIDWithRole(id uint) (*entity.User, error)
	GetPermissionsByRoleID(roleID uint) ([]string, error)
}

type authRepository struct {
	db *gorm.DB
}

func NewAuthRepository(db *gorm.DB) AuthRepository {
	return &authRepository{db: db}
}

func (r *authRepository) FindByEmail(email string) (*entity.User, error) {
	var user entity.User
	err := r.db.Preload("Role").Preload("Store").Preload("Branch").
		Where("email = ?", email).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *authRepository) FindByIDWithRole(id uint) (*entity.User, error) {
	var user entity.User
	err := r.db.Preload("Role").Preload("Store").Preload("Branch").
		First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *authRepository) GetPermissionsByRoleID(roleID uint) ([]string, error) {
	var permissions []string
	err := r.db.Model(&entity.RolePermission{}).
		Joins("JOIN permissions ON permissions.id = role_permissions.permission_id").
		Where("role_permissions.role_id = ?", roleID).
		Pluck("permissions.code", &permissions).Error
	return permissions, err
}
