package controller

import (
	"strconv"

	"jk-api/internal/module/document_type/usecase"
	"jk-api/pkg/response"

	"github.com/gofiber/fiber/v2"
)

type DocumentTypeController struct {
	uc usecase.DocumentTypeUsecase
}

func NewDocumentTypeController(uc usecase.DocumentTypeUsecase) *DocumentTypeController {
	return &DocumentTypeController{uc: uc}
}

func (ctrl *DocumentTypeController) GetAll(c *fiber.Ctx) error {
	types, err := ctrl.uc.GetAll()
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}
	return response.Success(c, "Document types retrieved", types)
}

func (ctrl *DocumentTypeController) GetByID(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid ID")
	}
	t, err := ctrl.uc.GetByID(uint(id))
	if err != nil {
		return response.NotFound(c, "ไม่พบประเภทเอกสาร")
	}
	return response.Success(c, "Document type retrieved", t)
}

func (ctrl *DocumentTypeController) Create(c *fiber.Ctx) error {
	var req usecase.DocumentTypeRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}
	t, err := ctrl.uc.Create(&req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.Created(c, "Document type created", t)
}

func (ctrl *DocumentTypeController) Update(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid ID")
	}
	var req usecase.DocumentTypeRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}
	t, err := ctrl.uc.Update(uint(id), &req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.Success(c, "Document type updated", t)
}

func (ctrl *DocumentTypeController) Delete(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid ID")
	}
	if err := ctrl.uc.Delete(uint(id)); err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.Success(c, "Document type deleted", nil)
}
