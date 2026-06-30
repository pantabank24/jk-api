package controller

import (
	"strconv"
	"time"

	"jk-api/internal/entity"
	"jk-api/internal/module/sales_schedule/repository"
	"jk-api/pkg/response"

	"github.com/gofiber/fiber/v2"
)

type SalesScheduleController struct {
	repo repository.SalesScheduleRepository
}

func NewSalesScheduleController(repo repository.SalesScheduleRepository) *SalesScheduleController {
	return &SalesScheduleController{repo: repo}
}

func (ctrl *SalesScheduleController) GetAll(c *fiber.Ctx) error {
	rules, err := ctrl.repo.GetAll()
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}
	return response.Success(c, "Sales schedules retrieved", rules)
}

type upsertRequest struct {
	ID                 uint       `json:"id"`
	Scope              string     `json:"scope"` // weekday|range
	Weekday            *int       `json:"weekday"`
	StartAt            *time.Time `json:"start_at"` // RFC3339 (range)
	EndAt              *time.Time `json:"end_at"`   // RFC3339 (range)
	Enabled            bool       `json:"enabled"`
	OpenTime           string     `json:"open_time"`
	CloseTime          string     `json:"close_time"`
	RealtimeAfterHours bool       `json:"realtime_after_hours"`
	Note               string     `json:"note"`
}

func (ctrl *SalesScheduleController) Upsert(c *fiber.Ctx) error {
	var req upsertRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}
	if req.Scope != "weekday" && req.Scope != "range" {
		return response.BadRequest(c, "scope must be 'weekday' or 'range'")
	}

	rule := entity.SalesSchedule{
		ID:                 req.ID,
		Scope:              req.Scope,
		Enabled:            req.Enabled,
		OpenTime:           defaultStr(req.OpenTime, "09:30"),
		CloseTime:          defaultStr(req.CloseTime, "16:30"),
		RealtimeAfterHours: req.RealtimeAfterHours,
		Note:               req.Note,
	}

	switch req.Scope {
	case "weekday":
		if req.Weekday == nil || *req.Weekday < 0 || *req.Weekday > 6 {
			return response.BadRequest(c, "weekday must be 0-6")
		}
		rule.Weekday = req.Weekday
	case "range":
		if req.StartAt == nil || req.EndAt == nil {
			return response.BadRequest(c, "ต้องระบุวันเวลาเริ่มและสิ้นสุด")
		}
		if req.EndAt.Before(*req.StartAt) {
			return response.BadRequest(c, "วันเวลาสิ้นสุดต้องอยู่หลังวันเวลาเริ่ม")
		}
		rule.StartAt = req.StartAt
		rule.EndAt = req.EndAt
	}

	if req.ID > 0 {
		if err := ctrl.repo.Update(&rule); err != nil {
			return response.InternalServerError(c, err.Error())
		}
		return response.Success(c, "Sales schedule updated", rule)
	}
	if err := ctrl.repo.Create(&rule); err != nil {
		return response.InternalServerError(c, err.Error())
	}
	return response.Created(c, "Sales schedule created", rule)
}

func (ctrl *SalesScheduleController) Delete(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "invalid id")
	}
	if err := ctrl.repo.Delete(uint(id)); err != nil {
		return response.InternalServerError(c, err.Error())
	}
	return response.Success(c, "Sales schedule deleted", nil)
}

func defaultStr(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
