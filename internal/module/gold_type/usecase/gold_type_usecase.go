package usecase

import (
	"errors"

	"jk-api/internal/entity"
	"jk-api/internal/module/gold_type/repository"
)

type GoldTypeUsecase interface {
	GetAll() ([]entity.GoldType, error)
	GetByID(id uint) (*entity.GoldType, error)
	Create(req *GoldTypeRequest) (*entity.GoldType, error)
	Update(id uint, req *GoldTypeRequest) (*entity.GoldType, error)
	Delete(id uint) error
}

type GoldTypeRequest struct {
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	PriceSource    string   `json:"price_source"`
	DefaultPercent float64  `json:"default_percent"`
	DefaultPlus    float64  `json:"default_plus"`
	FormulaSteps   string   `json:"formula_steps"` // JSON-encoded []entity.FormulaStep
	ServiceRate    float64  `json:"service_rate"`
	PlusType       int      `json:"plus_type"` // 0=บาท, 1=%
	SortOrder      int      `json:"sort_order"`
	IsActive       *bool    `json:"is_active"`
}

type goldTypeUsecase struct {
	repo repository.GoldTypeRepository
}

func NewGoldTypeUsecase(repo repository.GoldTypeRepository) GoldTypeUsecase {
	return &goldTypeUsecase{repo: repo}
}

func (u *goldTypeUsecase) GetAll() ([]entity.GoldType, error) {
	return u.repo.FindAll()
}

func (u *goldTypeUsecase) GetByID(id uint) (*entity.GoldType, error) {
	return u.repo.FindByID(id)
}

func (u *goldTypeUsecase) Create(req *GoldTypeRequest) (*entity.GoldType, error) {
	if req.Name == "" {
		return nil, errors.New("กรุณาระบุชื่อประเภท")
	}
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}
	steps := req.FormulaSteps
	if steps == "" {
		steps = "[]"
	}
	gt := &entity.GoldType{
		Name:           req.Name,
		Description:    req.Description,
		PriceSource:    req.PriceSource,
		DefaultPercent: req.DefaultPercent,
		DefaultPlus:    req.DefaultPlus,
		FormulaSteps:   steps,
		ServiceRate:    req.ServiceRate,
		PlusType:       req.PlusType,
		SortOrder:      req.SortOrder,
		IsActive:       isActive,
	}
	if err := u.repo.Create(gt); err != nil {
		return nil, err
	}
	return gt, nil
}

func (u *goldTypeUsecase) Update(id uint, req *GoldTypeRequest) (*entity.GoldType, error) {
	gt, err := u.repo.FindByID(id)
	if err != nil {
		return nil, errors.New("ไม่พบประเภททอง")
	}
	if req.Name != "" {
		gt.Name = req.Name
	}
	gt.Description = req.Description
	if req.PriceSource != "" {
		gt.PriceSource = req.PriceSource
	}
	gt.DefaultPercent = req.DefaultPercent
	gt.DefaultPlus = req.DefaultPlus
	if req.FormulaSteps != "" {
		gt.FormulaSteps = req.FormulaSteps
	}
	if req.ServiceRate != 0 {
		gt.ServiceRate = req.ServiceRate
	}
	gt.PlusType = req.PlusType
	gt.SortOrder = req.SortOrder
	if req.IsActive != nil {
		gt.IsActive = *req.IsActive
	}
	if err := u.repo.Update(gt); err != nil {
		return nil, err
	}
	return gt, nil
}

func (u *goldTypeUsecase) Delete(id uint) error {
	_, err := u.repo.FindByID(id)
	if err != nil {
		return errors.New("ไม่พบประเภททอง")
	}
	return u.repo.Delete(id)
}
