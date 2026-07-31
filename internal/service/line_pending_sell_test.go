package service

import (
	"testing"
	"time"

	"jk-api/internal/entity"

	"gorm.io/gorm"
)

// TestPendingSellMessageFormat pins the wording: it is forwarded to a
// counterparty as-is, so the shape of the line is the feature. Silver announces
// เงิน, not ทองคำ.
func TestPendingSellMessageFormat(t *testing.T) {
	at := time.Date(2025, 7, 31, 14, 35, 0, 0, time.FixedZone("ICT", 7*3600))

	got := pendingSellMessage("วีระชัย ชัยนุมาศ", at, "gold", "99.99", 1)
	want := "วีระชัย ชัยนุมาศ วันที่ 31/07/2568 เวลา 14:35 น. แจ้งขายทองคำ 99.99% จำนวน 1 กิโลกรัม"
	if got != want {
		t.Errorf("gold message =\n  %q\nwant\n  %q", got, want)
	}

	got = pendingSellMessage("วีระชัย ชัยนุมาศ", at, "silver", "99.9", 2)
	want = "วีระชัย ชัยนุมาศ วันที่ 31/07/2568 เวลา 14:35 น. แจ้งขายเงิน 99.9% จำนวน 2 กิโลกรัม"
	if got != want {
		t.Errorf("silver message =\n  %q\nwant\n  %q", got, want)
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

func enablePendingSell(tx *gorm.DB, goldThreshold, silverThreshold string) {
	setConfigValue(tx, KeyLineEnabled, "true")
	setConfigValue(tx, KeyLineTargetID, "") // no push in tests; the latch is what's under test
	for metal, threshold := range map[string]string{"gold": goldThreshold, "silver": silverThreshold} {
		setConfigValue(tx, keyPendingSellEnabled(metal), "true")
		setConfigValue(tx, keyPendingSellThreshold(metal), threshold)
	}
}

// lots reads back the latch — how many kilos have been announced for this pile.
func lots(t *testing.T, tx *gorm.DB, userID uint, metal string) int {
	t.Helper()
	var row entity.LinePendingSellAlert
	if err := tx.Where("user_id = ? AND metal = ?", userID, metal).First(&row).Error; err != nil {
		return 0
	}
	return row.AlertedLots
}

// TestPendingSellFiresOncePerKilo is the whole rule: the customer crosses 65 บาท
// and gets one message, keeps selling below the next kilo and gets none, then
// crosses 130 and gets a second.
func TestPendingSellFiresOncePerKilo(t *testing.T) {
	db := openTestDB(t, false)
	tx := db.Begin()
	defer tx.Rollback()

	enablePendingSell(tx, "65", "1000")
	cust := seedCustomer(t, tx, "pending-sell-1")

	// 40 บาท — below the line, nothing announced.
	seedPendingBill(t, tx, "T-PS-1", cust, "gold", 40)
	SyncPendingSellAlert(tx, cust, "gold")
	if got := lots(t, tx, cust, "gold"); got != 0 {
		t.Fatalf("announced %d kilo(s) at 40 บาท, want 0", got)
	}

	// 40 + 30 = 70 → over 65, one kilo announced.
	seedPendingBill(t, tx, "T-PS-2", cust, "gold", 30)
	SyncPendingSellAlert(tx, cust, "gold")
	if got := lots(t, tx, cust, "gold"); got != 1 {
		t.Fatalf("announced %d kilo(s) at 70 บาท, want 1", got)
	}

	// 70 + 20 = 90 → still the same kilo, stays quiet.
	seedPendingBill(t, tx, "T-PS-3", cust, "gold", 20)
	SyncPendingSellAlert(tx, cust, "gold")
	if got := lots(t, tx, cust, "gold"); got != 1 {
		t.Fatalf("announced %d kilo(s) at 90 บาท, want still 1", got)
	}

	// 90 + 45 = 135 → second kilo, announced again.
	seedPendingBill(t, tx, "T-PS-4", cust, "gold", 45)
	SyncPendingSellAlert(tx, cust, "gold")
	if got := lots(t, tx, cust, "gold"); got != 2 {
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

	enablePendingSell(tx, "65", "1000")

	under := seedCustomer(t, tx, "pending-sell-64")
	seedPendingBill(t, tx, "T-PS-64", under, "gold", 64.9999)
	SyncPendingSellAlert(tx, under, "gold")
	if got := lots(t, tx, under, "gold"); got != 0 {
		t.Errorf("64.9999 บาท announced %d kilo(s), want 0", got)
	}

	// Split across two sells, so the 65 is reached by summing — the real path.
	exact := seedCustomer(t, tx, "pending-sell-65")
	seedPendingBill(t, tx, "T-PS-65A", exact, "gold", 60)
	seedPendingBill(t, tx, "T-PS-65B", exact, "gold", 5)
	SyncPendingSellAlert(tx, exact, "gold")
	if got := lots(t, tx, exact, "gold"); got != 1 {
		t.Errorf("60 + 5 = 65 บาท announced %d kilo(s), want 1", got)
	}
}

// TestPendingSellIsPerCustomer — this is the correction that replaced the
// shop-wide meter: two customers at 40 บาท each is NOT a kilo.
func TestPendingSellIsPerCustomer(t *testing.T) {
	db := openTestDB(t, false)
	tx := db.Begin()
	defer tx.Rollback()

	enablePendingSell(tx, "65", "1000")
	a := seedCustomer(t, tx, "pending-sell-a")
	b := seedCustomer(t, tx, "pending-sell-b")

	seedPendingBill(t, tx, "T-PS-A", a, "gold", 40)
	seedPendingBill(t, tx, "T-PS-B", b, "gold", 40)
	SyncPendingSellAlert(tx, a, "gold")
	SyncPendingSellAlert(tx, b, "gold")

	if got := lots(t, tx, a, "gold"); got != 0 {
		t.Errorf("customer A announced %d, want 0 — 40+40 across customers is not a kilo", got)
	}
	if got := lots(t, tx, b, "gold"); got != 0 {
		t.Errorf("customer B announced %d, want 0", got)
	}
	if w := PendingSellWeight(tx, a, "gold"); w != 40 {
		t.Errorf("customer A pile = %v, want 40 (B's must not be counted in)", w)
	}
}

// TestPendingSellIssuingClearsThePile — ออกบิล takes the weight out of รอออกบิล,
// so the alert re-arms for whatever the customer sells next.
func TestPendingSellIssuingClearsThePile(t *testing.T) {
	db := openTestDB(t, false)
	tx := db.Begin()
	defer tx.Rollback()

	enablePendingSell(tx, "65", "1000")
	cust := seedCustomer(t, tx, "pending-sell-issue")

	bill := seedPendingBill(t, tx, "T-PS-ISS", cust, "gold", 70)
	SyncPendingSellAlert(tx, cust, "gold")
	if got := lots(t, tx, cust, "gold"); got != 1 {
		t.Fatalf("announced %d, want 1", got)
	}

	// Issue it → status 11, out of the pile.
	if err := tx.Model(&entity.Quotation{}).Where("id = ?", bill.ID).Update("status", 11).Error; err != nil {
		t.Fatalf("issue: %v", err)
	}
	SyncPendingSellAlert(tx, cust, "gold")
	if w := PendingSellWeight(tx, cust, "gold"); w != 0 {
		t.Fatalf("pile after ออกบิล = %v, want 0", w)
	}
	if got := lots(t, tx, cust, "gold"); got != 0 {
		t.Fatalf("latch after ออกบิล = %d, want 0 (re-armed)", got)
	}

	// The next pile announces again from scratch.
	seedPendingBill(t, tx, "T-PS-ISS2", cust, "gold", 65)
	SyncPendingSellAlert(tx, cust, "gold")
	if got := lots(t, tx, cust, "gold"); got != 1 {
		t.Fatalf("second pile announced %d, want 1", got)
	}
}

// TestPendingSellMetalsAreSeparate — ทอง and เงิน are weighed in different units,
// so one customer's silver must not push their gold over the line.
func TestPendingSellMetalsAreSeparate(t *testing.T) {
	db := openTestDB(t, false)
	tx := db.Begin()
	defer tx.Rollback()

	enablePendingSell(tx, "65", "1000")
	cust := seedCustomer(t, tx, "pending-sell-metals")

	seedPendingBill(t, tx, "T-PS-MG", cust, "gold", 40)
	seedPendingBill(t, tx, "T-PS-MS", cust, "silver", 1200)
	SyncPendingSellAlert(tx, cust, "gold")
	SyncPendingSellAlert(tx, cust, "silver")

	if got := lots(t, tx, cust, "gold"); got != 0 {
		t.Errorf("gold announced %d, want 0 (silver must not count towards it)", got)
	}
	if got := lots(t, tx, cust, "silver"); got != 1 {
		t.Errorf("silver announced %d, want 1", got)
	}
}

// TestPendingSellSumsEveryPendingBill — an auto-sell fill opens its own bill
// alongside the one the customer has been adding to, and both are metal the shop
// is holding for them.
func TestPendingSellSumsEveryPendingBill(t *testing.T) {
	db := openTestDB(t, false)
	tx := db.Begin()
	defer tx.Rollback()

	enablePendingSell(tx, "65", "1000")
	cust := seedCustomer(t, tx, "pending-sell-multi")

	seedPendingBill(t, tx, "T-PS-X1", cust, "gold", 30)
	seedPendingBill(t, tx, "T-PS-X2", cust, "gold", 40) // e.g. an auto-sell fill
	if w := PendingSellWeight(tx, cust, "gold"); w != 70 {
		t.Fatalf("pile = %v, want 70 across both bills", w)
	}
	SyncPendingSellAlert(tx, cust, "gold")
	if got := lots(t, tx, cust, "gold"); got != 1 {
		t.Fatalf("announced %d, want 1", got)
	}
}

// TestPendingSellThresholdZeroDisables — 0 means off for that metal alone, and
// it must not write a latch either.
func TestPendingSellThresholdZeroDisables(t *testing.T) {
	db := openTestDB(t, false)
	tx := db.Begin()
	defer tx.Rollback()

	enablePendingSell(tx, "0", "1000")
	cust := seedCustomer(t, tx, "pending-sell-zero")

	seedPendingBill(t, tx, "T-PS-Z", cust, "gold", 500)
	SyncPendingSellAlert(tx, cust, "gold")
	if got := lots(t, tx, cust, "gold"); got != 0 {
		t.Fatalf("latch written while the rule is off: %d", got)
	}
}

// TestPendingSellLegacyBillReadsAsGold — a row with no metal tag is gold
// (migration 85), the same reading every bill query uses.
func TestPendingSellLegacyBillReadsAsGold(t *testing.T) {
	db := openTestDB(t, false)
	tx := db.Begin()
	defer tx.Rollback()

	enablePendingSell(tx, "65", "1000")
	cust := seedCustomer(t, tx, "pending-sell-legacy")

	seedPendingBill(t, tx, "T-PS-LEG", cust, "", 70)
	if w := PendingSellWeight(tx, cust, "gold"); w != 70 {
		t.Fatalf("legacy untagged bill = %v, want 70 on gold", w)
	}
}

// TestPendingSellStatusCountsWhoIsOver feeds the settings page: how many
// customers are sitting over the line right now, and the biggest pile.
func TestPendingSellStatusCountsWhoIsOver(t *testing.T) {
	db := openTestDB(t, false)
	tx := db.Begin()
	defer tx.Rollback()

	enablePendingSell(tx, "65", "1000")
	over := seedCustomer(t, tx, "pending-sell-over")
	under := seedCustomer(t, tx, "pending-sell-under")
	seedPendingBill(t, tx, "T-PS-OV", over, "gold", 120)
	seedPendingBill(t, tx, "T-PS-UN", under, "gold", 10)

	var gold PendingSellStatus
	for _, s := range GetPendingSellStatus(tx) {
		if s.Metal == "gold" {
			gold = s
		}
	}
	if gold.OverCount != 1 {
		t.Errorf("over_count = %d, want 1", gold.OverCount)
	}
	if gold.TopWeight != 120 || gold.TopName != "pending-sell-over" {
		t.Errorf("top pile = %v by %q, want 120 by pending-sell-over", gold.TopWeight, gold.TopName)
	}
	if gold.Unit != "บาท" || gold.Threshold != 65 {
		t.Errorf("gold status = %+v", gold)
	}
}
