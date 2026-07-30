package service

import (
	"strconv"

	"gorm.io/gorm"
)

// Config keys for the auto-sell (limit-order) feature.
const (
	KeyAutoSellEnabled         = "auto_sell_enabled"
	KeyAutoSellIgnoreHours     = "auto_sell_ignore_hours"
	KeyAutoSellMaxSlippage     = "auto_sell_max_slippage_thb"
	KeyAutoSellMaxActiveOrders = "auto_sell_max_active_orders"
	KeyAutoSellMaxActiveWeight = "auto_sell_max_active_weight"
	KeyAutoSellMaxFeedAge      = "auto_sell_max_feed_age_sec"
	KeyAutoSellTickSeconds     = "auto_sell_tick_seconds"
)

// Guard rails for the editable values. They exist so a typo (5 → 500) cannot put
// the engine somewhere unrecoverable; anything inside them is the shop's call.
const (
	AutoSellMaxSlippageMin     = 0.0
	AutoSellMaxSlippageMax     = 10000.0
	AutoSellMaxActiveOrdersMin = 1
	AutoSellMaxActiveOrdersMax = 100
	AutoSellMaxActiveWeightMin = 1.0
	AutoSellMaxActiveWeightMax = 100000.0
	AutoSellMaxFeedAgeMin      = 3
	AutoSellMaxFeedAgeMax      = 600
	AutoSellTickSecondsMin     = 2
	AutoSellTickSecondsMax     = 300
)

// Weight bounds for one order, matching the customer sell screen's stepper
// (WEIGHT_MIN / WEIGHT_MAX in billCalculate.tsx). Weight is in บาททอง.
const (
	AutoSellWeightMin  = 1.0
	AutoSellWeightMax  = 1000.0
	AutoSellWeightStep = 5.0
)

// AutoSellConfig is the engine's editable policy, read fresh from the config
// table (no caching: the engine reads it once per tick, which is already the
// slowest thing it does, and an edit must take effect on the very next tick).
type AutoSellConfig struct {
	Enabled         bool    `json:"enabled"`
	IgnoreHours     bool    `json:"ignore_hours"`
	MaxSlippageTHB  float64 `json:"max_slippage_thb"` // 0 = no limit
	MaxActiveOrders int     `json:"max_active_orders"`
	MaxActiveWeight float64 `json:"max_active_weight"`
	MaxFeedAgeSec   int     `json:"max_feed_age_sec"`
	TickSeconds     int     `json:"tick_seconds"`
}

