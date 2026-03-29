package repository

import (
	"jk-api/internal/entity"

	"gorm.io/gorm"
)

type RoleRepository interface {
	FindAll() ([]entity.Role, error)
	FindByID(id uint) (*entity.Role, error)
	Create(role *entity.Role) error
	Update(role *entity.Role) error
	Delete(id uint) error
	GetAllPermissions() ([]entity.Permission, error)
	GetPermissionsByRoleID(roleID uint) ([]entity.Permission, error)
	SetRolePermissions(roleID uint, permissionIDs []uint) error
}

type roleRepository struct {
	db *gorm.DB
}

func NewRoleRepository(db *gorm.DB) RoleRepository {
	return &roleRepository{db: db}
}

func (r *roleRepository) FindAll() ([]entity.Role, error) {
	var roles []entity.Role
	err := r.db.Preload("Permissions.Permission").Order("id ASC").Find(&roles).Error
	return roles, err
}

func (r *roleRepository) FindByID(id uint) (*entity.Role, error) {
	var role entity.Role
	err := r.db.Preload("Permissions.Permission").First(&role, id).Error
	if err != nil {
		return nil, err
	}
	return &role, nil
}

func (r *roleRepository) Create(role *entity.Role) error {
	return r.db.Create(role).Error
}

func (r *roleRepository) Update(role *entity.Role) error {
	return r.db.Save(role).Error
}

func (r *roleRepository) Delete(id uint) error {
	// Delete role_permissions first
	r.db.Where("role_id = ?", id).Delete(&entity.RolePermission{})
	return r.db.Delete(&entity.Role{}, id).Error
}

func (r *roleRepository) GetAllPermissions() ([]entity.Permission, error) {
	var permissions []entity.Permission
	err := r.db.Order("group_name ASC, id ASC").Find(&permissions).Error
	return permissions, err
}

func (r *roleRepository) GetPermissionsByRoleID(roleID uint) ([]entity.Permission, error) {
	var permissions []entity.Permission
	err := r.db.Joins("JOIN role_permissions ON role_permissions.permission_id = permissions.id").
		Where("role_permissions.role_id = ?", roleID).Find(&permissions).Error
	return permissions, err
}

func (r *roleRepository) SetRolePermissions(roleID uint, permissionIDs []uint) error {
	// Delete existing
	if err := r.db.Where("role_id = ?", roleID).Delete(&entity.RolePermission{}).Error; err != nil {
		return err
	}

	// Insert new
	for _, pid := range permissionIDs {
		rp := entity.RolePermission{RoleID: roleID, PermissionID: pid}
		if err := r.db.Create(&rp).Error; err != nil {
			return err
		}
	}
	return nil
}
