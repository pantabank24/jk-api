package usecase

import (
	"errors"
	"fmt"

	"jk-api/internal/entity"
	"jk-api/internal/module/document_type/repository"
)

type DocumentTypeUsecase interface {
	GetAll() ([]entity.DocumentType, error)
	GetByID(id uint) (*entity.DocumentType, error)
	Create(req *DocumentTypeRequest) (*entity.DocumentType, error)
	Update(id uint, req *DocumentTypeRequest) (*entity.DocumentType, error)
	Delete(id uint) error
}

type DocumentTypeRequest struct {
	Name      string `json:"name"`
	Code      string `json:"code"`
	SortOrder int    `json:"sort_order"`
	IsActive  *bool  `json:"is_active"`
}

type documentTypeUsecase struct {
	repo repository.DocumentTypeRepository
}

func NewDocumentTypeUsecase(repo repository.DocumentTypeRepository) DocumentTypeUsecase {
	return &documentTypeUsecase{repo: repo}
}

func (u *documentTypeUsecase) GetAll() ([]entity.DocumentType, error) {
	return u.repo.FindAll()
}

func (u *documentTypeUsecase) GetByID(id uint) (*entity.DocumentType, error) {
	return u.repo.FindByID(id)
}

func (u *documentTypeUsecase) Create(req *DocumentTypeRequest) (*entity.DocumentType, error) {
	if req.Name == "" {
		return nil, errors.New("กรุณาระบุชื่อประเภทเอกสาร")
	}
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}
	t := &entity.DocumentType{
		Name:      req.Name,
		Code:      req.Code,
		SortOrder: req.SortOrder,
		IsActive:  isActive,
	}
	if err := u.repo.Create(t); err != nil {
		return nil, err
	}
	return t, nil
}

func (u *documentTypeUsecase) Update(id uint, req *DocumentTypeRequest) (*entity.DocumentType, error) {
	t, err := u.repo.FindByID(id)
	if err != nil {
		return nil, errors.New("ไม่พบประเภทเอกสาร")
	}
	if req.Name != "" {
		t.Name = req.Name
	}
	t.Code = req.Code
	t.SortOrder = req.SortOrder
	if req.IsActive != nil {
		t.IsActive = *req.IsActive
	}
	if err := u.repo.Update(t); err != nil {
		return nil, err
	}
	return t, nil
}

func (u *documentTypeUsecase) Delete(id uint) error {
	if _, err := u.repo.FindByID(id); err != nil {
		return errors.New("ไม่พบประเภทเอกสาร")
	}
	// Deleting would null out the type on every document using it (FK ON DELETE SET
	// NULL), silently losing what each file is — refuse and let the shop disable the
	// type instead (is_active = false hides it from the upload selector).
	n, err := u.repo.CountDocuments(id)
	if err != nil {
		return err
	}
	if n > 0 {
		return fmt.Errorf("ลบไม่ได้ มีเอกสาร %d รายการใช้ประเภทนี้อยู่ (ปิดใช้งานแทนได้)", n)
	}
	return u.repo.Delete(id)
}
