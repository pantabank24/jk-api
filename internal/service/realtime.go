package service

import (
	"encoding/json"
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

type realtimePayload struct {
	BarBuy  *float64 `json:"bar_buy"`
	BarSell *float64 `json:"bar_sell"`
	Spot    *float64 `json:"spot"`
}

// SnapshotRealtimeRound fetches the current real-time gold price from the
// sidecar and persists it as a gold_prices row (source='realtime'), returning
// the round label and the new row ID so a quotation/bill can lock onto this
// exact price. Falls back to CurrentRound if the sidecar is unreachable.
func SnapshotRealtimeRound(db *gorm.DB) (string, *uint) {
	client := http.Client{Timeout: 4 * time.Second}
	resp, err := client.Get(realtimeURL() + "/xau/latest")
	if err != nil {
		return CurrentRound(db)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return CurrentRound(db)
	}
	var p realtimePayload
	if err := json.Unmarshal(body, &p); err != nil || p.BarBuy == nil || p.BarSell == nil {
		return CurrentRound(db)
	}

	now := bangkokNow()
	gp := entity.GoldPrice{
		BarBuy:       *p.BarBuy,
		BarSell:      *p.BarSell,
		OrnamentBuy:  *p.BarBuy,  // sidecar only derives bar pricing for now
		OrnamentSell: *p.BarSell,
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
