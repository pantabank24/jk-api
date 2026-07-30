package controller

import (
	"fmt"
	"strconv"
	"strings"

	"jk-api/internal/module/config/repository"
	"jk-api/internal/service"
	"jk-api/pkg/response"

	"github.com/gofiber/fiber/v2"
	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

type ConfigController struct {
	repo        repository.ConfigRepository
	cronService *service.GoldPriceCron
	sellEngine  *service.SellOrderEngine
	db          *gorm.DB
}

func NewConfigController(repo repository.ConfigRepository, cronService *service.GoldPriceCron, sellEngine *service.SellOrderEngine, db *gorm.DB) *ConfigController {
	return &ConfigController{repo: repo, cronService: cronService, sellEngine: sellEngine, db: db}
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

// GetSilverSellStatus reports whether customers may sell silver right now.
// Available to any authenticated user so customers can check before selling.
func (ctrl *ConfigController) GetSilverSellStatus(c *fiber.Ctx) error {
	return response.Success(c, "ok", service.GetSilverSellStatus(ctrl.db))
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

// validateFloatRange rejects a value that is not a number or falls outside
// [lo, hi]. Used for the config keys that reach live pricing.
func validateFloatRange(value string, lo, hi float64) error {
	v, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return err
	}
	if v < lo || v > hi {
		return fmt.Errorf("out of range")
	}
	return nil
}

func (ctrl *ConfigController) Update(c *fiber.Ctx) error {
	var req UpdateConfigRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	// Reject malformed cron expressions up front so a typo can't freeze price
	// fetching (a bad "gold_price_cron" is what silently killed the cron before).
	if req.Key == "gold_price_cron" || req.Key == "silver_price_cron" {
		if _, err := cron.ParseStandard(req.Value); err != nil {
			return response.BadRequest(c, "รูปแบบ cron ไม่ถูกต้อง (เช่น ทุกนาที = \"* * * * *\", ทุก 30 นาที = \"*/30 * * * *\")")
		}
	}

	// Real-time pricing feeds customer quotes directly, with no build step in
	// between to catch a slip — so a stray zero is rejected here rather than
	// shipped to every open screen.
	switch req.Key {
	case service.KeyRealtimePremium:
		if err := validateFloatRange(req.Value, service.RealtimePremiumMin, service.RealtimePremiumMax); err != nil {
			return response.BadRequest(c, "ค่าปรับราคาต้องเป็นตัวเลขระหว่าง -500 ถึง 500 บาท")
		}
	case service.KeyRealtimeSpread:
		if err := validateFloatRange(req.Value, service.RealtimeSpreadMin, service.RealtimeSpreadMax); err != nil {
			return response.BadRequest(c, "ส่วนต่างราคาต้องเป็นตัวเลขระหว่าง 0 ถึง 1000 บาท")
		}
	// Auto-sell reaches live money with no human in the loop, so its numbers are
	// validated here as well as clamped on read.
	case service.KeyAutoSellMaxSlippage:
		if err := validateFloatRange(req.Value, service.AutoSellMaxSlippageMin, service.AutoSellMaxSlippageMax); err != nil {
			return response.BadRequest(c, "ส่วนต่างสูงสุดต้องเป็นตัวเลขระหว่าง 0 ถึง 10000 บาท (0 = ไม่จำกัด)")
		}
	case service.KeyAutoSellMaxActiveOrders:
		if err := validateFloatRange(req.Value, service.AutoSellMaxActiveOrdersMin, service.AutoSellMaxActiveOrdersMax); err != nil {
			return response.BadRequest(c, "จำนวนคำสั่งขายที่รออยู่ต้องเป็นตัวเลขระหว่าง 1 ถึง 100 รายการ")
		}
	case service.KeyAutoSellMaxActiveWeight:
		if err := validateFloatRange(req.Value, service.AutoSellMaxActiveWeightMin, service.AutoSellMaxActiveWeightMax); err != nil {
			return response.BadRequest(c, "น้ำหนักรวมที่รออยู่ต้องเป็นตัวเลขระหว่าง 1 ถึง 100000 บาท")
		}
	case service.KeyAutoSellMaxFeedAge:
		if err := validateFloatRange(req.Value, service.AutoSellMaxFeedAgeMin, service.AutoSellMaxFeedAgeMax); err != nil {
			return response.BadRequest(c, "อายุราคาสูงสุดต้องเป็นตัวเลขระหว่าง 3 ถึง 600 วินาที")
		}
	case service.KeyAutoSellTickSeconds:
		if err := validateFloatRange(req.Value, service.AutoSellTickSecondsMin, service.AutoSellTickSecondsMax); err != nil {
			return response.BadRequest(c, "ความถี่การตรวจราคาต้องเป็นตัวเลขระหว่าง 2 ถึง 300 วินาที")
		}
	}

	if err := ctrl.repo.Set(req.Key, req.Value); err != nil {
		return response.InternalServerError(c, err.Error())
	}

	// Reload cron if cron-related config changed
	if req.Key == "gold_price_cron" || req.Key == "gold_price_auto_fetch" {
		ctrl.cronService.Reload()
	}

	// Drop the cached pricing so the change shows up on the next poll instead
	// of waiting out the TTL.
	if req.Key == service.KeyRealtimePremium || req.Key == service.KeyRealtimeSpread {
		service.InvalidateRealtimePricing()
	}

	// The engine reads its policy every tick, so only the tick interval itself
	// needs the ticker rebuilt.
	if req.Key == service.KeyAutoSellTickSeconds && ctrl.sellEngine != nil {
		ctrl.sellEngine.Reload()
	}

	return response.Success(c, "Config updated", nil)
}
