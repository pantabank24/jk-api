package controller

import (
	"strconv"

	"jk-api/internal/module/store/usecase"
	"jk-api/pkg/response"
	"jk-api/pkg/upload"

	"github.com/gofiber/fiber/v2"
)

type StoreController struct {
	storeUsecase usecase.StoreUsecase
}

func NewStoreController(storeUsecase usecase.StoreUsecase) *StoreController {
	return &StoreController{storeUsecase: storeUsecase}
}

func (ctrl *StoreController) CreateStore(c *fiber.Ctx) error {
	var req usecase.CreateStoreRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}
	if req.Name == "" {
		return response.BadRequest(c, "Store name is required")
	}

	store, err := ctrl.storeUsecase.CreateStore(&req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.Created(c, "Store created successfully", store)
}

func (ctrl *StoreController) GetAllStores(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "10"))

	stores, total, err := ctrl.storeUsecase.GetAllStores(page, limit)
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}
	return response.Paginated(c, "Stores retrieved", stores, page, limit, total)
}

func (ctrl *StoreController) GetStoreByID(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid store ID")
	}

	store, err := ctrl.storeUsecase.GetStoreByID(uint(id))
	if err != nil {
		return response.NotFound(c, "Store not found")
	}
	return response.Success(c, "Store retrieved", store)
}

func (ctrl *StoreController) UpdateStore(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid store ID")
	}

	var req usecase.UpdateStoreRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	store, err := ctrl.storeUsecase.UpdateStore(uint(id), &req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.Success(c, "Store updated", store)
}

func (ctrl *StoreController) DeleteStore(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid store ID")
	}

	if err := ctrl.storeUsecase.DeleteStore(uint(id)); err != nil {
		return response.NotFound(c, err.Error())
	}
	return response.Success(c, "Store deleted", nil)
}

func (ctrl *StoreController) UploadLogo(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid store ID")
	}

	path, err := upload.SaveFile(c, "logo", "stores")
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	store, err := ctrl.storeUsecase.UpdateLogo(uint(id), path)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.Success(c, "Logo uploaded", store)
}
