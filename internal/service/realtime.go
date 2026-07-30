package service

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"time"

	"jk-api/internal/entity"

	"gorm.io/gorm"
)

// realtimeURL returns the tv-price-svc base URL (same default as config.Config).
func realtimeURL() string {
	if v := os.Getenv("GOLD_REALTIME_URL"); v != "" {
		return v
	}
	return "http://host.docker.internal:8000"
}

// realtimePayload is the part of the sidecar's response we price from. Its own
// bar_buy/bar_sell are deliberately ignored: the shop's premium and spread live
// in the config table so they can be retuned without redeploying the sidecar.
type realtimePayload struct {
	Spot      *float64 `json:"spot"`
	USDTHB    *float64 `json:"usdthb"`
	UpdatedAt *float64 `json:"updated_at"` // unix seconds, sidecar's clock
	Connected *bool    `json:"connected"`
}

// RealtimeTick is one reading of the market from the sidecar, with the freshness
// signals a caller needs to decide whether to trust it.
type RealtimeTick struct {
	Spot   float64
	USDTHB float64
	// UpdatedAt is when the sidecar last received a price. Zero when it never has.
	UpdatedAt time.Time
	// Connected reports the sidecar's own view of its TradingView socket. It keeps
	// serving its last value while reconnecting, so a stale price still looks like
	// a price — this is the flag that says otherwise.
	Connected bool
}

// AgeSeconds is how long ago the sidecar last received a price. Both clocks are
// the same docker host's wall clock, so the subtraction is meaningful. Returns a
// very large number when the sidecar has never had a price, so every staleness
// check treats "no data" as stale.
func (t RealtimeTick) AgeSeconds() float64 {
	if t.UpdatedAt.IsZero() {
		return 1e9
	}
	return time.Since(t.UpdatedAt).Seconds()
}

// FetchRealtimeTick reads the current market from the sidecar. It deliberately
// does NOT apply the shop's pricing policy — callers pair it with
// GetRealtimePricing so there is exactly one place that turns spot into a quote.
func FetchRealtimeTick() (RealtimeTick, error) {
	client := http.Client{Timeout: 4 * time.Second}
	resp, err := client.Get(realtimeURL() + "/xau/latest")
	if err != nil {
		return RealtimeTick{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return RealtimeTick{}, err
	}
	var p realtimePayload
	if err := json.Unmarshal(body, &p); err != nil {
		return RealtimeTick{}, err
	}
	if p.Spot == nil || p.USDTHB == nil {
		return RealtimeTick{}, errors.New("realtime feed has no price yet")
	}
	tick := RealtimeTick{Spot: *p.Spot, USDTHB: *p.USDTHB}
	if p.UpdatedAt != nil {
		sec := int64(*p.UpdatedAt)
		tick.UpdatedAt = time.Unix(sec, int64((*p.UpdatedAt-float64(sec))*1e9))
	}
	if p.Connected != nil {
		tick.Connected = *p.Connected
	}
	return tick, nil
}

// SaveRealtimeRound persists an already-priced real-time quote as a gold_prices
// row (source='realtime'), returning the round label and row ID so a document can
// lock onto this exact price. Split out from SnapshotRealtimeRound so a caller
// that already has the tick (the auto-sell engine, which must record the very
// price it matched against) does not fetch it a second time.
func SaveRealtimeRound(db *gorm.DB, buy, sell float64) (string, *uint) {
	if buy <= 0 || sell <= 0 {
		return CurrentRound(db)
	}
	now := bangkokNow()
	gp := entity.GoldPrice{
		BarBuy:       buy,
		BarSell:      sell,
		OrnamentBuy:  buy, // sidecar only derives bar pricing for now
		OrnamentSell: sell,
		GoldDate:     now.Format("2006-01-02"),
		GoldTime:     now.Format("15:04:05"),
		GoldRound:    "realtime",
		Source:       "realtime",
		CreatedAt:    time.Now(),
	}
	if err := db.Create(&gp).Error; err != nil {
		return CurrentRound(db)
	}
	id := gp.ID
	return "realtime", &id
}

// SnapshotRealtimeRound fetches the current real-time gold price from the
// sidecar and persists it as a gold_prices row (source='realtime'), returning
// the round label and the new row ID so a quotation/bill can lock onto this
// exact price. Falls back to CurrentRound if the sidecar is unreachable.
func SnapshotRealtimeRound(db *gorm.DB) (string, *uint) {
	tick, err := FetchRealtimeTick()
	if err != nil {
		return CurrentRound(db)
	}
	// Price the snapshot through the same policy the live screen shows, so the
	// number locked onto the document is the one the customer was quoted.
	_, buy, sell := GetRealtimePricing(db).Quote(tick.Spot, tick.USDTHB)
	return SaveRealtimeRound(db, buy, sell)
}
