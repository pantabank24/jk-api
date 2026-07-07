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
	UpdateLogo(id uint, logoPath string) (*entity.Branch, error)
}

type CreateBranchRequest struct {
	Name       string `json:"name" validate:"required"`
	Address    string `json:"address"`
	Phone      string `json:"phone"`
	HeaderName string `json:"header_name"`
	TaxID      string `json:"tax_id"`
	TaxName    string `json:"tax_name"`
	Website    string `json:"website"`
	IsMain     bool   `json:"is_main"`
}

type UpdateBranchRequest struct {
	Name       string  `json:"name"`
	Address    *string `json:"address"`
	Phone      *string `json:"phone"`
	HeaderName *string `json:"header_name"`
	TaxID      *string `json:"tax_id"`
	TaxName    *string `json:"tax_name"`
	Website    *string `json:"website"`
	IsMain     *bool   `json:"is_main"`
	IsActive   *bool   `json:"is_active"`
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

	// The first branch of a store is always the main one (there must be exactly
	// one); later branches are main only when explicitly requested.
	existing, err := u.branchRepo.CountByStoreID(storeID)
	if err != nil {
		return nil, err
	}
	isMain := req.IsMain || existing == 0

	branch := &entity.Branch{
		StoreID:    storeID,
		Code:       code,
		Name:       req.Name,
		Address:    req.Address,
		Phone:      req.Phone,
		HeaderName: req.HeaderName,
		TaxID:      req.TaxID,
		TaxName:    req.TaxName,
		Website:    req.Website,
		IsMain:     isMain,
		IsActive:   true,
	}

	if err := u.branchRepo.Create(branch); err != nil {
		return nil, err
	}
	// Enforce a single main branch when this one claims it.
	if isMain {
		if err := u.branchRepo.SetMain(storeID, branch.ID); err != nil {
			return nil, err
		}
		branch.IsMain = true
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
	if req.Address != nil {
		branch.Address = *req.Address
	}
	if req.Phone != nil {
		branch.Phone = *req.Phone
	}
	if req.HeaderName != nil {
		branch.HeaderName = *req.HeaderName
	}
	if req.TaxID != nil {
		branch.TaxID = *req.TaxID
	}
	if req.TaxName != nil {
		branch.TaxName = *req.TaxName
	}
	if req.Website != nil {
		branch.Website = *req.Website
	}
	if req.IsActive != nil {
		branch.IsActive = *req.IsActive
	}
	// Setting this branch as main is handled separately so exactly one branch of
	// the store stays main. Unsetting is ignored — pick another branch as main
	// instead of leaving the store with none.
	makeMain := req.IsMain != nil && *req.IsMain && !branch.IsMain

	if err := u.branchRepo.Update(branch); err != nil {
		return nil, err
	}
	if makeMain {
		if err := u.branchRepo.SetMain(branch.StoreID, branch.ID); err != nil {
			return nil, err
		}
		branch.IsMain = true
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

func (u *branchUsecase) UpdateLogo(id uint, logoPath string) (*entity.Branch, error) {
	branch, err := u.branchRepo.FindByID(id)
	if err != nil {
		return nil, errors.New("branch not found")
	}
	branch.Logo = logoPath
	if err := u.branchRepo.Update(branch); err != nil {
		return nil, err
	}
	return branch, nil
}
