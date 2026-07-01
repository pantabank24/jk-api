package line

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"os"

	"jk-api/internal/entity"
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
				_ = linenotify.ReplyText(event.ReplyToken,
					"✅ เชื่อมต่อสำเร็จ! ระบบจะแจ้งเตือนคุณที่นี่เมื่อมีบิลค้างถึงเกณฑ์ที่กำหนด")
			}

		case "join":
			// Bot was invited to a group → store the groupId as notification target
			groupID := event.Source.GroupID
			if groupID != "" {
				ctrl.upsertConfig("line_notify_target_id", groupID)
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

// Status returns the current LINE notification configuration (authenticated).
func (ctrl *LineController) Status(c *fiber.Ctx) error {
	keys := []string{"line_notify_enabled", "line_notify_target_id", "line_bill_notify_threshold"}
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
	return response.Success(c, "ok", result)
}

// Unlink clears the stored target ID (authenticated).
func (ctrl *LineController) Unlink(c *fiber.Ctx) error {
	ctrl.upsertConfig("line_notify_target_id", "")
	return response.Success(c, "unlinked", nil)
}

func (ctrl *LineController) upsertConfig(key, value string) {
	ctrl.db.Where(entity.SystemConfig{Key: key}).
		Assign(entity.SystemConfig{Value: value}).
		FirstOrCreate(&entity.SystemConfig{})
	ctrl.db.Model(&entity.SystemConfig{}).Where("key = ?", key).Update("value", value)
}