func configInt(db *gorm.DB, key string, def int) int {
	v, err := strconv.Atoi(configValue(db, key, ""))
	if err != nil {
		return def
	}
	return v
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// GetAutoSellConfig loads the policy, clamping every value: a row written
// straight into the database never passed the API's validation, and the engine
// must not act on a nonsense tick interval or an unbounded cap.
func GetAutoSellConfig(db *gorm.DB) AutoSellConfig {
	return AutoSellConfig{
		Enabled:     configValue(db, KeyAutoSellEnabled, "false") == "true",
		IgnoreHours: configValue(db, KeyAutoSellIgnoreHours, "true") == "true",
		MaxSlippageTHB: clamp(configFloat(db, KeyAutoSellMaxSlippage, 0),
			AutoSellMaxSlippageMin, AutoSellMaxSlippageMax),
		MaxActiveOrders: clampInt(configInt(db, KeyAutoSellMaxActiveOrders, 5),
			AutoSellMaxActiveOrdersMin, AutoSellMaxActiveOrdersMax),
		MaxActiveWeight: clamp(configFloat(db, KeyAutoSellMaxActiveWeight, 200),
			AutoSellMaxActiveWeightMin, AutoSellMaxActiveWeightMax),
		MaxFeedAgeSec: clampInt(configInt(db, KeyAutoSellMaxFeedAge, 15),
			AutoSellMaxFeedAgeMin, AutoSellMaxFeedAgeMax),
		TickSeconds: clampInt(configInt(db, KeyAutoSellTickSeconds, 5),
			AutoSellTickSecondsMin, AutoSellTickSecondsMax),
	}
}

// AutoSellStatus is what the customer's screen needs to explain the feature's
// current state: whether orders can be placed at all, whether one would fire
// right now, and the caps their next order is measured against.
type AutoSellStatus struct {
	AutoSellConfig
	// CanFireNow is true when an order whose target the price reached would be
	// filled at this moment (feed live, shop buying, hours satisfied).
	CanFireNow bool `json:"can_fire_now"`
	// BlockedReason explains a false CanFireNow in Thai, ready to display.
	BlockedReason string `json:"blocked_reason"`
	// Price is the live buy price targets are compared against; nil with no feed.
	Price *float64 `json:"price"`
	// PremiumTHB/SpreadTHB are the policy behind Price, so the screen can show
	// what the target is actually measured against.
	PremiumTHB float64 `json:"premium_thb"`
	SpreadTHB  float64 `json:"spread_thb"`
	// WeightMin/WeightMax/WeightStep mirror the sell screen's stepper. Step is 0
	// when the customer may type any whole weight right now.
	WeightMin  float64 `json:"weight_min"`
	WeightMax  float64 `json:"weight_max"`
	WeightStep float64 `json:"weight_step"`
}

// GetAutoSellStatus resolves everything the auto-sell screen displays, reusing
// the exact gates the engine applies so the screen can never promise a fill the
// engine would refuse.
func GetAutoSellStatus(db *gorm.DB) AutoSellStatus {
	cfg := GetAutoSellConfig(db)
	st := AutoSellStatus{
		AutoSellConfig: cfg,
		WeightMin:      AutoSellWeightMin,
		WeightMax:      AutoSellWeightMax,
		WeightStep:     AutoSellWeightStep,
	}
	if GetCustomWeightStatus(db).Allowed {
		st.WeightStep = 0 // any whole weight
	} else {
		st.WeightMin = AutoSellWeightStep
	}

	pricing := GetRealtimePricing(db)
	st.PremiumTHB, st.SpreadTHB = pricing.Premium, pricing.Spread

	// One feed read serves both the displayed price and the gate — the sidecar is
	// an HTTP hop, and this endpoint is polled by every open screen.
	tick, tickErr := FetchRealtimeTick()
	if tickErr == nil {
		if _, buy, _ := pricing.Quote(tick.Spot, tick.USDTHB); buy > 0 {
			st.Price = &buy
		}
	}

	st.BlockedReason = autoSellGate(db, cfg, tick, tickErr)
	st.CanFireNow = st.BlockedReason == ""
	return st
}

// autoSellGate returns the reason auto-sell cannot fill right now, or "" when it
// can. Single source of truth for both the engine and the status endpoint, so the
// screen can never promise a fill the engine would refuse. The tick is passed in
// because both callers already have one.
func autoSellGate(db *gorm.DB, cfg AutoSellConfig, tick RealtimeTick, tickErr error) string {
	if !cfg.Enabled {
		return "ระบบขายอัตโนมัติปิดอยู่"
	}
	// "ปิดรับซื้อทอง" is a deliberate manual stop, not a schedule — it applies
	// even when the shop has told the engine to ignore trading hours.
	if configValue(db, "bills_open", "true") == "false" {
		return "ขณะนี้ร้านปิดรับซื้อทอง"
	}
	if tickErr != nil {
		return "เชื่อมต่อราคาเรียลไทม์ไม่ได้"
	}
	if !tick.Connected {
		return "ราคาเรียลไทม์หลุดการเชื่อมต่อ"
	}
	if age := tick.AgeSeconds(); age > float64(cfg.MaxFeedAgeSec) {
		return "ราคาเรียลไทม์ค้าง (ไม่อัปเดต " + strconv.Itoa(int(age)) + " วินาที)"
	}
	if !cfg.IgnoreHours {
		if sales := GetSalesStatus(db); sales.PriceMode != PriceModeRealtime {
			return "อยู่นอกเวลาที่ขายด้วยราคาเรียลไทม์"
		}
	}
	return ""
}
