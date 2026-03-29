package repository

import (
	"fmt"

	"jk-api/internal/entity"

	"gorm.io/gorm"
)

type MemberRepository interface {
	Create(member *entity.Member) error
	FindAll(storeID *uint, branchID *uint, page, limit int, search string) ([]entity.Member, int64, error)
	FindByID(id uint) (*entity.Member, error)
	FindByUserID(userID uint) (*entity.Member, error)
	Update(member *entity.Member) error
	Delete(id uint) error
	GenerateCode() (string, error)
	CreateCreditTransaction(tx *entity.CreditTransaction) error
	GetCreditTransactions(memberID uint, page, limit int) ([]entity.CreditTransaction, int64, error)
	GetAllCreditTransactions(storeID, branchID, memberID *uint, source string, page, limit int, search string) ([]entity.CreditTransaction, int64, error)
}

type memberRepository struct {
	db *gorm.DB
}

func NewMemberRepository(db *gorm.DB) MemberRepository {
	return &memberRepository{db: db}
}

func (r *memberRepository) Create(member *entity.Member) error {
	return r.db.Create(member).Error
}

func (r *memberRepository) FindAll(storeID *uint, branchID *uint, page, limit int, search string) ([]entity.Member, int64, error) {
	var members []entity.Member
	var total int64

	query := r.db.Model(&entity.Member{})
	if storeID != nil {
		query = query.Where("store_id = ?", *storeID)
	}
	if branchID != nil {
		query = query.Where("branch_id = ?", *branchID)
	}
	if search != "" {
		query = query.Where("fname ILIKE ? OR lname ILIKE ? OR phone ILIKE ? OR code ILIKE ?",
			"%"+search+"%", "%"+search+"%", "%"+search+"%", "%"+search+"%")
	}

	query.Count(&total)
	offset := (page - 1) * limit
	err := query.Offset(offset).Limit(limit).Order("id DESC").Find(&members).Error
	return members, total, err
}

func (r *memberRepository) FindByID(id uint) (*entity.Member, error) {
	var member entity.Member
	err := r.db.Preload("Store").Preload("Branch").Preload("User").Preload("User.Role").First(&member, id).Error
	if err != nil {
		return nil, err
	}
	return &member, nil
}

func (r *memberRepository) FindByUserID(userID uint) (*entity.Member, error) {
	var member entity.Member
	err := r.db.Where("user_id = ?", userID).First(&member).Error
	if err != nil {
		return nil, err
	}
	return &member, nil
}

func (r *memberRepository) Update(member *entity.Member) error {
	return r.db.Save(member).Error
}

func (r *memberRepository) Delete(id uint) error {
	return r.db.Delete(&entity.Member{}, id).Error
}

func (r *memberRepository) GenerateCode() (string, error) {
	var count int64
	r.db.Unscoped().Model(&entity.Member{}).Count(&count)
	return fmt.Sprintf("MBR%04d", count+1), nil
}

func (r *memberRepository) CreateCreditTransaction(tx *entity.CreditTransaction) error {
	return r.db.Create(tx).Error
}

func (r *memberRepository) GetCreditTransactions(memberID uint, page, limit int) ([]entity.CreditTransaction, int64, error) {
	var txs []entity.CreditTransaction
	var total int64

	query := r.db.Model(&entity.CreditTransaction{}).Where("member_id = ?", memberID)
	query.Count(&total)

	offset := (page - 1) * limit
	err := query.Offset(offset).Limit(limit).Order("id DESC").Find(&txs).Error
	return txs, total, err
}

func (r *memberRepository) GetAllCreditTransactions(storeID, branchID, memberID *uint, source string, page, limit int, search string) ([]entity.CreditTransaction, int64, error) {
	var txs []entity.CreditTransaction
	var total int64

	query := r.db.Model(&entity.CreditTransaction{}).
		Preload("Member").
		Preload("Creator")

	if storeID != nil {
		query = query.Where("store_id = ?", *storeID)
	}
	if branchID != nil {
		query = query.Where("branch_id = ?", *branchID)
	}
	if memberID != nil {
		query = query.Where("member_id = ?", *memberID)
	}
	switch source {
	case "deposit":
		query = query.Where("action = 0")
	case "withdraw":
		query = query.Where("action = 1 AND description NOT LIKE ?", "%ใบเสนอราคา%")
	case "quotation":
		query = query.Where("action = 1 AND description LIKE ?", "%ใบเสนอราคา%")
	}
	if search != "" {
		query = query.Joins("JOIN members ON members.id = credit_transactions.member_id").
			Where("members.fname ILIKE ? OR members.lname ILIKE ? OR members.code ILIKE ? OR credit_transactions.description ILIKE ?",
				"%"+search+"%", "%"+search+"%", "%"+search+"%", "%"+search+"%")
	}

	query.Count(&total)
	offset := (page - 1) * limit
	err := query.Offset(offset).Limit(limit).Order("credit_transactions.id DESC").Find(&txs).Error
	return txs, total, err
}
