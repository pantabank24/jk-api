package controller

import (
	"strconv"

	"jk-api/internal/middleware"
	"jk-api/internal/module/receipt/repository"
	"jk-api/internal/module/receipt/usecase"
	"jk-api/pkg/response"
	"jk-api/pkg/upload"

	"github.com/gofiber/fiber/v2"
)

type ReceiptController struct {
	uc usecase.ReceiptUsecase
}

func NewReceiptController(uc usecase.ReceiptUsecase) *ReceiptController {
	return &ReceiptController{uc: uc}
}

func (ctrl *ReceiptController) GetAll(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	if limit < 1 || limit > 200 {
		limit = 20
	}
	f := repository.ReceiptFilter{Search: c.Query("search")}

	receipts, total, err := ctrl.uc.GetAll(f, page, limit)
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}
	return response.Paginated(c, "Receipts retrieved", receipts, page, limit, total)
}

func (ctrl *ReceiptController) GetByID(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid ID")
	}
	r, err := ctrl.uc.GetByID(uint(id))
	if err != nil {
		return response.NotFound(c, "ไม่พบใบเสร็จ")
	}
	return response.Success(c, "Receipt retrieved", r)
}

func (ctrl *ReceiptController) Create(c *fiber.Ctx) error {
	var req usecase.ReceiptRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}
	r, err := ctrl.uc.Create(&req, middleware.GetUserID(c))
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.Created(c, "Receipt created", r)
}

func (ctrl *ReceiptController) Update(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid ID")
	}
	var req usecase.ReceiptRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}
	r, err := ctrl.uc.Update(uint(id), &req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.Success(c, "Receipt updated", r)
}

func (ctrl *ReceiptController) Delete(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid ID")
	}
	if err := ctrl.uc.Delete(uint(id)); err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.Success(c, "Receipt deleted", nil)
}

func (ctrl *ReceiptController) GetSettings(c *fiber.Ctx) error {
	s, err := ctrl.uc.GetSettings()
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}
	return response.Success(c, "Receipt settings retrieved", s)
}

func (ctrl *ReceiptController) UpdateSettings(c *fiber.Ctx) error {
	var req usecase.SettingsRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}
	s, err := ctrl.uc.SaveSettings(&req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.Success(c, "Receipt settings updated", s)
}

func (ctrl *ReceiptController) UploadLogo(c *fiber.Ctx) error {
	path, err := upload.SaveFile(c, "logo", "receipts")
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	s, err := ctrl.uc.SaveLogo(path)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.Success(c, "Logo uploaded", s)
}
