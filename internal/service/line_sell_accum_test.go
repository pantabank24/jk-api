package service

import (
	"testing"
	"time"

	"jk-api/internal/entity"

	"gorm.io/gorm"
)

// TestSellAccumMessageFormat pins the wording: it is forwarded to a counterparty
// as-is, so the shape of the line is the feature. Silver announces เงิน, not ทองคำ.
func TestSellAccumMessageFormat(t *testing.T) {
	at := time.Date(2025, 7, 31, 14, 35, 0, 0, time.FixedZone("ICT", 7*3600))

	got := sellAccumMessage("วีระชัย ชัยนุมาศ", at, "gold", "99.99", 1)
	want := "วีระชัย ชัยนุมาศ วันที่ 31/07/2568 เวลา 14:35 น. แจ้งขายทองคำ 99.99% จำนวน 1 กิโลกรัม"
	if got != want {
		t.Errorf("gold message =\n  %q\nwant\n  %q", got, want)
	}

	got = sellAccumMessage("วีระชัย ชัยนุมาศ", at, "silver", "99.9", 2)
	want = "วีระชัย ชัยนุมาศ วันที่ 31/07/2568 เวลา 14:35 น. แจ้งขายเงิน 99.9% จำนวน 2 กิโลกรัม"
	if got != want {
		t.Errorf("silver message =\n  %q\nwant\n  %q", got, want)
	}
}

// seedIssuedBill inserts a bill at รอตรวจบิล (11) with one item of the given metal.
func seedIssuedBill(t *testing.T, tx *gorm.DB, code, metal string, weight float64) *entity.Quotation {
	t.Helper()
	bill := entity.Quotation{Code: code, IsBill: true, Status: 11, Metal: metal}
	if err := tx.Create(&bill).Error; err != nil {
		t.Fatalf("seed bill %s: %v", code, err)
	}
	if err := tx.Create(&entity.QuotationItem{
		QuotationID: bill.ID, Metal: metal, Weight: weight, Total: weight * 1000,
	}).Error; err != nil {
		t.Fatalf("seed item for %s: %v", code, err)
	}
	return &bill
}

// enableSellAccum turns both metals' alerts on with a clean meter each.
func enableSellAccum(tx *gorm.DB, goldThreshold, silverThreshold string) {
	setConfigValue(tx, KeyLineEnabled, "true")
	setConfigValue(tx, KeyLineTargetID, "") // no push in tests; the meter is what's under test
	for metal, threshold := range map[string]string{"gold": goldThreshold, "silver": silverThreshold} {
		setConfigValue(tx, keySellAccumEnabled(metal), "true")
		setConfigValue(tx, keySellAccumThreshold(metal), threshold)
		setConfigValue(tx, keySellAccumWeight(metal), "0")
	}
}

func meter(tx *gorm.DB, metal string) float64 {
	for _, s := range GetSellAccumStatus(tx) {
		if s.Metal == metal {
			return s.Accumulated
		}
	}
	return -1
}

// TestSellAccumCarriesRemainder is the point of the meter: partial sell-ins add
// up across customers, and what's left after a lot is announced stays on for the
// next one instead of being thrown away.
func TestSellAccumCarriesRemainder(t *testing.T) {
	db := openTestDB(t, false)
	tx := db.Begin()
	defer tx.Rollback()

	enableSellAccum(tx, "65", "1000")

	AccumulateSellIn(tx, seedIssuedBill(t, tx, "T-SA-1", "gold", 30))
	if got := meter(tx, "gold"); got != 30 {
		t.Fatalf("meter after 30 บาท = %v, want 30", got)
	}

	// 30 + 40 = 70 → one lot out, 5 บาท carried.
	AccumulateSellIn(tx, seedIssuedBill(t, tx, "T-SA-2", "gold", 40))
	if got := meter(tx, "gold"); got != 5 {
		t.Fatalf("meter after the lot = %v, want 5 carried", got)
	}
}

