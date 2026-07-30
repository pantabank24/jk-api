package service

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	"jk-api/internal/entity"
	"jk-api/pkg/linenotify"

	"gorm.io/gorm"
)

// setConfigValue writes a system config row, creating it if the migration that
// seeds it hasn't run yet.
func setConfigValue(db *gorm.DB, key, value string) {
	var existing entity.SystemConfig
	if err := db.Where("key = ?", key).First(&existing).Error; err != nil {
		db.Create(&entity.SystemConfig{Key: key, Value: value})
		return
	}
	db.Model(&entity.SystemConfig{}).Where("key = ?", key).Update("value", value)
}

// Bill statuses this file cares about. Duplicated from the bill repository's
// constants rather than imported, so service/ stays free of module/ imports.
const (
	billStatusCompleted = 12 // สำเร็จ — waiting for เคลียร์บิล
)

// LINE config keys. The per-metal pair is derived by suffix so a new metal only
// needs a migration, not a code change here.
const (
	KeyLineEnabled  = "line_notify_enabled"   // master switch
	KeyLineTargetID = "line_notify_target_id" // user or group to push to
)

func keyLineMetalEnabled(metal string) string   { return "line_notify_" + metal + "_enabled" }
func keyLineMetalThreshold(metal string) string { return "line_bill_notify_threshold_" + metal }
func keyLineMetalLatch(metal string) string     { return "line_notify_latch_" + metal }

// LineMetals are the metals that get their own alert channel — one per เคลียร์บิล
// page (/bills = ทอง, /bills/silver = เงิน).
var LineMetals = []string{"gold", "silver"}

// metalLabel / metalUnit mirror the frontend: gold is weighed in baht, every
// other metal in grams.
func metalLabel(metal string) string {
	if metal == "silver" {
		return "เงิน"
	}
	return "ทอง"
}

func metalUnit(metal string) string {
	if metal == "silver" {
		return "กรัม"
	}
	return "บาท"
}

// metalTheme is the header colour of the Flex bubble — the shop's gold, and a
// neutral grey for silver.
func metalTheme(metal string) string {
	if metal == "silver" {
		return "#8A8F98"
	}
	return "#C09C42"
}

// LineBacklog is the pending-clear picture for one metal, as the เคลียร์บิล page
// would show it.
type LineBacklog struct {
	Metal  string  `json:"metal"`
	Count  int64   `json:"count"`  // bills waiting to be cleared (issue-groups, not rows)
	Weight float64 `json:"weight"` // total weight of those bills, in the metal's unit
	Amount float64 `json:"amount"` // total amount of those bills (baht)
}

// CountLineBacklog counts the bills sitting at สำเร็จ (12) for one metal, the way
// the เคลียร์บิล screen counts them: bills issued together are ONE bill, because
// they settle together on a single ledger row. Legacy rows with no metal tag read
// as gold (see migration 85).
func CountLineBacklog(db *gorm.DB, metal string, storeID *uint) LineBacklog {
	out := LineBacklog{Metal: metal}

	bills := db.Model(&entity.Quotation{}).
		Where("is_bill = ? AND status = ?", true, billStatusCompleted).
		Where("COALESCE(NULLIF(metal, ''), 'gold') = ?", metal)
	if storeID != nil {
		bills = bills.Where("store_id = ?", *storeID)
	}

	var head struct{ Count int64 }
	if err := bills.Session(&gorm.Session{}).
		Select("COUNT(DISTINCT COALESCE(issued_quotation_id, id)) AS count").
		Scan(&head).Error; err != nil {
		return out
	}
	out.Count = head.Count
	if out.Count == 0 {
		return out
	}

	// Weight/amount come from the items, filtered to this metal so a legacy mixed
	// bill can't drag the other metal's lines in.
	var agg struct {
		Weight float64
		Amount float64
	}
	db.Model(&entity.QuotationItem{}).
		Where("quotation_id IN (?)", bills.Session(&gorm.Session{}).Select("quotations.id")).
		Where("COALESCE(NULLIF(metal, ''), 'gold') = ?", metal).
		Select("COALESCE(SUM(weight), 0) AS weight, COALESCE(SUM(total), 0) AS amount").
		Scan(&agg)
	out.Weight, out.Amount = agg.Weight, agg.Amount
	return out
}

