package usecase

import (
	"errors"

	"jk-api/internal/entity"
	"jk-api/internal/module/store/repository"
)

type StoreUsecase interface {
	CreateStore(req *CreateStoreRequest) (*entity.Store, error)
	GetAllStores(page, limit int) ([]entity.Store, int64, error)
	GetStoreByID(id uint) (*entity.Store, error)
	UpdateStore(id uint, req *UpdateStoreRequest) (*entity.Store, error)
	DeleteStore(id uint) error
	UpdateLogo(id uint, logoPath string) (*entity.Store, error)
}

type CreateStoreRequest struct {
	Name    string `json:"name" validate:"required"`
	Address string `json:"address"`
	Phone   string `json:"phone"`
}

type UpdateStoreRequest struct {
	Name     string `json:"name"`
	Address  string `json:"address"`
	Phone    string `json:"phone"`
	IsActive *bool  `json:"is_active"`
}

type storeUsecase struct {
	storeRepo repository.StoreRepository
}

func NewStoreUsecase(storeRepo repository.StoreRepository) StoreUsecase {
	return &storeUsecase{storeRepo: storeRepo}
}

func (u *storeUsecase) CreateStore(req *CreateStoreRequest) (*entity.Store, error) {
	code, err := u.storeRepo.GenerateCode()
	if err != nil {
		return nil, err
	}

	store := &entity.Store{
		Code:     code,
		Name:     req.Name,
		Address:  req.Address,
		Phone:    req.Phone,
		IsActive: true,
	}

	if err := u.storeRepo.Create(store); err != nil {
		return nil, err
	}
	return store, nil
}

func (u *storeUsecase) GetAllStores(page, limit int) ([]entity.Store, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}
	return u.storeRepo.FindAll(page, limit)
}

func (u *storeUsecase) GetStoreByID(id uint) (*entity.Store, error) {
	return u.storeRepo.FindByID(id)
}

func (u *storeUsecase) UpdateStore(id uint, req *UpdateStoreRequest) (*entity.Store, error) {
	store, err := u.storeRepo.FindByID(id)
	if err != nil {
		return nil, errors.New("store not found")
	}

	if req.Name != "" {
		store.Name = req.Name
	}
	if req.Address != "" {
		store.Address = req.Address
	}
	if req.Phone != "" {
		store.Phone = req.Phone
	}
	if req.IsActive != nil {
		store.IsActive = *req.IsActive
	}

	if err := u.storeRepo.Update(store); err != nil {
		return nil, err
	}
	return store, nil
}

func (u *storeUsecase) DeleteStore(id uint) error {
	_, err := u.storeRepo.FindByID(id)
	if err != nil {
		return errors.New("store not found")
	}
	return u.storeRepo.Delete(id)
}

func (u *storeUsecase) UpdateLogo(id uint, logoPath string) (*entity.Store, error) {
	store, err := u.storeRepo.FindByID(id)
	if err != nil {
		return nil, errors.New("store not found")
	}
	store.Logo = logoPath
	if err := u.storeRepo.Update(store); err != nil {
		return nil, err
	}
	return store, nil
}