// TestSellAccumMetersAreSeparate is what this split is for: ทอง and เงิน are
// weighed in different units, so a silver sell-in must not push the gold meter
// towards its next announcement (or the other way round).
func TestSellAccumMetersAreSeparate(t *testing.T) {
	db := openTestDB(t, false)
	tx := db.Begin()
	defer tx.Rollback()

	enableSellAccum(tx, "65", "1000")

	AccumulateSellIn(tx, seedIssuedBill(t, tx, "T-SA-SEP-G", "gold", 30))
	AccumulateSellIn(tx, seedIssuedBill(t, tx, "T-SA-SEP-S", "silver", 400))

	if got := meter(tx, "gold"); got != 30 {
		t.Errorf("gold meter = %v, want 30 (silver must not land on it)", got)
	}
	if got := meter(tx, "silver"); got != 400 {
		t.Errorf("silver meter = %v, want 400 (gold must not land on it)", got)
	}

	// Silver crosses its own 1000-กรัม lot; gold stays where it was.
	AccumulateSellIn(tx, seedIssuedBill(t, tx, "T-SA-SEP-S2", "silver", 650))
	if got := meter(tx, "silver"); got != 50 {
		t.Errorf("silver meter after its lot = %v, want 50 carried", got)
	}
	if got := meter(tx, "gold"); got != 30 {
		t.Errorf("gold meter moved on a silver announcement: %v", got)
	}
}

// TestSellAccumPerMetalSwitches — one metal switched off must not silence or
// stall the other.
func TestSellAccumPerMetalSwitches(t *testing.T) {
	db := openTestDB(t, false)
	tx := db.Begin()
	defer tx.Rollback()

	enableSellAccum(tx, "65", "1000")
	setConfigValue(tx, keySellAccumEnabled("gold"), "false")

	AccumulateSellIn(tx, seedIssuedBill(t, tx, "T-SA-SW-G", "gold", 30))
	AccumulateSellIn(tx, seedIssuedBill(t, tx, "T-SA-SW-S", "silver", 300))

	if got := meter(tx, "gold"); got != 0 {
		t.Errorf("gold meter moved while gold is switched off: %v", got)
	}
	if got := meter(tx, "silver"); got != 300 {
		t.Errorf("silver meter = %v, want 300 (gold's switch must not gate it)", got)
	}

	// The master switch gates both.
	setConfigValue(tx, KeyLineEnabled, "false")
	AccumulateSellIn(tx, seedIssuedBill(t, tx, "T-SA-SW-S2", "silver", 300))
	if got := meter(tx, "silver"); got != 300 {
		t.Errorf("silver meter moved while the master switch is off: %v", got)
	}
}

// TestSellAccumBigBillAnnouncesEveryLot — a 200-บาท bill is three lots, not one,
// and not three separate messages either (consumeSellAccum returns the count).
func TestSellAccumBigBillAnnouncesEveryLot(t *testing.T) {
	db := openTestDB(t, false)
	tx := db.Begin()
	defer tx.Rollback()

	enableSellAccum(tx, "65", "1000")

	lots, remainder, err := consumeSellAccum(tx, "gold", 200, 65)
	if err != nil {
		t.Fatalf("consume: %v", err)
	}
	if lots != 3 {
		t.Errorf("lots = %d, want 3", lots)
	}
	if remainder != 5 {
		t.Errorf("remainder = %v, want 5", remainder)
	}
}

// TestSellAccumThresholdZeroDisables — 0 means off for that metal alone.
func TestSellAccumThresholdZeroDisables(t *testing.T) {
	db := openTestDB(t, false)
	tx := db.Begin()
	defer tx.Rollback()

	enableSellAccum(tx, "0", "1000")

	AccumulateSellIn(tx, seedIssuedBill(t, tx, "T-SA-Z-G", "gold", 30))
	if got := meter(tx, "gold"); got != 0 {
		t.Errorf("gold meter moved with threshold 0: %v", got)
	}
	AccumulateSellIn(tx, seedIssuedBill(t, tx, "T-SA-Z-S", "silver", 30))
	if got := meter(tx, "silver"); got != 30 {
		t.Errorf("silver meter = %v, want 30 (gold's 0 must not disable it)", got)
	}
}

