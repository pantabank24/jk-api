package controller

import (
	"strconv"

	"jk-api/internal/module/gold_type/usecase"
	"jk-api/pkg/response"

	"github.com/gofiber/fiber/v2"
)

type GoldTypeController struct {
	uc usecase.GoldTypeUsecase
}

func NewGoldTypeController(uc usecase.GoldTypeUsecase) *GoldTypeController {
	return &GoldTypeController{uc: uc}
}

func (ctrl *GoldTypeController) GetAll(c *fiber.Ctx) error {
	types, err := ctrl.uc.GetAll()
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}
	return response.Success(c, "Gold types retrieved", types)
}

func (ctrl *GoldTypeController) GetByID(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid ID")
	}
	gt, err := ctrl.uc.GetByID(uint(id))
	if err != nil {
		return response.NotFound(c, "ไม่พบประเภททอง")
	}
	return response.Success(c, "Gold type retrieved", gt)
}

func (ctrl *GoldTypeController) Create(c *fiber.Ctx) error {
	var req usecase.GoldTypeRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}
	gt, err := ctrl.uc.Create(&req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.Created(c, "Gold type created", gt)
}

func (ctrl *GoldTypeController) Update(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid ID")
	}
	var req usecase.GoldTypeRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}
	gt, err := ctrl.uc.Update(uint(id), &req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.Success(c, "Gold type updated", gt)
}

func (ctrl *GoldTypeController) Delete(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid ID")
	}
	if err := ctrl.uc.Delete(uint(id)); err != nil {
		return response.NotFound(c, err.Error())
	}
	return response.Success(c, "Gold type deleted", nil)
}