// SyncLineBacklogAlert re-evaluates one metal's backlog against its threshold and
// pushes a Flex alert on the UPWARD crossing only — 19 → 20 sends, 21 → 22 stays
// quiet, and the latch re-arms once the backlog drops back below the threshold.
// That is what keeps a busy day from burning the monthly message quota.
//
// allowSend=false skips the push and only re-arms the latch: callers that can
// only shrink the backlog (เคลียร์บิล, ยกเลิก, ดึงกลับไปแก้ไข, ลบบิล) pass false, so
// a change on one metal never triggers the other's alert.
//
// Safe to call in a goroutine; every failure path is silent by design (an alert
// must never break the request that caused it).
func SyncLineBacklogAlert(db *gorm.DB, metal string, storeID *uint, allowSend bool) {
	if metal == "" {
		metal = "gold"
	}
	threshold, _ := strconv.Atoi(strings.TrimSpace(configValue(db, keyLineMetalThreshold(metal), "0")))
	if threshold <= 0 {
		// Alerts off for this metal — drop the latch so re-enabling starts clean.
		setConfigValue(db, keyLineMetalLatch(metal), "false")
		return
	}

	backlog := CountLineBacklog(db, metal, storeID)
	latched := configValue(db, keyLineMetalLatch(metal), "false") == "true"

	if backlog.Count < int64(threshold) {
		if latched {
			setConfigValue(db, keyLineMetalLatch(metal), "false")
		}
		return
	}
	if latched || !allowSend {
		return
	}

	// At or above the threshold and not yet alerted: latch first, then push.
	// Latching up
	// front means a failed push doesn't turn into a retry storm on the next
	// approval — the shop still sees the backlog on screen.
	setConfigValue(db, keyLineMetalLatch(metal), "true")

	if configValue(db, KeyLineEnabled, "false") != "true" {
		return
	}
	if configValue(db, keyLineMetalEnabled(metal), "false") != "true" {
		return
	}
	target := configValue(db, KeyLineTargetID, "")
	if target == "" {
		return
	}

	alt := fmt.Sprintf("🔔 บิล%sค้างเคลียร์ %d บิล (เกณฑ์ %d)", metalLabel(metal), backlog.Count, threshold)
	_ = linenotify.SendFlex(target, alt, backlogBubble(metal, backlog, threshold))
}

// SyncAllLineBacklogAlerts re-arms every metal's latch without sending. Used
// after a settings change, so a lowered threshold can fire again immediately and
// a raised one doesn't stay stuck latched.
func SyncAllLineBacklogAlerts(db *gorm.DB) {
	for _, metal := range LineMetals {
		SyncLineBacklogAlert(db, metal, nil, false)
	}
}

// ResetLineLatches clears both latches outright. Called when the target is
// unlinked or the thresholds change, so the next crossing always alerts.
func ResetLineLatches(db *gorm.DB) {
	for _, metal := range LineMetals {
		setConfigValue(db, keyLineMetalLatch(metal), "false")
	}
}

// LineBacklogStatus reports every metal's backlog next to its configured
// threshold — what the settings page shows so the shop can see whether the
// numbers match the เคลียร์บิล screen.
type LineBacklogStatus struct {
	Metal     string  `json:"metal"`
	Enabled   bool    `json:"enabled"`
	Threshold int     `json:"threshold"`
	Count     int64   `json:"count"`
	Weight    float64 `json:"weight"`
	Amount    float64 `json:"amount"`
	Latched   bool    `json:"latched"`
}

func GetLineBacklogStatus(db *gorm.DB) []LineBacklogStatus {
	out := make([]LineBacklogStatus, 0, len(LineMetals))
	for _, metal := range LineMetals {
		threshold, _ := strconv.Atoi(strings.TrimSpace(configValue(db, keyLineMetalThreshold(metal), "0")))
		backlog := CountLineBacklog(db, metal, nil)
		out = append(out, LineBacklogStatus{
			Metal:     metal,
			Enabled:   configValue(db, keyLineMetalEnabled(metal), "false") == "true",
			Threshold: threshold,
			Count:     backlog.Count,
			Weight:    backlog.Weight,
			Amount:    backlog.Amount,
			Latched:   configValue(db, keyLineMetalLatch(metal), "false") == "true",
		})
	}
	return out
}

