package usecase

import (
	"errors"

	"jk-api/internal/entity"
	"jk-api/internal/module/branch/repository"
)

type BranchUsecase interface {
	CreateBranch(storeID uint, req *CreateBranchRequest) (*entity.Branch, error)
	GetAllBranches(storeID uint, page, limit int) ([]entity.Branch, int64, error)
	GetBranchByID(id uint) (*entity.Branch, error)
	UpdateBranch(id uint, req *UpdateBranchRequest) (*entity.Branch, error)
	DeleteBranch(id uint) error
}

type CreateBranchRequest struct {
	Name    string `json:"name" validate:"required"`
	Address string `json:"address"`
	Phone   string `json:"phone"`
}

type UpdateBranchRequest struct {
	Name     string `json:"name"`
	Address  string `json:"address"`
	Phone    string `json:"phone"`
	IsActive *bool  `json:"is_active"`
}

type branchUsecase struct {
	branchRepo repository.BranchRepository
}

func NewBranchUsecase(branchRepo repository.BranchRepository) BranchUsecase {
	return &branchUsecase{branchRepo: branchRepo}
}

func (u *branchUsecase) CreateBranch(storeID uint, req *CreateBranchRequest) (*entity.Branch, error) {
	code, err := u.branchRepo.GenerateCode()
	if err != nil {
		return nil, err
	}

	branch := &entity.Branch{
		StoreID:  storeID,
		Code:     code,
		Name:     req.Name,
		Address:  req.Address,
		Phone:    req.Phone,
		IsActive: true,
	}

	if err := u.branchRepo.Create(branch); err != nil {
		return nil, err
	}
	return branch, nil
}

func (u *branchUsecase) GetAllBranches(storeID uint, page, limit int) ([]entity.Branch, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}
	return u.branchRepo.FindAllByStoreID(storeID, page, limit)
}

func (u *branchUsecase) GetBranchByID(id uint) (*entity.Branch, error) {
	return u.branchRepo.FindByID(id)
}

func (u *branchUsecase) UpdateBranch(id uint, req *UpdateBranchRequest) (*entity.Branch, error) {
	branch, err := u.branchRepo.FindByID(id)
	if err != nil {
		return nil, errors.New("branch not found")
	}

	if req.Name != "" {
		branch.Name = req.Name
	}
	if req.Address != "" {
		branch.Address = req.Address
	}
	if req.Phone != "" {
		branch.Phone = req.Phone
	}
	if req.IsActive != nil {
		branch.IsActive = *req.IsActive
	}

	if err := u.branchRepo.Update(branch); err != nil {
		return nil, err
	}
	return branch, nil
}

func (u *branchUsecase) DeleteBranch(id uint) error {
	_, err := u.branchRepo.FindByID(id)
	if err != nil {
		return errors.New("branch not found")
	}
	return u.branchRepo.Delete(id)
}
