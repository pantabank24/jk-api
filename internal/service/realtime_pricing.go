package service

import (
	"math"
	"strconv"
	"sync"
	"time"

	"gorm.io/gorm"
)

// bahtOzFactor converts a troy-ounce spot price into one Thai baht-weight of
// 96.5% gold. It is a physical constant, not a business decision, so it stays
// in code — an editable field here would only give someone a way to break every
// quote at once.
//
// The exact value is (15.244 g / 31.1035 g) × 0.965 = 0.472951. This keeps the
// sidecar's rounded 0.473 so moving the settings into the database does not
// move any price: the two differ by ~7 THB per baht-weight, which is a separate
// decision from where the config lives.
const bahtOzFactor = 0.473

// Config keys holding the shop's real-time pricing policy.
const (
	KeyRealtimePremium = "realtime_premium_thb"
	KeyRealtimeSpread  = "realtime_spread_thb"
)

// Accepted ranges. These are guard rails against a typo (30 → 3000) reaching
// live quotes, not opinions about what the shop should charge: anything inside
// them is the operator's call.
const (
	RealtimePremiumMin = -500.0
	RealtimePremiumMax = 500.0
	RealtimeSpreadMin  = 0.0
	RealtimeSpreadMax  = 1000.0
)

// Fallbacks when a row is missing or unparsable — the values that were compiled
// into the sidecar before this became configurable.
const (
	defaultRealtimePremium = -20.0
	defaultRealtimeSpread  = 80.0
)

// RealtimePricingTTL caps how long a loaded policy is reused before the table
// is read again. /gold-prices/realtime is polled every 1.5s per open page, so
// hitting the DB per request would be pure waste. Saving from the settings page
// clears the cache immediately, so an edit normally applies on the next poll;
// this ceiling only matters if a row is changed some other way (psql, restore).
const RealtimePricingTTL = 10 * time.Second

// RealtimePricing is the shop's positioning against the raw spot-derived mid.
type RealtimePricing struct {
	// Premium shifts the centre of the quote. Negative sits under the mid.
	Premium float64 `json:"premium_thb"`
	// Spread is the full gap between the buy and the sell price.
	Spread float64 `json:"spread_thb"`
}

var (
	rtPricingMu     sync.RWMutex
	rtPricingCache  *RealtimePricing
	rtPricingLoaded time.Time
)

// GetRealtimePricing returns the current policy, reading the config table at
// most once per RealtimePricingTTL.
func GetRealtimePricing(db *gorm.DB) RealtimePricing {
	rtPricingMu.RLock()
	if rtPricingCache != nil && time.Since(rtPricingLoaded) < RealtimePricingTTL {
		p := *rtPricingCache
		rtPricingMu.RUnlock()
		return p
	}
	rtPricingMu.RUnlock()

	p := RealtimePricing{
		Premium: configFloat(db, KeyRealtimePremium, defaultRealtimePremium),
		Spread:  configFloat(db, KeyRealtimeSpread, defaultRealtimeSpread),
	}
	// Clamp on read too: a value written straight into the DB never went past
	// the API's validation, and a wild premium must not reach a customer quote.
	p.Premium = clamp(p.Premium, RealtimePremiumMin, RealtimePremiumMax)
	p.Spread = clamp(p.Spread, RealtimeSpreadMin, RealtimeSpreadMax)

	rtPricingMu.Lock()
	rtPricingCache, rtPricingLoaded = &p, time.Now()
	rtPricingMu.Unlock()
	return p
}

// InvalidateRealtimePricing drops the cached policy so the next quote reloads
// it. Called when the settings page saves, which is what makes an edit look
// instant rather than taking up to RealtimePricingTTL.
func InvalidateRealtimePricing() {
	rtPricingMu.Lock()
	rtPricingCache = nil
	rtPricingMu.Unlock()
}

// Quote turns a spot price (USD/oz) and a USD/THB rate into the shop's prices
// for one baht-weight of 96.5% gold. Returns zeros when either input is
// missing, so callers can tell "no feed" apart from a real price.
func (p RealtimePricing) Quote(spot, usdthb float64) (mid, buy, sell float64) {
	if spot <= 0 || usdthb <= 0 {
		return 0, 0, 0
	}
	mid = spot*usdthb*bahtOzFactor + p.Premium
	return mid, math.Round(mid - p.Spread/2), math.Round(mid + p.Spread/2)
}

func configFloat(db *gorm.DB, key string, def float64) float64 {
	v, err := strconv.ParseFloat(configValue(db, key, ""), 64)
	if err != nil {
		return def
	}
	return v
}

func clamp(v, lo, hi float64) float64 {
	return math.Max(lo, math.Min(hi, v))
}
