package service

import (
	"testing"
	"time"

	"jk-api/internal/entity"

	"gorm.io/gorm"
)

// TestPendingSellMessageFormat pins the wording: it is forwarded to a
// counterparty as-is, so the shape of the line is the feature. Silver announces
// เงิน, not ทองคำ — and the quantity is always 1 กิโลกรัม, because the shop sells
// this on one kilo at a time (a bigger pile sends more notices, not a bigger
// number inside one).
func TestPendingSellMessageFormat(t *testing.T) {
	at := time.Date(2025, 7, 31, 14, 35, 0, 0, time.FixedZone("ICT", 7*3600))

	got := pendingSellMessage("วีรชัย ชัยนุมาศ", at, "gold", "99.99")
	want := "วีรชัย ชัยนุมาศ วันที่ 31/07/2568 เวลา 14:35 น. แจ้งขายทองคำ 99.99% จำนวน 1 กิโลกรัม"
	if got != want {
		t.Errorf("gold message =\n  %q\nwant\n  %q", got, want)
	}

	got = pendingSellMessage("วีรชัย ชัยนุมาศ", at, "silver", "99.9")
	want = "วีรชัย ชัยนุมาศ วันที่ 31/07/2568 เวลา 14:35 น. แจ้งขายเงิน 99.9% จำนวน 1 กิโลกรัม"
	if got != want {
		t.Errorf("silver message =\n  %q\nwant\n  %q", got, want)
	}
}

// TestPendingSellNameIsConfigurable — the signature is the shop's, editable on
// the settings page, and falls back to the owner's name when nothing is stored.
func TestPendingSellNameIsConfigurable(t *testing.T) {
	db := openTestDB(t, false)
	tx := db.Begin()
	defer tx.Rollback()

	setConfigValue(tx, KeyPendingSellName, "")
	if got := PendingSellName(tx); got != defaultPendingSellName {
		t.Errorf("unset name = %q, want the default %q", got, defaultPendingSellName)
	}

	SetPendingSellName(tx, "  สมชาย ใจดี  ")
	if got := PendingSellName(tx); got != "สมชาย ใจดี" {
		t.Errorf("name = %q, want the trimmed %q", got, "สมชาย ใจดี")
	}

	// Blank is ignored rather than saved — an unsigned message would go out
	// reading " วันที่ ...".
	SetPendingSellName(tx, "   ")
	if got := PendingSellName(tx); got != "สมชาย ใจดี" {
		t.Errorf("name after a blank save = %q, want it unchanged", got)
	}
}

// seedCustomer creates a user to own the pending bills.
func seedCustomer(t *testing.T, tx *gorm.DB, name string) uint {
	t.Helper()
	u := entity.User{Name: name, Email: name + "@test.local", Password: "x"}
	if err := tx.Create(&u).Error; err != nil {
		t.Fatalf("seed user %s: %v", name, err)
	}
	return u.ID
}

