package service

import (
	"math"
	"testing"

	"jk-api/internal/entity"
)

// The live types on prod as of 2026-09-17.
var (
	testGoldBar = &entity.GoldType{
		ID: 15, Name: "ทองคำแท่ง 96.5%", Metal: "gold", PriceSource: "bar_buy",
		FormulaSteps: `[{"operator":"/","operand_type":"number","value":15.2}]`,
	}
	testSilverBar = &entity.GoldType{
		ID: 23, Name: "เงินแท่ง", Metal: "silver", PriceSource: "buy", DefaultPercent: 99.9,
		FormulaSteps: `[{"operator":"/","operand_type":"number","value":1000},{"operator":"*","operand_type":"percent","value":0},{"operator":"/","operand_type":"number","value":100}]`,
	}
)

func kg(v float64) *float64 { return &v }

func TestCheckGoldWeight(t *testing.T) {
	cases := []struct {
		w       float64
		stepped bool
		ok      bool
	}{
		{5, true, true},
		{1000, true, true},
		{1, true, false},    // below the stepper floor
		{7, true, false},    // not a step of 5
		{1005, true, false}, // above the ceiling
		{1, false, true},
		{2, false, true},
		{1000, false, true},
		{0, false, false},
		{-5, false, false},
		{2.5, false, false}, // the typed field only takes whole numbers
		{1001, false, false},
	}
	for _, c := range cases {
		if got := checkGoldWeight(c.w, c.stepped) == ""; got != c.ok {
			t.Errorf("checkGoldWeight(%v, stepped=%v) ok=%v, want %v", c.w, c.stepped, got, c.ok)
		}
	}
}

func TestCheckSilverWeightAndPercent(t *testing.T) {
	if checkSilverWeight(999.99) == "" || checkSilverWeight(100000.01) == "" {
		t.Error("silver weight outside 1,000–100,000 g must be refused")
	}
	if checkSilverWeight(1000) != "" || checkSilverWeight(1500.5) != "" {
		t.Error("silver weight inside 1,000–100,000 g must pass")
	}
	if checkSilverPercent(99.9, 99.9) != "" || checkSilverPercent(50, 99.9) != "" {
		t.Error("purity up to the type default must pass")
	}
	for _, p := range []float64{0, -1, 99.91, 100, 150} {
		if checkSilverPercent(p, 99.9) == "" {
			t.Errorf("purity %v must be refused against a 99.9 cap", p)
		}
	}
}

func TestResolveSilverTier(t *testing.T) {
	// The prod configuration, deliberately out of order.
	tiers := []SilverTier{
		{UpToKg: nil, AddPerKg: 0},
		{UpToKg: kg(10), AddPerKg: 100},
		{UpToKg: kg(5), AddPerKg: 0},
	}
	if tier := ResolveSilverTier(tiers, 5); tier == nil || tier.AddPerKg != 0 || tier.UpToKg == nil || *tier.UpToKg != 5 {
		t.Errorf("5 kg should land in the ≤5 tier, got %+v", tier)
	}
	if tier := ResolveSilverTier(tiers, 7.5); tier == nil || tier.AddPerKg != 100 {
		t.Errorf("7.5 kg should land in the ≤10 tier, got %+v", tier)
	}
	if tier := ResolveSilverTier(tiers, 50); tier == nil || tier.UpToKg != nil {
		t.Errorf("50 kg should land in the catch-all, got %+v", tier)
	}

	noCatchAll := []SilverTier{{UpToKg: kg(5)}, {UpToKg: kg(10), Blocked: true}}
	if tier := ResolveSilverTier(noCatchAll, 11); tier != nil {
		t.Errorf("past every bound with no catch-all should be unsellable, got %+v", tier)
	}
	if tier := ResolveSilverTier(noCatchAll, 8); tier == nil || !tier.Blocked {
		t.Errorf("8 kg should land in the blocked tier, got %+v", tier)
	}
	if tier := ResolveSilverTier(nil, 99); tier == nil || tier.Blocked || tier.AddPerKg != 0 {
		t.Errorf("no tiers should sell any weight at base price, got %+v", tier)
	}
}

func TestLineAmountsMatchTheSellScreen(t *testing.T) {
	// A browser sell from 2026-09-16 22:21:46: per_gram 4491.973684210527.
	perGram, total := goldLineAmounts(testGoldBar, 68278, 1)
	if perGram != 4491.973684210527 || total != 68278 {
		t.Errorf("gold 68,278 × 1 = (%v, %v), want (4491.973684210527, 68278)", perGram, total)
	}
	if _, total := goldLineAmounts(testGoldBar, 68281, 25); total != 68281*25 {
		t.Errorf("gold total must be price × weight, got %v", total)
	}

	// BILL1753: 1,000 g at 66,500/kg and 99.9% came to 66,433.50.
	perGram, total = silverLineAmounts(testSilverBar, 66500, 0, 99.9, 1000)
	if math.Abs(total-66433.5) > 1e-6 || math.Abs(perGram-66.4335) > 1e-9 {
		t.Errorf("silver 1 kg = (%v, %v), want (66.4335, 66433.5)", perGram, total)
	}
	// The tier surcharge is added per kg before the formula.
	if _, total := silverLineAmounts(testSilverBar, 66500, 100, 99.9, 7000); math.Abs(total-66600*7*0.999) > 1e-6 {
		t.Errorf("silver 7 kg with +100/kg = %v, want %v", total, 66600*7*0.999)
	}
}

func TestGoldQuoteForSource(t *testing.T) {
	q := goldQuote{barBuy: 1, barSell: 2, ornamentBuy: 3, ornamentSell: 4}
	for src, want := range map[string]float64{
		"bar_buy": 1, "bar_sell": 2, "ornament_buy": 3, "ornament_sell": 4, "": 1, "unknown": 1,
	} {
		if got := q.forSource(src); got != want {
			t.Errorf("forSource(%q)=%v, want %v", src, got, want)
		}
	}
}
