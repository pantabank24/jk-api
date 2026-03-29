package usecase

import (
	"errors"

	"jk-api/internal/entity"
	roleRepo "jk-api/internal/module/role/repository"
	"jk-api/internal/module/user/repository"
	jwtPkg "jk-api/pkg/jwt"
)

type UserUsecase interface {
	CreateUser(req *CreateUserRequest) (*entity.User, error)
	GetAllUsers(page, limit int) ([]entity.User, int64, error)
	GetUserByID(id uint) (*entity.User, error)
	UpdateUser(id uint, req *UpdateUserRequest) (*entity.User, error)
	DeleteUser(id uint) error
}

type CreateUserRequest struct {
	Name     string `json:"name" validate:"required"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=6"`
	Phone    string `json:"phone"`
	RoleID   *uint  `json:"role_id"`
	StoreID  *uint  `json:"store_id"`
	BranchID *uint  `json:"branch_id"`
}

type UpdateUserRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email" validate:"omitempty,email"`
	Phone    string `json:"phone"`
	RoleID   *uint  `json:"role_id"`
	StoreID  *uint  `json:"store_id"`
	BranchID *uint  `json:"branch_id"`
	IsActive *bool  `json:"is_active"`
}

type userUsecase struct {
	userRepo repository.UserRepository
	roleRepo roleRepo.RoleRepository
}

func NewUserUsecase(userRepo repository.UserRepository, roleRepo roleRepo.RoleRepository) UserUsecase {
	return &userUsecase{userRepo: userRepo, roleRepo: roleRepo}
}

// requiresBranch returns true if the role name is employee or branch level
func (u *userUsecase) requiresBranch(roleID *uint) (bool, error) {
	if roleID == nil {
		return false, nil
	}
	role, err := u.roleRepo.FindByID(*roleID)
	if err != nil {
		return false, nil
	}
	return role.Name == "employee" || role.Name == "branch", nil
}

func (u *userUsecase) CreateUser(req *CreateUserRequest) (*entity.User, error) {
	existing, _ := u.userRepo.FindByEmail(req.Email)
	if existing != nil {
		return nil, errors.New("email already exists")
	}

	// Validate: employee/branch role requires branch_id
	needsBranch, _ := u.requiresBranch(req.RoleID)
	if needsBranch && req.BranchID == nil {
		return nil, errors.New("พนักงานระดับสาขาจำเป็นต้องระบุสาขา")
	}

	hashedPassword, err := jwtPkg.HashPassword(req.Password)
	if err != nil {
		return nil, errors.New("failed to hash password")
	}

	user := &entity.User{
		Name:     req.Name,
		Email:    req.Email,
		Password: hashedPassword,
		Phone:    req.Phone,
		RoleID:   req.RoleID,
		StoreID:  req.StoreID,
		BranchID: req.BranchID,
		IsActive: true,
	}

	if err := u.userRepo.Create(user); err != nil {
		return nil, err
	}
	return user, nil
}

func (u *userUsecase) GetAllUsers(page, limit int) ([]entity.User, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}
	return u.userRepo.FindAll(page, limit)
}

func (u *userUsecase) GetUserByID(id uint) (*entity.User, error) {
	return u.userRepo.FindByID(id)
}

func (u *userUsecase) UpdateUser(id uint, req *UpdateUserRequest) (*entity.User, error) {
	user, err := u.userRepo.FindByID(id)
	if err != nil {
		return nil, errors.New("user not found")
	}

	// Determine the effective role (incoming or current)
	effectiveRoleID := user.RoleID
	if req.RoleID != nil {
		effectiveRoleID = req.RoleID
	}
	effectiveBranchID := user.BranchID
	if req.BranchID != nil {
		effectiveBranchID = req.BranchID
	}

	needsBranch, _ := u.requiresBranch(effectiveRoleID)
	if needsBranch && effectiveBranchID == nil {
		return nil, errors.New("พนักงานระดับสาขาจำเป็นต้องระบุสาขา")
	}

	if req.Name != "" {
		user.Name = req.Name
	}
	if req.Email != "" {
		existing, _ := u.userRepo.FindByEmail(req.Email)
		if existing != nil && existing.ID != user.ID {
			return nil, errors.New("email already exists")
		}
		user.Email = req.Email
	}
	if req.Phone != "" {
		user.Phone = req.Phone
	}
	if req.RoleID != nil {
		user.RoleID = req.RoleID
	}
	if req.StoreID != nil {
		user.StoreID = req.StoreID
	}
	if req.BranchID != nil {
		user.BranchID = req.BranchID
	}
	if req.IsActive != nil {
		user.IsActive = *req.IsActive
	}

	if err := u.userRepo.Update(user); err != nil {
		return nil, err
	}
	return user, nil
}

func (u *userUsecase) DeleteUser(id uint) error {
	_, err := u.userRepo.FindByID(id)
	if err != nil {
		return errors.New("user not found")
	}
	return u.userRepo.Delete(id)
}
