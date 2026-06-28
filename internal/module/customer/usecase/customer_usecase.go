package usecase

import (
	"errors"

	"jk-api/internal/entity"
	"jk-api/internal/module/customer/repository"
	jwtPkg "jk-api/pkg/jwt"
)

type CustomerUsecase interface {
	CreateCustomer(req *CreateCustomerRequest) (*entity.User, error)
	GetAllCustomers(page, limit int, storeID, branchID *uint, search string) ([]entity.User, int64, error)
	GetCustomerByID(id uint) (*entity.User, error)
	UpdateCustomer(id uint, req *UpdateCustomerRequest) (*entity.User, error)
	DeleteCustomer(id uint) error
}

type CreateCustomerRequest struct {
	Name     string `json:"name" validate:"required"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=6"`
	Phone    string `json:"phone"`
	StoreID  *uint  `json:"store_id"`
	BranchID *uint  `json:"branch_id"`
}

type UpdateCustomerRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email" validate:"omitempty,email"`
	Password string `json:"password"`
	Phone    string `json:"phone"`
	StoreID  *uint  `json:"store_id"`
	BranchID *uint  `json:"branch_id"`
	IsActive *bool  `json:"is_active"`
}

type customerUsecase struct {
	customerRepo repository.CustomerRepository
}

func NewCustomerUsecase(customerRepo repository.CustomerRepository) CustomerUsecase {
	return &customerUsecase{customerRepo: customerRepo}
}

func (u *customerUsecase) CreateCustomer(req *CreateCustomerRequest) (*entity.User, error) {
	if u.customerRepo.ExistsByEmail(req.Email) {
		return nil, errors.New("email already exists")
	}
	if req.StoreID == nil {
		return nil, errors.New("กรุณาระบุร้าน")
	}
	if req.BranchID == nil {
		return nil, errors.New("กรุณาระบุสาขาของลูกค้า")
	}

	roleID, err := u.customerRepo.GetCustomerRoleID()
	if err != nil {
		return nil, err
	}

	hashed, err := jwtPkg.HashPassword(req.Password)
	if err != nil {
		return nil, errors.New("failed to hash password")
	}

	user := &entity.User{
		Name:     req.Name,
		Email:    req.Email,
		Password: hashed,
		Phone:    req.Phone,
		RoleID:   &roleID,
		StoreID:  req.StoreID,
		BranchID: req.BranchID,
		IsActive: true,
	}
	if err := u.customerRepo.Create(user); err != nil {
		return nil, err
	}
	return u.customerRepo.FindByID(user.ID)
}

func (u *customerUsecase) GetAllCustomers(page, limit int, storeID, branchID *uint, search string) ([]entity.User, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}
	return u.customerRepo.FindAll(page, limit, storeID, branchID, search)
}

func (u *customerUsecase) GetCustomerByID(id uint) (*entity.User, error) {
	return u.customerRepo.FindByID(id)
}

func (u *customerUsecase) UpdateCustomer(id uint, req *UpdateCustomerRequest) (*entity.User, error) {
	user, err := u.customerRepo.FindByID(id)
	if err != nil {
		return nil, errors.New("customer not found")
	}

	if req.Name != "" {
		user.Name = req.Name
	}
	if req.Email != "" && req.Email != user.Email {
		if u.customerRepo.ExistsByEmail(req.Email) {
			return nil, errors.New("email already exists")
		}
		user.Email = req.Email
	}
	if req.Password != "" {
		hashed, err := jwtPkg.HashPassword(req.Password)
		if err != nil {
			return nil, errors.New("failed to hash password")
		}
		user.Password = hashed
	}
	if req.Phone != "" {
		user.Phone = req.Phone
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

	if err := u.customerRepo.Update(user); err != nil {
		return nil, err
	}
	return u.customerRepo.FindByID(id)
}

func (u *customerUsecase) DeleteCustomer(id uint) error {
	if _, err := u.customerRepo.FindByID(id); err != nil {
		return errors.New("customer not found")
	}
	return u.customerRepo.Delete(id)
}
