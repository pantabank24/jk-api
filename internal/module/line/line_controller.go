package line

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"os"
	"strconv"
	"strings"

	"jk-api/internal/entity"
	"jk-api/internal/service"
	"jk-api/pkg/linenotify"
	"jk-api/pkg/response"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type LineController struct {
	db *gorm.DB
}

func NewLineController(db *gorm.DB) *LineController {
	return &LineController{db: db}
}

// Webhook handles POST events from LINE (follow, join, unfollow, leave).
// No JWT auth — LINE signature is used instead.
func (ctrl *LineController) Webhook(c *fiber.Ctx) error {
	body := c.Body()

	// Verify LINE signature
	secret := os.Getenv("LINE_CHANNEL_SECRET")
	if secret != "" {
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(body)
		computed := base64.StdEncoding.EncodeToString(mac.Sum(nil))
		if computed != c.Get("X-Line-Signature") {
			return response.BadRequest(c, "invalid signature")
		}
	}

	var payload linenotify.WebhookPayload
	if err := c.BodyParser(&payload); err != nil {
		return response.BadRequest(c, "invalid payload")
	}

	for _, event := range payload.Events {
		switch event.Type {
		case "follow":
			// User added the bot as a friend → store their userId as notification target
			userID := event.Source.UserID
			if userID != "" {
				ctrl.upsertConfig("line_notify_target_id", userID)
				service.ResetLineLatches(ctrl.db)
				_ = linenotify.ReplyText(event.ReplyToken,
					"✅ เชื่อมต่อสำเร็จ! ระบบจะแจ้งเตือนคุณที่นี่เมื่อมีบิลค้างถึงเกณฑ์ที่กำหนด")
			}

		case "join":
			// Bot was invited to a group → store the groupId as notification target
			groupID := event.Source.GroupID
			if groupID != "" {
				ctrl.upsertConfig("line_notify_target_id", groupID)
				service.ResetLineLatches(ctrl.db)
				_ = linenotify.ReplyText(event.ReplyToken,
					"✅ เชื่อมต่อกลุ่มสำเร็จ! ระบบจะแจ้งเตือนในกลุ่มนี้เมื่อมีบิลค้างถึงเกณฑ์ที่กำหนด")
			}

		case "unfollow", "leave":
			// User blocked the bot or bot was removed from group → clear target
			ctrl.upsertConfig("line_notify_target_id", "")
		}
	}

	return c.SendStatus(fiber.StatusOK)
}

// Status returns the current LINE notification configuration (authenticated),
// alongside the live per-metal backlog so the settings page can show the same
// numbers the เคลียร์บิล screen does.
func (ctrl *LineController) Status(c *fiber.Ctx) error {
	keys := []string{
		"line_notify_enabled",
		"line_notify_target_id",
		// Per-metal toggles + thresholds: gold and silver are cleared separately,
		// so they alert separately too. (The sell-in meters below follow suit.)
		"line_notify_gold_enabled",
		"line_notify_silver_enabled",
		"line_bill_notify_threshold_gold",
		"line_bill_notify_threshold_silver",
	}
	result := fiber.Map{}
	for _, key := range keys {
		var cfg entity.SystemConfig
		if err := ctrl.db.Where("key = ?", key).First(&cfg).Error; err == nil {
			result[key] = cfg.Value
		} else {
			result[key] = ""
		}
	}
	// Don't expose the token itself — just confirm it's configured.
	result["token_set"] = os.Getenv("LINE_CHANNEL_ACCESS_TOKEN") != ""
	// OA Basic ID is non-secret: used to generate the add-friend URL on the frontend.
	result["oa_basic_id"] = os.Getenv("LINE_OA_BASIC_ID")
	result["backlog"] = service.GetLineBacklogStatus(ctrl.db)
	// The sell-in meters carry their own rule (enabled/threshold/purity) rather
	// than being read out of the key list above — one row per metal, so a new
	// metal needs no change here.
	result["sell_accum"] = service.GetSellAccumStatus(ctrl.db)
	return response.Success(c, "ok", result)
}

// Quota reports the channel's monthly push-message allowance and how much of it
// this month has used — the usage dashboard on the settings page. Kept separate
// from Status so a slow/failing LINE API can't hold up the settings page.
func (ctrl *LineController) Quota(c *fiber.Ctx) error {
	quota, err := linenotify.GetQuota()
	if err != nil {
		return response.BadRequest(c, "ไม่สามารถดึงโควตาข้อความจาก LINE ได้: "+err.Error())
	}
	if quota == nil {
		return response.Success(c, "ok", fiber.Map{"configured": false})
	}
	return response.Success(c, "ok", fiber.Map{
		"configured": true,
		"type":       quota.Type,
		"limit":      quota.Limit,
		"used":       quota.Used,
		"remaining":  quota.Remaining,
	})
}

// SaveConfigRequest is the settings page's single save: the master switch, then
// each metal's rule for both alerts — the บิลค้างเคลียร์ threshold and the sell-in
// meter. Saving them together keeps the latch reset (below) consistent with
// whatever combination the shop just chose.
type SaveConfigRequest struct {
	Enabled         *bool `json:"enabled"`
	GoldEnabled     *bool `json:"gold_enabled"`
	SilverEnabled   *bool `json:"silver_enabled"`
	GoldThreshold   *int  `json:"gold_threshold"`
	SilverThreshold *int  `json:"silver_threshold"`
	// Sell-in accumulation alert, keyed by metal so a new metal needs no field
	// here. Its threshold is a weight (บาท for gold, กรัม for silver) rather than
	// a bill count, so it takes decimals; the purity is free text that goes into
	// the message as typed.
	SellAccum map[string]SellAccumInput `json:"sell_accum"`
}