// seedPendingBill inserts a bill at รอออกบิล (10) owned by userID with one item.
func seedPendingBill(t *testing.T, tx *gorm.DB, code string, userID uint, metal string, weight float64) *entity.Quotation {
	t.Helper()
	owner := userID
	bill := entity.Quotation{Code: code, IsBill: true, Status: 10, Metal: metal, CreatedBy: &owner}
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

func enablePendingSell(t *testing.T, tx *gorm.DB, goldThreshold, silverThreshold string) {
	t.Helper()
	setConfigValue(tx, KeyLineEnabled, "true")
	setConfigValue(tx, KeyLineTargetID, "") // no push in tests; the latch is what's under test
	for metal, threshold := range map[string]string{"gold": goldThreshold, "silver": silverThreshold} {
		setConfigValue(tx, keyPendingSellEnabled(metal), "true")
		setConfigValue(tx, keyPendingSellThreshold(metal), threshold)
	}

	// The pile is the whole shop's now, so anything the test database already has
	// at รอออกบิล would be summed into every assertion below. Soft-delete it out
	// of the way — rolled back with the rest of the transaction.
	if err := tx.Where("is_bill = ? AND status = ?", true, billStatusPendingIssue).
		Delete(&entity.Quotation{}).Error; err != nil {
		t.Fatalf("isolate the existing รอออกบิล pile: %v", err)
	}
	if err := tx.Exec("DELETE FROM line_pending_sell_alerts").Error; err != nil {
		t.Fatalf("reset the latch: %v", err)
	}
}

// lots reads back the latch — how many kilos have been announced for this pile.
func lots(t *testing.T, tx *gorm.DB, metal string) int {
	t.Helper()
	var row entity.LinePendingSellAlert
	if err := tx.Where("metal = ?", metal).First(&row).Error; err != nil {
		return 0
	}
	return row.AlertedLots
}

// TestPendingSellFiresOncePerKilo is the whole rule: the pile crosses 65 บาท and
// gets one message, keeps growing below the next kilo and gets none, then crosses
// 130 and gets a second.
func TestPendingSellFiresOncePerKilo(t *testing.T) {
	db := openTestDB(t, false)
	tx := db.Begin()
	defer tx.Rollback()

	enablePendingSell(t, tx, "65", "1000")
	cust := seedCustomer(t, tx, "pending-sell-1")

	// 40 บาท — below the line, nothing announced.
	seedPendingBill(t, tx, "T-PS-1", cust, "gold", 40)
	SyncPendingSellAlert(tx, "gold")
	if got := lots(t, tx, "gold"); got != 0 {
		t.Fatalf("announced %d kilo(s) at 40 บาท, want 0", got)
	}

	// 40 + 30 = 70 → over 65, one kilo announced.
	seedPendingBill(t, tx, "T-PS-2", cust, "gold", 30)
	SyncPendingSellAlert(tx, "gold")
	if got := lots(t, tx, "gold"); got != 1 {
		t.Fatalf("announced %d kilo(s) at 70 บาท, want 1", got)
	}

	// 70 + 20 = 90 → still the same kilo, stays quiet.
	seedPendingBill(t, tx, "T-PS-3", cust, "gold", 20)
	SyncPendingSellAlert(tx, "gold")
	if got := lots(t, tx, "gold"); got != 1 {
		t.Fatalf("announced %d kilo(s) at 90 บาท, want still 1", got)
	}

	// 90 + 45 = 135 → second kilo, announced again.
	seedPendingBill(t, tx, "T-PS-4", cust, "gold", 45)
	SyncPendingSellAlert(tx, "gold")
	if got := lots(t, tx, "gold"); got != 2 {
		t.Fatalf("announced %d kilo(s) at 135 บาท, want 2", got)
	}
}

// TestPendingSellFiresAtExactlyTheThreshold pins the boundary the shop cares
// about: 65.00 บาท on the nose alerts (>=, not >), and a hair under does not.
// Weights are decimal(12,4), so this also covers the rounding epsilon.
func TestPendingSellFiresAtExactlyTheThreshold(t *testing.T) {
	db := openTestDB(t, false)
	tx := db.Begin()
	defer tx.Rollback()

	enablePendingSell(t, tx, "65", "1000")
	cust := seedCustomer(t, tx, "pending-sell-boundary")

	seedPendingBill(t, tx, "T-PS-64", cust, "gold", 64.9999)
	SyncPendingSellAlert(tx, "gold")
	if got := lots(t, tx, "gold"); got != 0 {
		t.Errorf("64.9999 บาท announced %d kilo(s), want 0", got)
	}

	// The last 0.0001 lands, reaching exactly 65 by summing — the real path.
	seedPendingBill(t, tx, "T-PS-65", cust, "gold", 0.0001)
	SyncPendingSellAlert(tx, "gold")
	if got := lots(t, tx, "gold"); got != 1 {
		t.Errorf("64.9999 + 0.0001 = 65 บาท announced %d kilo(s), want 1", got)
	}
}

// TestPendingSellCountsEveryCustomerTogether — this is the correction that
// replaced the per-customer rule: two customers at 40 บาท each IS a kilo, because
// what the shop forwards is the metal, not the person who brought it.
func TestPendingSellCountsEveryCustomerTogether(t *testing.T) {
	db := openTestDB(t, false)
	tx := db.Begin()
	defer tx.Rollback()

	enablePendingSell(t, tx, "65", "1000")
	a := seedCustomer(t, tx, "pending-sell-a")
	b := seedCustomer(t, tx, "pending-sell-b")

	seedPendingBill(t, tx, "T-PS-A", a, "gold", 40)
	seedPendingBill(t, tx, "T-PS-B", b, "gold", 40)

	if w := PendingSellWeight(tx, "gold"); w != 80 {
		t.Errorf("shop pile = %v, want 80 across both customers", w)
	}
	SyncPendingSellAlert(tx, "gold")
	if got := lots(t, tx, "gold"); got != 1 {
		t.Errorf("announced %d, want 1 — 40+40 across customers is a kilo", got)
	}
}

// TestPendingSellIssuingClearsThePile — ออกบิล takes the weight out of รอออกบิล,
// so the alert re-arms for whatever gets sold in next.
func TestPendingSellIssuingClearsThePile(t *testing.T) {
	db := openTestDB(t, false)
	tx := db.Begin()
	defer tx.Rollback()

	enablePendingSell(t, tx, "65", "1000")
	cust := seedCustomer(t, tx, "pending-sell-issue")

	bill := seedPendingBill(t, tx, "T-PS-ISS", cust, "gold", 70)
	SyncPendingSellAlert(tx, "gold")
	if got := lots(t, tx, "gold"); got != 1 {
		t.Fatalf("announced %d, want 1", got)
	}

	// Issue it → status 11, out of the pile.
	if err := tx.Model(&entity.Quotation{}).Where("id = ?", bill.ID).Update("status", 11).Error; err != nil {
		t.Fatalf("issue: %v", err)
	}
	SyncPendingSellAlert(tx, "gold")
	if w := PendingSellWeight(tx, "gold"); w != 0 {
		t.Fatalf("pile after ออกบิล = %v, want 0", w)
	}
	if got := lots(t, tx, "gold"); got != 0 {
		t.Fatalf("latch after ออกบิล = %d, want 0 (re-armed)", got)
	}

	// The next pile announces again from scratch.
	seedPendingBill(t, tx, "T-PS-ISS2", cust, "gold", 65)
	SyncPendingSellAlert(tx, "gold")
	if got := lots(t, tx, "gold"); got != 1 {
		t.Fatalf("second pile announced %d, want 1", got)
	}
}

// TestPendingSellMetalsAreSeparate — ทอง and เงิน are weighed in different units,
// so silver must not push gold over the line.
func TestPendingSellMetalsAreSeparate(t *testing.T) {
	db := openTestDB(t, false)
	tx := db.Begin()
	defer tx.Rollback()

	enablePendingSell(t, tx, "65", "1000")
	cust := seedCustomer(t, tx, "pending-sell-metals")

	seedPendingBill(t, tx, "T-PS-MG", cust, "gold", 40)
	seedPendingBill(t, tx, "T-PS-MS", cust, "silver", 1200)
	SyncPendingSellAlert(tx, "gold")
	SyncPendingSellAlert(tx, "silver")

	if got := lots(t, tx, "gold"); got != 0 {
		t.Errorf("gold announced %d, want 0 (silver must not count towards it)", got)
	}
	if got := lots(t, tx, "silver"); got != 1 {
		t.Errorf("silver announced %d, want 1", got)
	}
}

// TestPendingSellSumsEveryPendingBill — an auto-sell fill opens its own bill
// alongside the one being added to by hand, and both are metal the shop holds.
func TestPendingSellSumsEveryPendingBill(t *testing.T) {
	db := openTestDB(t, false)
	tx := db.Begin()
	defer tx.Rollback()

	enablePendingSell(t, tx, "65", "1000")
	cust := seedCustomer(t, tx, "pending-sell-multi")

	seedPendingBill(t, tx, "T-PS-X1", cust, "gold", 30)
	seedPendingBill(t, tx, "T-PS-X2", cust, "gold", 40) // e.g. an auto-sell fill
	if w := PendingSellWeight(tx, "gold"); w != 70 {
		t.Fatalf("pile = %v, want 70 across both bills", w)
	}
	SyncPendingSellAlert(tx, "gold")
	if got := lots(t, tx, "gold"); got != 1 {
		t.Fatalf("announced %d, want 1", got)
	}
}

// TestPendingSellThresholdZeroDisables — 0 means off for that metal alone, and
// it must not write a latch either.
func TestPendingSellThresholdZeroDisables(t *testing.T) {
	db := openTestDB(t, false)
	tx := db.Begin()
	defer tx.Rollback()

	enablePendingSell(t, tx, "0", "1000")
	cust := seedCustomer(t, tx, "pending-sell-zero")

	seedPendingBill(t, tx, "T-PS-Z", cust, "gold", 500)
	SyncPendingSellAlert(tx, "gold")
	if got := lots(t, tx, "gold"); got != 0 {
		t.Fatalf("latch written while the rule is off: %d", got)
	}
}

// TestPendingSellLegacyBillReadsAsGold — a row with no metal tag is gold
// (migration 85), the same reading every bill query uses.
func TestPendingSellLegacyBillReadsAsGold(t *testing.T) {
	db := openTestDB(t, false)
	tx := db.Begin()
	defer tx.Rollback()

	enablePendingSell(t, tx, "65", "1000")
	cust := seedCustomer(t, tx, "pending-sell-legacy")

	seedPendingBill(t, tx, "T-PS-LEG", cust, "", 70)
	if w := PendingSellWeight(tx, "gold"); w != 70 {
		t.Fatalf("legacy untagged bill = %v, want 70 on gold", w)
	}
}

// TestPendingSellStatusReportsThePile feeds the settings page: the shop-wide
// weight, how many bills make it up, and how much of it has been announced.
func TestPendingSellStatusReportsThePile(t *testing.T) {
	db := openTestDB(t, false)
	tx := db.Begin()
	defer tx.Rollback()

	enablePendingSell(t, tx, "65", "1000")
	a := seedCustomer(t, tx, "pending-sell-status-a")
	b := seedCustomer(t, tx, "pending-sell-status-b")
	seedPendingBill(t, tx, "T-PS-ST1", a, "gold", 120)
	seedPendingBill(t, tx, "T-PS-ST2", b, "gold", 10)
	SyncPendingSellAlert(tx, "gold")

	var gold PendingSellStatus
	for _, s := range GetPendingSellStatus(tx) {
		if s.Metal == "gold" {
			gold = s
		}
	}
	if gold.Weight != 130 {
		t.Errorf("weight = %v, want 130 across both customers", gold.Weight)
	}
	if gold.BillCount != 2 {
		t.Errorf("bill_count = %d, want 2", gold.BillCount)
	}
	if gold.Lots != 2 || gold.AlertedLots != 2 {
		t.Errorf("lots = %d / alerted = %d, want 2 / 2", gold.Lots, gold.AlertedLots)
	}
	if gold.Unit != "บาท" || gold.Threshold != 65 {
		t.Errorf("gold status = %+v", gold)
	}
}