// SendLineTestAlert pushes the real Flex card with the current numbers, so the
// shop can confirm the wiring without waiting for a crossing. Returns an error
// the settings page can show.
func SendLineTestAlert(db *gorm.DB, metal string) error {
	target := configValue(db, KeyLineTargetID, "")
	if target == "" {
		return fmt.Errorf("ยังไม่ได้เชื่อมต่อ LINE")
	}
	threshold, _ := strconv.Atoi(strings.TrimSpace(configValue(db, keyLineMetalThreshold(metal), "0")))
	backlog := CountLineBacklog(db, metal, nil)
	alt := fmt.Sprintf("🔔 [ทดสอบ] บิล%sค้างเคลียร์ %d บิล", metalLabel(metal), backlog.Count)
	return linenotify.SendFlex(target, alt, backlogBubble(metal, backlog, threshold))
}

// backlogBubble builds the Flex bubble for a backlog alert: coloured header per
// metal, the count as the hero figure, then the weight/amount detail rows and a
// deep link into the เคลียร์บิล page when APP_PUBLIC_URL is configured.
func backlogBubble(metal string, b LineBacklog, threshold int) map[string]interface{} {
	theme := metalTheme(metal)

	body := []map[string]interface{}{
		{
			"type": "box", "layout": "baseline", "contents": []map[string]interface{}{
				{"type": "text", "text": formatInt(b.Count), "size": "3xl", "weight": "bold", "color": "#222222", "flex": 0},
				{"type": "text", "text": "บิล", "size": "sm", "color": "#999999", "margin": "sm", "flex": 0},
			},
		},
		{
			"type": "text",
			"text": fmt.Sprintf("ถึงเกณฑ์ที่ตั้งไว้ (%d บิล) — รอเคลียร์บิล%s", threshold, metalLabel(metal)),
			"size": "xs", "color": "#999999", "wrap": true,
		},
		{"type": "separator", "margin": "lg", "color": "#EEEEEE"},
		detailRow("น้ำหนักรวม", fmt.Sprintf("%s %s", formatFloat(b.Weight), metalUnit(metal))),
		detailRow("ยอดรวม", fmt.Sprintf("%s บาท", formatFloat(b.Amount))),
	}

	bubble := map[string]interface{}{
		"type": "bubble",
		"size": "mega",
		"header": map[string]interface{}{
			"type": "box", "layout": "vertical", "backgroundColor": theme,
			"paddingAll": "16px", "spacing": "xs",
			"contents": []map[string]interface{}{
				{"type": "text", "text": "🔔 บิลค้างเคลียร์", "color": "#FFFFFF", "size": "sm", "weight": "bold"},
				{"type": "text", "text": "รายการขาย" + metalLabel(metal), "color": "#FFFFFFCC", "size": "xs"},
			},
		},
		"body": map[string]interface{}{
			"type": "box", "layout": "vertical", "paddingAll": "16px", "spacing": "sm",
			"contents": body,
		},
	}

	if url := billsPageURL(metal); url != "" {
		bubble["footer"] = map[string]interface{}{
			"type": "box", "layout": "vertical", "paddingAll": "12px",
			"contents": []map[string]interface{}{{
				"type": "button", "style": "primary", "height": "sm", "color": theme,
				"action": map[string]interface{}{"type": "uri", "label": "เปิดหน้าเคลียร์บิล", "uri": url},
			}},
		}
	}
	return bubble
}

func detailRow(label, value string) map[string]interface{} {
	return map[string]interface{}{
		"type": "box", "layout": "horizontal", "margin": "md",
		"contents": []map[string]interface{}{
			{"type": "text", "text": label, "size": "sm", "color": "#999999", "flex": 3},
			{"type": "text", "text": value, "size": "sm", "color": "#333333", "weight": "bold", "align": "end", "flex": 4},
		},
	}
}

// billsPageURL deep-links the alert to the metal's เคลียร์บิล page. Empty when
// APP_PUBLIC_URL is unset — the card then simply has no button.
func billsPageURL(metal string) string {
	base := strings.TrimRight(os.Getenv("APP_PUBLIC_URL"), "/")
	if base == "" {
		return ""
	}
	if metal == "gold" {
		return base + "/bills"
	}
	return base + "/bills/" + metal
}

func formatInt(n int64) string {
	s := strconv.FormatInt(n, 10)
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	for i, ch := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(ch)
	}
	return b.String()
}

// formatFloat renders a 2-decimal, thousands-separated number for the Flex card.
func formatFloat(v float64) string {
	sign := ""
	if v < 0 {
		sign, v = "-", -v
	}
	cents := int64(math.Round(v * 100))
	return fmt.Sprintf("%s%s.%02d", sign, formatInt(cents/100), cents%100)
}
