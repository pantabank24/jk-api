package controller

import (
	"strconv"

	"jk-api/internal/module/bank/usecase"
	"jk-api/pkg/response"

	"github.com/gofiber/fiber/v2"
)

type BankController struct {
	uc usecase.BankUsecase
}

func NewBankController(uc usecase.BankUsecase) *BankController {
	return &BankController{uc: uc}
}

func (ctrl *BankController) GetAll(c *fiber.Ctx) error {
	banks, err := ctrl.uc.GetAll()
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}
	return response.Success(c, "Banks retrieved", banks)
}

func (ctrl *BankController) GetByID(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid ID")
	}
	b, err := ctrl.uc.GetByID(uint(id))
	if err != nil {
		return response.NotFound(c, "ไม่พบธนาคาร")
	}
	return response.Success(c, "Bank retrieved", b)
}

func (ctrl *BankController) Create(c *fiber.Ctx) error {
	var req usecase.BankRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}
	b, err := ctrl.uc.Create(&req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.Created(c, "Bank created", b)
}

func (ctrl *BankController) Update(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid ID")
	}
	var req usecase.BankRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}
	b, err := ctrl.uc.Update(uint(id), &req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.Success(c, "Bank updated", b)
}

func (ctrl *BankController) Delete(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid ID")
	}
	if err := ctrl.uc.Delete(uint(id)); err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.Success(c, "Bank deleted", nil)
}
