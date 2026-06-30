package controller

import (
	"strconv"

	"jk-api/internal/middleware"
	"jk-api/internal/module/news/usecase"
	"jk-api/pkg/response"
	"jk-api/pkg/upload"

	"github.com/gofiber/fiber/v2"
)

type NewsController struct {
	newsUsecase usecase.NewsUsecase
}

func NewNewsController(newsUsecase usecase.NewsUsecase) *NewsController {
	return &NewsController{newsUsecase: newsUsecase}
}

func (ctrl *NewsController) CreateNews(c *fiber.Ctx) error {
	var req usecase.CreateNewsRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}
	if req.Title == "" || req.Body == "" {
		return response.BadRequest(c, "กรุณากรอกหัวข้อและเนื้อหาข่าว")
	}
	req.CreatedBy = middleware.GetUserID(c)

	news, err := ctrl.newsUsecase.CreateNews(&req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.Created(c, "News created", news)
}

func (ctrl *NewsController) GetAllNews(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "10"))

	news, total, err := ctrl.newsUsecase.GetAllNews(page, limit)
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}
	return response.Paginated(c, "News retrieved", news, page, limit, total)
}

func (ctrl *NewsController) GetVisibleNews(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "5"))
	roleName := middleware.GetRoleName(c)

	news, total, err := ctrl.newsUsecase.GetVisibleNews(roleName, page, limit)
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}
	return response.Paginated(c, "News retrieved", news, page, limit, total)
}

func (ctrl *NewsController) GetNewsByID(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid news ID")
	}

	news, err := ctrl.newsUsecase.GetNewsByID(uint(id))
	if err != nil {
		return response.NotFound(c, "News not found")
	}
	return response.Success(c, "News retrieved", news)
}

func (ctrl *NewsController) UpdateNews(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid news ID")
	}

	var req usecase.UpdateNewsRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	news, err := ctrl.newsUsecase.UpdateNews(uint(id), &req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.Success(c, "News updated", news)
}

func (ctrl *NewsController) DeleteNews(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid news ID")
	}

	if err := ctrl.newsUsecase.DeleteNews(uint(id)); err != nil {
		return response.NotFound(c, err.Error())
	}
	return response.Success(c, "News deleted", nil)
}

func (ctrl *NewsController) UploadImage(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid news ID")
	}

	path, err := upload.SaveFile(c, "image", "news")
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	news, err := ctrl.newsUsecase.UpdateImage(uint(id), path)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.Success(c, "Image uploaded", news)
}
