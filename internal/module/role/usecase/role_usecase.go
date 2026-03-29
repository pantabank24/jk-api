package usecase

import (
	"errors"

	"jk-api/internal/entity"
	"jk-api/internal/module/role/repository"
)

type RoleUsecase interface {
	GetAllRoles() ([]entity.Role, error)
	GetRoleByID(id uint) (*entity.Role, error)
	CreateRole(req *CreateRoleRequest) (*entity.Role, error)
	UpdateRole(id uint, req *UpdateRoleRequest) (*entity.Role, error)
	DeleteRole(id uint) error
	GetAllPermissions() ([]entity.Permission, error)
	SetRolePermissions(id uint, req *SetPermissionsRequest) error
}

type CreateRoleRequest struct {
	Name        string `json:"name" validate:"required"`
	DisplayName string `json:"display_name" validate:"required"`
	Description string `json:"description"`
}

type UpdateRoleRequest struct {
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
}

type SetPermissionsRequest struct {
	PermissionIDs []uint `json:"permission_ids" validate:"required"`
}

type roleUsecase struct {
	roleRepo repository.RoleRepository
}

func NewRoleUsecase(roleRepo repository.RoleRepository) RoleUsecase {
	return &roleUsecase{roleRepo: roleRepo}
}

func (u *roleUsecase) GetAllRoles() ([]entity.Role, error) {
	return u.roleRepo.FindAll()
}

func (u *roleUsecase) GetRoleByID(id uint) (*entity.Role, error) {
	return u.roleRepo.FindByID(id)
}

func (u *roleUsecase) CreateRole(req *CreateRoleRequest) (*entity.Role, error) {
	role := &entity.Role{
		Name:        req.Name,
		DisplayName: req.DisplayName,
		Description: req.Description,
		IsSystem:    false,
	}

	if err := u.roleRepo.Create(role); err != nil {
		return nil, err
	}
	return role, nil
}

func (u *roleUsecase) UpdateRole(id uint, req *UpdateRoleRequest) (*entity.Role, error) {
	role, err := u.roleRepo.FindByID(id)
	if err != nil {
		return nil, errors.New("role not found")
	}

	if req.DisplayName != "" {
		role.DisplayName = req.DisplayName
	}
	if req.Description != "" {
		role.Description = req.Description
	}

	if err := u.roleRepo.Update(role); err != nil {
		return nil, err
	}
	return role, nil
}

func (u *roleUsecase) DeleteRole(id uint) error {
	role, err := u.roleRepo.FindByID(id)
	if err != nil {
		return errors.New("role not found")
	}
	if role.IsSystem {
		return errors.New("cannot delete system role")
	}
	return u.roleRepo.Delete(id)
}

func (u *roleUsecase) GetAllPermissions() ([]entity.Permission, error) {
	return u.roleRepo.GetAllPermissions()
}

func (u *roleUsecase) SetRolePermissions(id uint, req *SetPermissionsRequest) error {
	_, err := u.roleRepo.FindByID(id)
	if err != nil {
		return errors.New("role not found")
	}
	return u.roleRepo.SetRolePermissions(id, req.PermissionIDs)
}
