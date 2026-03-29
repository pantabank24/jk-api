package controller

import (
	"jk-api/internal/module/config/repository"
	"jk-api/internal/service"
	"jk-api/pkg/response"

	"github.com/gofiber/fiber/v2"
)

type ConfigController struct {
	repo        repository.ConfigRepository
	cronService *service.GoldPriceCron
}

func NewConfigController(repo repository.ConfigRepository, cronService *service.GoldPriceCron) *ConfigController {
	return &ConfigController{repo: repo, cronService: cronService}
}

func (ctrl *ConfigController) GetAll(c *fiber.Ctx) error {
	configs, err := ctrl.repo.GetAll()
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}
	return response.Success(c, "Configs retrieved", configs)
}

type UpdateConfigRequest struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func (ctrl *ConfigController) Update(c *fiber.Ctx) error {
	var req UpdateConfigRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}
	if err := ctrl.repo.Set(req.Key, req.Value); err != nil {
		return response.InternalServerError(c, err.Error())
	}

	// Reload cron if cron-related config changed
	if req.Key == "gold_price_cron" || req.Key == "gold_price_auto_fetch" {
		ctrl.cronService.Reload()
	}

	return response.Success(c, "Config updated", nil)
}
