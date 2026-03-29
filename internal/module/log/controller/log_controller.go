package controller

import (
	"strconv"

	"jk-api/internal/module/log/usecase"
	"jk-api/pkg/response"

	"github.com/gofiber/fiber/v2"
)

type LogController struct {
	logUsecase usecase.LogUsecase
}

func NewLogController(logUsecase usecase.LogUsecase) *LogController {
	return &LogController{logUsecase: logUsecase}
}

func (ctrl *LogController) GetLoginLogs(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))

	var userID *uint
	if uid := c.Query("user_id"); uid != "" {
		id, _ := strconv.ParseUint(uid, 10, 32)
		u := uint(id)
		userID = &u
	}

	var success *bool
	if s := c.Query("success"); s != "" {
		b := s == "true"
		success = &b
	}

	logs, total, err := ctrl.logUsecase.GetLoginLogs(userID, success, page, limit)
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}
	return response.Paginated(c, "Login logs retrieved", logs, page, limit, total)
}

func (ctrl *LogController) GetActivityLogs(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	method := c.Query("method", "")

	var userID *uint
	if uid := c.Query("user_id"); uid != "" {
		id, _ := strconv.ParseUint(uid, 10, 32)
		u := uint(id)
		userID = &u
	}

	logs, total, err := ctrl.logUsecase.GetActivityLogs(userID, method, page, limit)
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}
	return response.Paginated(c, "Activity logs retrieved", logs, page, limit, total)
}

func (ctrl *LogController) DeleteOldLoginLogs(c *fiber.Ctx) error {
	days, _ := strconv.Atoi(c.Query("days", "90"))
	deleted, err := ctrl.logUsecase.DeleteOldLoginLogs(days)
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}
	return response.Success(c, "Deleted old login logs", fiber.Map{"deleted": deleted})
}

func (ctrl *LogController) DeleteOldActivityLogs(c *fiber.Ctx) error {
	days, _ := strconv.Atoi(c.Query("days", "90"))
	deleted, err := ctrl.logUsecase.DeleteOldActivityLogs(days)
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}
	return response.Success(c, "Deleted old activity logs", fiber.Map{"deleted": deleted})
}
