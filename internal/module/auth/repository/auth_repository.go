package repository

import (
	"jk-api/internal/entity"

	"gorm.io/gorm"
)

type AuthRepository interface {
	FindByEmail(email string) (*entity.User, error)
	FindByIDWithRole(id uint) (*entity.User, error)
	GetPermissionsByRoleID(roleID uint) ([]string, error)
	GetMemberCreditsByUserID(userID uint) (float64, bool)
	EmailExistsForOtherUser(email string, excludeID uint) (bool, error)
	UpdateProfile(userID uint, fields map[string]interface{}) error
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

// GetMemberCreditsByUserID returns the credit balance of the member profile
// linked to the user, and whether such a profile exists.
func (r *authRepository) GetMemberCreditsByUserID(userID uint) (float64, bool) {
	var member entity.Member
	err := r.db.Select("credits").Where("user_id = ?", userID).First(&member).Error
	if err != nil {
		return 0, false
	}
	return member.Credits, true
}

// EmailExistsForOtherUser reports whether the email is already used by a
// user other than excludeID (used to guard self-service profile updates).
func (r *authRepository) EmailExistsForOtherUser(email string, excludeID uint) (bool, error) {
	var count int64
	err := r.db.Model(&entity.User{}).
		Where("email = ? AND id <> ?", email, excludeID).
		Count(&count).Error
	return count > 0, err
}

// UpdateProfile applies a partial column update to the user, leaving
// association / FK columns untouched.
func (r *authRepository) UpdateProfile(userID uint, fields map[string]interface{}) error {
	return r.db.Model(&entity.User{}).
		Where("id = ?", userID).
		Updates(fields).Error
}

func (r *authRepository) GetPermissionsByRoleID(roleID uint) ([]string, error) {
	var permissions []string
	err := r.db.Model(&entity.RolePermission{}).
		Joins("JOIN permissions ON permissions.id = role_permissions.permission_id").
		Where("role_permissions.role_id = ?", roleID).
		Pluck("permissions.code", &permissions).Error
	return permissions, err
}
