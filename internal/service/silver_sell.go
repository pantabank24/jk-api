package service

import (
	"encoding/json"
	"strconv"

	"gorm.io/gorm"
)

// parseSilverFloat parses a config numeric string, defaulting to 0.
func parseSilverFloat(s string) float64 {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return v
}

// SilverTier is one weight-based pricing rule for silver. Tiers are matched by
// the weight of the single sale (in kg): the first tier whose UpToKg covers the
// weight applies. UpToKg == nil is the catch-all (largest weights).
//   - Blocked: selling is not allowed in this tier.
//   - AddPerKg: baht added to the buy price per kg BEFORE the ÷1000 × % formula.
type SilverTier struct {
	UpToKg   *float64 `json:"up_to_kg"`
	AddPerKg float64  `json:"add_per_kg"`
	Blocked  bool     `json:"blocked"`
}

// SilverSellStatus describes whether customers may sell silver right now and the
// pricing rules to apply. Times are HH:MM in Asia/Bangkok. Silver has its own
// schedule, independent of the gold sales hours / bills_open switch.
type SilverSellStatus struct {
	Enabled   bool   `json:"enabled"`    // master switch (silver_sell_enabled)
	ShopOpen  bool   `json:"shop_open"`  // manual open/close (silver_shop_open) — the "ปิดร้าน" button
	IsOpen    bool   `json:"is_open"`    // enabled && shop_open && within the daily cutoff
	CloseTime string `json:"close_time"` // sell until this time; "" = no limit
	Now       string `json:"now"`
	// Pricing
	PriceMode   string       `json:"price_mode"`   // feed|manual
	ManualPrice float64      `json:"manual_price"` // buy price (baht/kg), used when price_mode=manual
	Tiers       []SilverTier `json:"tiers"`        // weight-based pricing rules
}

// parseSilverTiers decodes the silver_weight_tiers config JSON. On empty/invalid
// input it returns a single catch-all tier (any weight, base price) so pricing
// still works before any tier is configured.
func parseSilverTiers(raw string) []SilverTier {
	if raw != "" {
		var tiers []SilverTier
		if err := json.Unmarshal([]byte(raw), &tiers); err == nil && len(tiers) > 0 {
			return tiers
		}
	}
	return []SilverTier{{UpToKg: nil, AddPerKg: 0, Blocked: false}}
}

// GetSilverSellStatus resolves whether customers can sell silver right now from
// the master switch, the manual open/close flag, and the daily cutoff time, plus
// the pricing rules (source + weight tiers).
func GetSilverSellStatus(db *gorm.DB) SilverSellStatus {
	enabled := configValue(db, "silver_sell_enabled", "false") == "true"
	// Manual close ("ปิดร้าน") — default open so a fresh install with the feature
	// on isn't blocked by an unset flag.
	shopOpen := configValue(db, "silver_shop_open", "true") != "false"
	closeTime := configValue(db, "silver_sell_close_time", "")

	now := bangkokNow()
	nowHM := now.Format("15:04")

	// Empty cutoff means no time limit; otherwise open until closeTime.
	withinTime := closeTime == "" || nowHM <= closeTime
	isOpen := enabled && shopOpen && withinTime

	priceMode := configValue(db, "silver_price_mode", "feed")
	if priceMode != "manual" {
		priceMode = "feed"
	}
	manualPrice := parseSilverFloat(configValue(db, "silver_manual_price", "0"))
	tiers := parseSilverTiers(configValue(db, "silver_weight_tiers", ""))

	return SilverSellStatus{
		Enabled:     enabled,
		ShopOpen:    shopOpen,
		IsOpen:      isOpen,
		CloseTime:   closeTime,
		Now:         nowHM,
		PriceMode:   priceMode,
		ManualPrice: manualPrice,
		Tiers:       tiers,
	}
}
