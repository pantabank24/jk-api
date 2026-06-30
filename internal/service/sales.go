package service

import (
	"time"

	"jk-api/internal/entity"

	"gorm.io/gorm"
)

// Price modes returned by the sales resolver.
const (
	PriceModeClosed      = "closed"      // selling not allowed right now
	PriceModeAssociation = "association" // use Gold Traders Association price
	PriceModeRealtime    = "realtime"    // use real-time (TradingView) price
)

// SalesStatus describes whether sales are currently open and which price source
// applies. Times are HH:MM in Asia/Bangkok.
type SalesStatus struct {
	Enabled            bool   `json:"enabled"`              // master switch (sales_enabled)
	IsOpen             bool   `json:"is_open"`              // true when PriceMode != closed
	PriceMode          string `json:"price_mode"`           // closed|association|realtime
	OpenTime           string `json:"open_time"`            // effective rule's window
	CloseTime          string `json:"close_time"`
	RealtimeAfterHours bool   `json:"realtime_after_hours"` // effective rule
	RuleSource         string `json:"rule_source"`          // date|weekday|default
	Now                string `json:"now"`
}

func bangkokNow() time.Time {
	loc, err := time.LoadLocation("Asia/Bangkok")
	if err != nil {
		loc = time.FixedZone("ICT", 7*3600)
	}
	return time.Now().In(loc)
}

func configValue(db *gorm.DB, key, def string) string {
	var c entity.SystemConfig
	if err := db.Where("key = ?", key).First(&c).Error; err == nil && c.Value != "" {
		return c.Value
	}
	return def
}

// effectiveRule is the resolved window+flags that apply to a given moment.
type effectiveRule struct {
	Enabled            bool
	OpenTime           string
	CloseTime          string
	RealtimeAfterHours bool
	Source             string // date|weekday|default
}

// resolveRule picks the rule in effect for `now`: a datetime-range override
// covering now wins (latest start), then a weekday rule, then the defaults.
func resolveRule(db *gorm.DB, now time.Time) effectiveRule {
	// 1. Range override covering now (most-recently-started wins on overlap).
	var rangeRule entity.SalesSchedule
	if err := db.Where("scope = ? AND start_at <= ? AND end_at >= ?", "range", now, now).
		Order("start_at DESC").First(&rangeRule).Error; err == nil {
		return effectiveRule{
			Enabled: rangeRule.Enabled, OpenTime: rangeRule.OpenTime,
			CloseTime: rangeRule.CloseTime, RealtimeAfterHours: rangeRule.RealtimeAfterHours,
			Source: "range",
		}
	}

	// 2. Weekday rule.
	wd := int(now.Weekday()) // 0=Sun..6=Sat
	var wdRule entity.SalesSchedule
	if err := db.Where("scope = ? AND weekday = ?", "weekday", wd).
		First(&wdRule).Error; err == nil {
		return effectiveRule{
			Enabled: wdRule.Enabled, OpenTime: wdRule.OpenTime,
			CloseTime: wdRule.CloseTime, RealtimeAfterHours: wdRule.RealtimeAfterHours,
			Source: "weekday",
		}
	}

	// 3. Default config keys.
	return effectiveRule{
		Enabled:            true,
		OpenTime:           configValue(db, "sales_open_time", "09:30"),
		CloseTime:          configValue(db, "sales_close_time", "16:30"),
		RealtimeAfterHours: configValue(db, "sales_realtime_after_hours", "false") == "true",
		Source:             "default",
	}
}

// GetSalesStatus resolves the current sales mode from the master switch, the
// schedule rules, and the trading window. Real-time applies to ANY time outside
// the association window when the effective rule enables it.
func GetSalesStatus(db *gorm.DB) SalesStatus {
	master := configValue(db, "sales_enabled", "true") == "true"
	now := bangkokNow()
	nowHM := now.Format("15:04")

	rule := resolveRule(db, now)

	mode := PriceModeClosed
	switch {
	case !master || !rule.Enabled:
		mode = PriceModeClosed
	case withinWindow(nowHM, rule.OpenTime, rule.CloseTime):
		mode = PriceModeAssociation
	case rule.RealtimeAfterHours:
		mode = PriceModeRealtime
	default:
		mode = PriceModeClosed
	}

	return SalesStatus{
		Enabled:            master,
		IsOpen:             mode != PriceModeClosed,
		PriceMode:          mode,
		OpenTime:           rule.OpenTime,
		CloseTime:          rule.CloseTime,
		RealtimeAfterHours: rule.RealtimeAfterHours,
		RuleSource:         rule.Source,
		Now:                nowHM,
	}
}

// withinWindow compares zero-padded HH:MM strings (lexical compare is valid).
// Supports overnight windows where open > close (e.g. 22:00-02:00).
func withinWindow(now, open, clse string) bool {
	if open <= clse {
		return now >= open && now <= clse
	}
	return now >= open || now <= clse
}

// CurrentRound returns the gold-price round in effect right now (the latest gold
// price), used to stamp quotations/bills for reporting. Returns ("", nil) when
// no gold price exists yet.
func CurrentRound(db *gorm.DB) (string, *uint) {
	var gp entity.GoldPrice
	if err := db.Order("id DESC").First(&gp).Error; err != nil {
		return "", nil
	}
	id := gp.ID
	return gp.GoldRound, &id
}