// SellAccumInput is one metal's sell-in rule. Every field is optional so the page
// can save a single toggle without restating the rest.
type SellAccumInput struct {
	Enabled   *bool    `json:"enabled"`
	Threshold *float64 `json:"threshold"`
	Purity    *string  `json:"purity"`
}

// SaveConfig persists the notification settings (authenticated).
func (ctrl *LineController) SaveConfig(c *fiber.Ctx) error {
	var req SaveConfigRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}
	for _, t := range []*int{req.GoldThreshold, req.SilverThreshold} {
		if t != nil && (*t < 0 || *t > 10000) {
			return response.BadRequest(c, "เกณฑ์จำนวนบิลต้องอยู่ระหว่าง 0 ถึง 10000")
		}
	}
	for metal, rule := range req.SellAccum {
		if !service.IsLineMetal(metal) {
			return response.BadRequest(c, "metal ต้องเป็น gold หรือ silver")
		}
		if rule.Threshold != nil && (*rule.Threshold < 0 || *rule.Threshold > 1000000) {
			return response.BadRequest(c, "เกณฑ์น้ำหนักสะสมต้องอยู่ระหว่าง 0 ถึง 1000000")
		}
		if rule.Purity != nil {
			purity := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(*rule.Purity), "%"))
			if purity == "" {
				return response.BadRequest(c, "กรุณาระบุความบริสุทธิ์ที่จะแสดงในข้อความ")
			}
			if len([]rune(purity)) > 10 {
				return response.BadRequest(c, "ความบริสุทธิ์ยาวเกินไป")
			}
		}
	}

	if req.Enabled != nil {
		ctrl.upsertConfig("line_notify_enabled", boolStr(*req.Enabled))
	}
	if req.GoldEnabled != nil {
		ctrl.upsertConfig("line_notify_gold_enabled", boolStr(*req.GoldEnabled))
	}
	if req.SilverEnabled != nil {
		ctrl.upsertConfig("line_notify_silver_enabled", boolStr(*req.SilverEnabled))
	}
	if req.GoldThreshold != nil {
		ctrl.upsertConfig("line_bill_notify_threshold_gold", strconv.Itoa(*req.GoldThreshold))
	}
	if req.SilverThreshold != nil {
		ctrl.upsertConfig("line_bill_notify_threshold_silver", strconv.Itoa(*req.SilverThreshold))
	}
	for metal, rule := range req.SellAccum {
		service.SetSellAccumConfig(ctrl.db, metal, rule.Enabled, rule.Threshold, rule.Purity)
	}

	// A new threshold makes the old latch meaningless: re-arm, then re-evaluate so
	// a backlog that is already over the new (lower) threshold isn't stuck silent
	// until it dips and climbs again.
	service.ResetLineLatches(ctrl.db)
	service.SyncAllLineBacklogAlerts(ctrl.db)

	return response.Success(c, "saved", fiber.Map{
		"backlog":    service.GetLineBacklogStatus(ctrl.db),
		"sell_accum": service.GetSellAccumStatus(ctrl.db),
	})
}

// Test pushes the real alert with the current numbers so the shop can verify the
// connection without waiting for a threshold to be crossed. `kind` picks which
// of the two alerts to send; it defaults to the backlog card so the older
// `?metal=` callers keep working unchanged.
func (ctrl *LineController) Test(c *fiber.Ctx) error {
	metal := c.Query("metal", "gold")
	if !service.IsLineMetal(metal) {
		return response.BadRequest(c, "metal ต้องเป็น gold หรือ silver")
	}
	switch c.Query("kind", "backlog") {
	case "sell_accum":
		if err := service.SendSellAccumTestAlert(ctrl.db, metal); err != nil {
			return response.BadRequest(c, "ส่งข้อความทดสอบไม่สำเร็จ: "+err.Error())
		}
	case "backlog":
		if err := service.SendLineTestAlert(ctrl.db, metal); err != nil {
			return response.BadRequest(c, "ส่งข้อความทดสอบไม่สำเร็จ: "+err.Error())
		}
	default:
		return response.BadRequest(c, "kind ต้องเป็น backlog หรือ sell_accum")
	}
	return response.Success(c, "sent", nil)
}

// ResetSellAccum zeroes one metal's sell-in meter. Manual, and one metal at a
// time, because the carried remainder is real metal on the bench: it is only
// wrong when a lot was settled outside the system, and the shop is the one who
// knows that happened — to one metal, not to both.
func (ctrl *LineController) ResetSellAccum(c *fiber.Ctx) error {
	metal := c.Query("metal", "")
	if !service.IsLineMetal(metal) {
		return response.BadRequest(c, "metal ต้องเป็น gold หรือ silver")
	}
	service.ResetSellAccum(ctrl.db, metal)
	return response.Success(c, "reset", fiber.Map{"sell_accum": service.GetSellAccumStatus(ctrl.db)})
}

// Unlink clears the stored target ID (authenticated).
func (ctrl *LineController) Unlink(c *fiber.Ctx) error {
	ctrl.upsertConfig("line_notify_target_id", "")
	service.ResetLineLatches(ctrl.db)
	return response.Success(c, "unlinked", nil)
}

func boolStr(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

func (ctrl *LineController) upsertConfig(key, value string) {
	ctrl.db.Where(entity.SystemConfig{Key: key}).
		Assign(entity.SystemConfig{Value: value}).
		FirstOrCreate(&entity.SystemConfig{})
	ctrl.db.Model(&entity.SystemConfig{}).Where("key = ?", key).Update("value", value)
}
