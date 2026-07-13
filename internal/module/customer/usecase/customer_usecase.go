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
	UpdateAvatar(id uint, avatar string) (*entity.User, error)
	DeleteCustomer(id uint) error

	AddDocument(doc *entity.CustomerDocument) error
	GetDocuments(userID uint) ([]entity.CustomerDocument, error)
	GetDocumentByID(id uint) (*entity.CustomerDocument, error)
	DeleteDocument(id uint) error
}

type CreateCustomerRequest struct {
	Name            string `json:"name" validate:"required"`
	Email           string `json:"email" validate:"required,email"`
	Password        string `json:"password" validate:"required,min=6"`
	Phone           string `json:"phone"`
	StoreName       string `json:"store_name"`
	Address         string `json:"address"`
	TaxID           string `json:"tax_id"`
	BankID          *uint  `json:"bank_id"`
	BankAccountNo   string `json:"bank_account_no"`
	BankAccountName string `json:"bank_account_name"`
}

type UpdateCustomerRequest struct {
	Name      string  `json:"name"`
	Email     string  `json:"email" validate:"omitempty,email"`
	Password  string  `json:"password"`
	Phone     string  `json:"phone"`
	StoreName *string `json:"store_name"`
	Address   *string `json:"address"`
	TaxID     *string `json:"tax_id"`
	// Pointers so clearing a bank (bank_id: null) is distinguishable from "not sent".
	BankID          *uint   `json:"bank_id"`
	BankAccountNo   *string `json:"bank_account_no"`
	BankAccountName *string `json:"bank_account_name"`
	IsActive        *bool   `json:"is_active"`
}

// normalizeBankID maps "no bank" to a NULL bank_id. The form always sends a value,
// using 0 for "ไม่ระบุ" (an empty <Select> yields no id), so 0 must not be written
// as a real foreign key.
func normalizeBankID(id *uint) *uint {
	if id == nil || *id == 0 {
		return nil
	}
	return id
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

	roleID, err := u.customerRepo.GetCustomerRoleID()
	if err != nil {
		return nil, err
	}

	hashed, err := jwtPkg.HashPassword(req.Password)
	if err != nil {
		return nil, errors.New("failed to hash password")
	}

	user := &entity.User{
		Name:            req.Name,
		Email:           req.Email,
		Password:        hashed,
		Phone:           req.Phone,
		StoreName:       req.StoreName,
		Address:         req.Address,
		TaxID:           req.TaxID,
		BankID:          normalizeBankID(req.BankID),
		BankAccountNo:   req.BankAccountNo,
		BankAccountName: req.BankAccountName,
		RoleID:          &roleID,
		IsActive:        true,
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
	if req.StoreName != nil {
		user.StoreName = *req.StoreName
	}
	if req.Address != nil {
		user.Address = *req.Address
	}
	if req.TaxID != nil {
		user.TaxID = *req.TaxID
	}
	if req.BankID != nil {
		user.BankID = normalizeBankID(req.BankID) // 0 = ไม่ระบุ → clear it
	}
	if req.BankAccountNo != nil {
		user.BankAccountNo = *req.BankAccountNo
	}
	if req.BankAccountName != nil {
		user.BankAccountName = *req.BankAccountName
	}
	if req.IsActive != nil {
		user.IsActive = *req.IsActive
	}

	if err := u.customerRepo.Update(user); err != nil {
		return nil, err
	}
	return u.customerRepo.FindByID(id)
}

func (u *customerUsecase) UpdateAvatar(id uint, avatar string) (*entity.User, error) {
	user, err := u.customerRepo.FindByID(id)
	if err != nil {
		return nil, errors.New("customer not found")
	}
	user.Avatar = avatar
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

func (u *customerUsecase) AddDocument(doc *entity.CustomerDocument) error {
	return u.customerRepo.CreateDocument(doc)
}

func (u *customerUsecase) GetDocuments(userID uint) ([]entity.CustomerDocument, error) {
	return u.customerRepo.FindDocuments(userID)
}

func (u *customerUsecase) GetDocumentByID(id uint) (*entity.CustomerDocument, error) {
	return u.customerRepo.FindDocumentByID(id)
}

func (u *customerUsecase) DeleteDocument(id uint) error {
	return u.customerRepo.DeleteDocument(id)
}
