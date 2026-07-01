package controller

import (
	"jk-api/internal/module/config/repository"
	"jk-api/internal/service"
	"jk-api/pkg/response"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type ConfigController struct {
	repo        repository.ConfigRepository
	cronService *service.GoldPriceCron
	db          *gorm.DB
}

func NewConfigController(repo repository.ConfigRepository, cronService *service.GoldPriceCron, db *gorm.DB) *ConfigController {
	return &ConfigController{repo: repo, cronService: cronService, db: db}
}

// GetSalesStatus reports whether sales are open right now. Available to any
// authenticated user (not gated by config.read) so staff/customers can see it.
func (ctrl *ConfigController) GetSalesStatus(c *fiber.Ctx) error {
	return response.Success(c, "ok", service.GetSalesStatus(ctrl.db))
}

// GetCustomWeightStatus reports whether customers may type the bill weight
// directly right now. Available to any authenticated user (not gated by
// config.read) so customers can see it.
func (ctrl *ConfigController) GetCustomWeightStatus(c *fiber.Ctx) error {
	return response.Success(c, "ok", service.GetCustomWeightStatus(ctrl.db))
}

// GetBillsOpenStatus reports whether customer bill creation is enabled.
// Available to any authenticated user so customers can check before selling.
func (ctrl *ConfigController) GetBillsOpenStatus(c *fiber.Ctx) error {
	cfg, err := ctrl.repo.GetByKey("bills_open")
	open := true
	if err == nil {
		open = cfg.Value != "false"
	}
	return response.Success(c, "ok", fiber.Map{"open": open})
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
