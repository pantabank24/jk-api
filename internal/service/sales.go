package service

import (
	"time"

	"jk-api/internal/entity"

	"gorm.io/gorm"
)

// SalesStatus describes whether sales are currently open based on the configured
// trading window. Times are HH:MM in Asia/Bangkok.
type SalesStatus struct {
	Enabled   bool   `json:"enabled"`
	IsOpen    bool   `json:"is_open"`
	OpenTime  string `json:"open_time"`
	CloseTime string `json:"close_time"`
	Now       string `json:"now"`
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

// GetSalesStatus reads the sale-hours config and reports whether sales are open
// right now. When the feature is disabled, sales are always open.
func GetSalesStatus(db *gorm.DB) SalesStatus {
	enabled := configValue(db, "sales_hours_enabled", "true") == "true"
	open := configValue(db, "sales_open_time", "09:30")
	clse := configValue(db, "sales_close_time", "16:30")
	now := bangkokNow().Format("15:04")

	isOpen := true
	if enabled {
		isOpen = withinWindow(now, open, clse)
	}
	return SalesStatus{Enabled: enabled, IsOpen: isOpen, OpenTime: open, CloseTime: clse, Now: now}
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