// TestSellAccumLegacyBillReadsAsGold — a row with no metal tag is gold
// (migration 85), the same reading every bill query uses.
func TestSellAccumLegacyBillReadsAsGold(t *testing.T) {
	db := openTestDB(t, false)
	tx := db.Begin()
	defer tx.Rollback()

	enableSellAccum(tx, "65", "1000")

	AccumulateSellIn(tx, seedIssuedBill(t, tx, "T-SA-LEG", "", 10))
	if got := meter(tx, "gold"); got != 10 {
		t.Fatalf("legacy untagged bill = %v, want 10 on the gold meter", got)
	}
}

// TestReleaseSellInUndoesAnIssue covers the re-issue path: a bill pulled back to
// รอออกบิล must leave the meter where it was, or issuing it again counts twice.
func TestReleaseSellInUndoesAnIssue(t *testing.T) {
	db := openTestDB(t, false)
	tx := db.Begin()
	defer tx.Rollback()

	enableSellAccum(tx, "65", "1000")

	bill := seedIssuedBill(t, tx, "T-SA-REV", "silver", 200)
	AccumulateSellIn(tx, bill)
	ReleaseSellIn(tx, bill)
	if got := meter(tx, "silver"); got != 0 {
		t.Fatalf("meter after revert = %v, want 0", got)
	}

	AccumulateSellIn(tx, bill)
	if got := meter(tx, "silver"); got != 200 {
		t.Fatalf("meter after re-issue = %v, want 200 (counted once)", got)
	}
}

// TestReleaseSellInFloorsAtZero — a release with no matching accumulation (the
// alert was switched on midway) must not drive the meter negative, which would
// silently swallow the next real sell-ins.
func TestReleaseSellInFloorsAtZero(t *testing.T) {
	db := openTestDB(t, false)
	tx := db.Begin()
	defer tx.Rollback()

	enableSellAccum(tx, "65", "1000")

	ReleaseSellIn(tx, seedIssuedBill(t, tx, "T-SA-NEG", "gold", 40))
	if got := meter(tx, "gold"); got != 0 {
		t.Fatalf("meter = %v, want 0 (never negative)", got)
	}
}

// TestSellAccumStatusReportsEachMetal feeds the settings page's two cards.
func TestSellAccumStatusReportsEachMetal(t *testing.T) {
	db := openTestDB(t, false)
	tx := db.Begin()
	defer tx.Rollback()

	enableSellAccum(tx, "65", "1000")
	setConfigValue(tx, keySellAccumWeight("gold"), "50")
	setConfigValue(tx, keySellAccumWeight("silver"), "250")
	setConfigValue(tx, keySellAccumPurity("gold"), "99.99")
	setConfigValue(tx, keySellAccumPurity("silver"), "99.9")

	byMetal := map[string]SellAccumStatus{}
	for _, s := range GetSellAccumStatus(tx) {
		byMetal[s.Metal] = s
	}
	if len(byMetal) != len(LineMetals) {
		t.Fatalf("status covers %d metals, want %d", len(byMetal), len(LineMetals))
	}

	gold := byMetal["gold"]
	if gold.Remaining != 15 || gold.Unit != "บาท" || gold.Purity != "99.99" || !gold.Enabled {
		t.Errorf("gold status = %+v", gold)
	}
	silver := byMetal["silver"]
	if silver.Remaining != 750 || silver.Unit != "กรัม" || silver.Purity != "99.9" {
		t.Errorf("silver status = %+v", silver)
	}
}
