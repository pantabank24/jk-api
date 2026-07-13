package usecase

import (
	"errors"
	"fmt"

	"jk-api/internal/entity"
	"jk-api/internal/module/bank/repository"
)

type BankUsecase interface {
	GetAll() ([]entity.Bank, error)
	GetByID(id uint) (*entity.Bank, error)
	Create(req *BankRequest) (*entity.Bank, error)
	Update(id uint, req *BankRequest) (*entity.Bank, error)
	Delete(id uint) error
}

type BankRequest struct {
	Name      string `json:"name"`
	Code      string `json:"code"`
	SortOrder int    `json:"sort_order"`
	IsActive  *bool  `json:"is_active"`
}

type bankUsecase struct {
	repo repository.BankRepository
}

func NewBankUsecase(repo repository.BankRepository) BankUsecase {
	return &bankUsecase{repo: repo}
}

func (u *bankUsecase) GetAll() ([]entity.Bank, error) {
	return u.repo.FindAll()
}

func (u *bankUsecase) GetByID(id uint) (*entity.Bank, error) {
	return u.repo.FindByID(id)
}

func (u *bankUsecase) Create(req *BankRequest) (*entity.Bank, error) {
	if req.Name == "" {
		return nil, errors.New("กรุณาระบุชื่อธนาคาร")
	}
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}
	b := &entity.Bank{
		Name:      req.Name,
		Code:      req.Code,
		SortOrder: req.SortOrder,
		IsActive:  isActive,
	}
	if err := u.repo.Create(b); err != nil {
		return nil, err
	}
	return b, nil
}

func (u *bankUsecase) Update(id uint, req *BankRequest) (*entity.Bank, error) {
	b, err := u.repo.FindByID(id)
	if err != nil {
		return nil, errors.New("ไม่พบธนาคาร")
	}
	if req.Name != "" {
		b.Name = req.Name
	}
	b.Code = req.Code
	b.SortOrder = req.SortOrder
	if req.IsActive != nil {
		b.IsActive = *req.IsActive
	}
	if err := u.repo.Update(b); err != nil {
		return nil, err
	}
	return b, nil
}

func (u *bankUsecase) Delete(id uint) error {
	if _, err := u.repo.FindByID(id); err != nil {
		return errors.New("ไม่พบธนาคาร")
	}
	// Deleting would null out the bank on every customer using it (FK ON DELETE SET
	// NULL), silently losing their payout details — refuse and let the shop disable
	// the bank instead (is_active = false hides it from the selector).
	n, err := u.repo.CountUsers(id)
	if err != nil {
		return err
	}
	if n > 0 {
		return fmt.Errorf("ลบไม่ได้ มีลูกค้า %d รายใช้ธนาคารนี้อยู่ (ปิดใช้งานแทนได้)", n)
	}
	return u.repo.Delete(id)
}
