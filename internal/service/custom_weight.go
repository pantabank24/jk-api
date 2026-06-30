package service

import (
	"time"

	"jk-api/internal/entity"

	"gorm.io/gorm"
)

// CustomWeightStatus describes whether customers may type the bill weight
// directly right now (instead of using the fixed +/-5 baht stepper). Times
// are HH:MM in Asia/Bangkok.
type CustomWeightStatus struct {
	Enabled    bool   `json:"enabled"`     // master switch (custom_weight_enabled)
	Allowed    bool   `json:"allowed"`     // true when an enabled rule covers now
	OpenTime   string `json:"open_time"`   // effective rule's window
	CloseTime  string `json:"close_time"`
	RuleSource string `json:"rule_source"` // range|weekday|none
	Now        string `json:"now"`
}

// resolveCustomWeightRule picks the rule in effect for `now`: a datetime-range
// override covering now wins (latest start), then a weekday rule. Unlike
// sales scheduling, no rule means NOT allowed (opt-in, not default-open).
func resolveCustomWeightRule(db *gorm.DB, now time.Time) (effectiveRule, bool) {
	var rangeRule entity.CustomWeightSchedule
	if err := db.Where("scope = ? AND start_at <= ? AND end_at >= ?", "range", now, now).
		Order("start_at DESC").First(&rangeRule).Error; err == nil {
		return effectiveRule{
			Enabled: rangeRule.Enabled, OpenTime: rangeRule.OpenTime,
			CloseTime: rangeRule.CloseTime, Source: "range",
		}, true
	}

	wd := int(now.Weekday())
	var wdRule entity.CustomWeightSchedule
	if err := db.Where("scope = ? AND weekday = ?", "weekday", wd).
		First(&wdRule).Error; err == nil {
		return effectiveRule{
			Enabled: wdRule.Enabled, OpenTime: wdRule.OpenTime,
			CloseTime: wdRule.CloseTime, Source: "weekday",
		}, true
	}

	return effectiveRule{}, false
}

// GetCustomWeightStatus resolves whether typing the weight directly is
// allowed right now, from the master switch and the schedule rules.
func GetCustomWeightStatus(db *gorm.DB) CustomWeightStatus {
	master := configValue(db, "custom_weight_enabled", "false") == "true"
	now := bangkokNow()
	nowHM := now.Format("15:04")

	rule, found := resolveCustomWeightRule(db, now)
	if !found {
		return CustomWeightStatus{Enabled: master, Allowed: false, RuleSource: "none", Now: nowHM}
	}

	allowed := master && rule.Enabled && withinWindow(nowHM, rule.OpenTime, rule.CloseTime)

	return CustomWeightStatus{
		Enabled:    master,
		Allowed:    allowed,
		OpenTime:   rule.OpenTime,
		CloseTime:  rule.CloseTime,
		RuleSource: rule.Source,
		Now:        nowHM,
	}
}
