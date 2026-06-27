package repository

import (
	"jk-api/internal/entity"

	"gorm.io/gorm"
)

type UserFilter struct {
	StoreID  *uint
	BranchID *uint
	RoleID   *uint
	Search   string
	IsActive *bool
}

type UserRepository interface {
	Create(user *entity.User) error
	FindAll(page, limit int, f UserFilter) ([]entity.User, int64, error)
	FindByID(id uint) (*entity.User, error)
	FindByEmail(email string) (*entity.User, error)
	Update(user *entity.User) error
	Delete(id uint) error
	ExistsByEmail(email string) bool
	HasOwnerForStore(storeID uint) bool
	UpdateAvatar(id uint, path string) (*entity.User, error)
}

type userRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) Create(user *entity.User) error {
	return r.db.Create(user).Error
}

func (r *userRepository) FindAll(page, limit int, f UserFilter) ([]entity.User, int64, error) {
	var users []entity.User
	var total int64

	query := r.db.Model(&entity.User{})
	if f.StoreID != nil {
		query = query.Where("store_id = ?", *f.StoreID)
	}
	if f.BranchID != nil {
		query = query.Where("branch_id = ?", *f.BranchID)
	}
	if f.RoleID != nil {
		query = query.Where("role_id = ?", *f.RoleID)
	}
	if f.Search != "" {
		query = query.Where("name ILIKE ? OR email ILIKE ?", "%"+f.Search+"%", "%"+f.Search+"%")
	}
	if f.IsActive != nil {
		query = query.Where("is_active = ?", *f.IsActive)
	}

	query.Count(&total)
	offset := (page - 1) * limit
	err := query.Preload("Role").Preload("Store").Preload("Branch").
		Offset(offset).Limit(limit).Order("id DESC").Find(&users).Error
	return users, total, err
}

func (r *userRepository) FindByID(id uint) (*entity.User, error) {
	var user entity.User
	err := r.db.Preload("Role").Preload("Store").Preload("Branch").First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) FindByEmail(email string) (*entity.User, error) {
	var user entity.User
	err := r.db.Where("email = ?", email).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) Update(user *entity.User) error {
	return r.db.Save(user).Error
}

func (r *userRepository) Delete(id uint) error {
	return r.db.Delete(&entity.User{}, id).Error
}

func (r *userRepository) ExistsByEmail(email string) bool {
	var count int64
	r.db.Model(&entity.User{}).Where("email = ?", email).Count(&count)
	return count > 0
}

func (r *userRepository) UpdateAvatar(id uint, path string) (*entity.User, error) {
	if err := r.db.Model(&entity.User{}).Where("id = ?", id).Update("avatar", path).Error; err != nil {
		return nil, err
	}
	return r.FindByID(id)
}

func (r *userRepository) HasOwnerForStore(storeID uint) bool {
	var count int64
	r.db.Model(&entity.User{}).
		Joins("JOIN roles ON roles.id = users.role_id").
		Where("users.store_id = ? AND roles.name = ?", storeID, "owner").
		Count(&count)
	return count > 0
}
