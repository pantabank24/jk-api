package repository

import (
	"errors"

	"jk-api/internal/entity"

	"gorm.io/gorm"
)

// CustomerRepository manages customer accounts. A customer is a User whose role
// is "customer"; every list/lookup here is scoped to that role.
type CustomerRepository interface {
	GetCustomerRoleID() (uint, error)
	Create(user *entity.User) error
	FindAll(page, limit int, storeID, branchID *uint, search string) ([]entity.User, int64, error)
	FindByID(id uint) (*entity.User, error)
	Update(user *entity.User) error
	Delete(id uint) error
	ExistsByEmail(email string) bool

	// Documents uploaded for a customer.
	CreateDocument(doc *entity.CustomerDocument) error
	FindDocuments(userID uint) ([]entity.CustomerDocument, error)
	FindDocumentByID(id uint) (*entity.CustomerDocument, error)
	DeleteDocument(id uint) error
}

type customerRepository struct {
	db *gorm.DB
}

func NewCustomerRepository(db *gorm.DB) CustomerRepository {
	return &customerRepository{db: db}
}

func (r *customerRepository) GetCustomerRoleID() (uint, error) {
	var role entity.Role
	if err := r.db.Where("name = ?", "customer").First(&role).Error; err != nil {
		return 0, errors.New("customer role not found")
	}
	return role.ID, nil
}

func (r *customerRepository) Create(user *entity.User) error {
	return r.db.Create(user).Error
}

func (r *customerRepository) FindAll(page, limit int, storeID, branchID *uint, search string) ([]entity.User, int64, error) {
	var users []entity.User
	var total int64

	roleID, err := r.GetCustomerRoleID()
	if err != nil {
		return nil, 0, err
	}

	query := r.db.Model(&entity.User{}).Where("role_id = ?", roleID)
	if storeID != nil {
		query = query.Where("store_id = ?", *storeID)
	}
	if branchID != nil {
		query = query.Where("branch_id = ?", *branchID)
	}
	if search != "" {
		query = query.Where("name ILIKE ? OR email ILIKE ?", "%"+search+"%", "%"+search+"%")
	}

	query.Count(&total)
	offset := (page - 1) * limit
	err = query.Preload("Role").Preload("Store").Preload("Branch").
		Offset(offset).Limit(limit).Order("id DESC").Find(&users).Error
	return users, total, err
}

func (r *customerRepository) FindByID(id uint) (*entity.User, error) {
	roleID, err := r.GetCustomerRoleID()
	if err != nil {
		return nil, err
	}
	var user entity.User
	err = r.db.Preload("Role").Preload("Store").Preload("Branch").
		Where("role_id = ?", roleID).First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *customerRepository) Update(user *entity.User) error {
	return r.db.Save(user).Error
}

func (r *customerRepository) Delete(id uint) error {
	roleID, err := r.GetCustomerRoleID()
	if err != nil {
		return err
	}
	return r.db.Where("role_id = ?", roleID).Delete(&entity.User{}, id).Error
}

func (r *customerRepository) ExistsByEmail(email string) bool {
	var count int64
	r.db.Model(&entity.User{}).Where("email = ?", email).Count(&count)
	return count > 0
}

func (r *customerRepository) CreateDocument(doc *entity.CustomerDocument) error {
	return r.db.Create(doc).Error
}

func (r *customerRepository) FindDocuments(userID uint) ([]entity.CustomerDocument, error) {
	var docs []entity.CustomerDocument
	err := r.db.Where("user_id = ?", userID).Order("id DESC").Find(&docs).Error
	return docs, err
}

func (r *customerRepository) FindDocumentByID(id uint) (*entity.CustomerDocument, error) {
	var doc entity.CustomerDocument
	if err := r.db.First(&doc, id).Error; err != nil {
		return nil, err
	}
	return &doc, nil
}

func (r *customerRepository) DeleteDocument(id uint) error {
	return r.db.Delete(&entity.CustomerDocument{}, id).Error
}
