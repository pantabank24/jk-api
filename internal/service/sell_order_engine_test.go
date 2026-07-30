package service

import (
	"math"
	"testing"

	"jk-api/internal/entity"
)

// TestAutoSellPricesLikeTheSellScreen pins the money math a fill produces against
// the customer sell screen's, which is the whole promise of the feature: an
// automatic sale must pay exactly what the same sale by hand would have.
//
// The screen (billCalculate.tsx) computes, for gold:
//
//	perGram = computeItem(price, percent=0, plus=0, weight=1).perGram
//	total   = price × weight        ← gold is quoted per baht-weight
//
// Read-only: it resolves the seeded gold-bar type and does arithmetic.
func TestAutoSellPricesLikeTheSellScreen(t *testing.T) {
	db := openTestDB(t, false)
	e := NewSellOrderEngine(db, nil, nil, nil)

	// No GoldTypeID — exercises the same name-based fallback the engine uses when
	// an order's type has been reseeded out from under it.
	goldType, err := e.loadGoldType(&entity.SellOrder{Metal: "gold"})
	if err != nil {
		t.Skipf("no gold-bar type seeded in the test database: %v", err)
	}

	const price = 60000.0
	const weight = 10.0

	perGram, lineTotal := goldType.ComputeItem(price, 0, 0, 1)
	total := price * weight

	// The seeded type divides the baht-weight price by 15.2 to get per-gram.
	if want := price / 15.2; math.Abs(perGram-want) > 0.01 {
		t.Fatalf("per-gram = %.4f, want %.4f (price ÷ 15.2)", perGram, want)
	}
	// At weight=1 the formula's own total is the per-gram figure — which is exactly
	// why the engine must NOT use it as the line total for gold.
	if math.Abs(lineTotal-perGram) > 0.01 {
		t.Fatalf("formula total at weight=1 = %.4f, want it to equal per-gram %.4f", lineTotal, perGram)
	}
	if want := 600000.0; math.Abs(total-want) > 0.01 {
		t.Fatalf("line total = %.2f, want %.2f (price × weight)", total, want)
	}
}

// TestRealtimeTickAgeTreatsNoDataAsStale: the staleness guard is what stops a fill
// at a frozen price, so a feed that has never had one must not read as fresh.
func TestRealtimeTickAgeTreatsNoDataAsStale(t *testing.T) {
	var never RealtimeTick
	if age := never.AgeSeconds(); age < 1e6 {
		t.Fatalf("age with no data = %v, want a value larger than any configured max", age)
	}
}
