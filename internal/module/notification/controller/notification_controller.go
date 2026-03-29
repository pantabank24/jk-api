package controller

import (
	"strconv"

	"jk-api/internal/middleware"
	"jk-api/internal/module/notification/usecase"
	"jk-api/pkg/response"

	"github.com/gofiber/fiber/v2"
)

type NotificationController struct {
	uc usecase.NotificationUsecase
}

func NewNotificationController(uc usecase.NotificationUsecase) *NotificationController {
	return &NotificationController{uc: uc}
}

func (ctrl *NotificationController) GetNotifications(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	notifs, total, err := ctrl.uc.GetNotifications(userID, page, limit)
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}
	return response.Paginated(c, "Notifications retrieved", notifs, page, limit, total)
}

func (ctrl *NotificationController) CountUnread(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	count, err := ctrl.uc.CountUnread(userID)
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}
	return response.Success(c, "Unread count", map[string]int64{"count": count})
}

func (ctrl *NotificationController) MarkRead(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid notification ID")
	}
	if err := ctrl.uc.MarkRead(uint(id), userID); err != nil {
		return response.InternalServerError(c, err.Error())
	}
	return response.Success(c, "Marked as read", nil)
}

func (ctrl *NotificationController) MarkAllRead(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	if err := ctrl.uc.MarkAllRead(userID); err != nil {
		return response.InternalServerError(c, err.Error())
	}
	return response.Success(c, "All marked as read", nil)
}
