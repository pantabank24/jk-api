package service

import (
	"os"
	"strings"
	"testing"

	"jk-api/internal/entity"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TEST_DB_DSN points these tests at a dev database. Every test that writes runs
// inside a transaction that is rolled back, so nothing is left behind.
func testDSN() string {
	if dsn := os.Getenv("TEST_DB_DSN"); dsn != "" {
		return dsn
	}
	return "host=localhost port=5432 user=postgres password=postgres dbname=jk_db sslmode=disable"
}

func openTestDB(t *testing.T, dryRun bool) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(postgres.Open(testDSN()), &gorm.Config{DryRun: dryRun, Logger: logger.Discard})
	if err != nil {
		t.Skipf("no test database available: %v", err)
	}
	return db
}

// TestBacklogSQL pins the generated SQL: the count must group issue-groups, both
// queries must keep every condition, and soft-deleted rows must stay excluded.
func TestBacklogSQL(t *testing.T) {
	db := openTestDB(t, true)
	store := uint(3)
	bills := db.Model(&entity.Quotation{}).
		Where("is_bill = ? AND status = ?", true, billStatusCompleted).
		Where("COALESCE(NULLIF(metal, ''), 'gold') = ?", "silver").
		Where("store_id = ?", store)

	var head struct{ Count int64 }
	countSQL := bills.Session(&gorm.Session{}).
		Select("COUNT(DISTINCT COALESCE(issued_quotation_id, id)) AS count").
		Scan(&head).Statement.SQL.String()

	var agg struct{ Weight, Amount float64 }
	itemSQL := db.Model(&entity.QuotationItem{}).
		Where("quotation_id IN (?)", bills.Session(&gorm.Session{}).Select("quotations.id")).
		Where("COALESCE(NULLIF(metal, ''), 'gold') = ?", "silver").
		Select("COALESCE(SUM(weight), 0) AS weight, COALESCE(SUM(total), 0) AS amount").
		Scan(&agg).Statement.SQL.String()

	for _, want := range []string{"COUNT(DISTINCT COALESCE(issued_quotation_id, id))", "is_bill", "store_id", `"quotations"."deleted_at" IS NULL`} {
		if !strings.Contains(countSQL, want) {
			t.Fatalf("count SQL missing %q:\n%s", want, countSQL)
		}
	}
	for _, want := range []string{"SELECT quotations.id", "is_bill", `"quotation_items"."deleted_at" IS NULL`} {
		if !strings.Contains(itemSQL, want) {
			t.Fatalf("item SQL missing %q:\n%s", want, itemSQL)
		}
	}
}

// seedBill inserts a completed (status 12) bill with one item, optionally rolled
// into an issue-group via issuedID.
func seedBill(t *testing.T, tx *gorm.DB, code, metal string, issuedID *uint, weight, amount float64) uint {
	t.Helper()
	bill := entity.Quotation{
		Code: code, IsBill: true, Status: billStatusCompleted, Metal: metal,
		IssuedQuotationID: issuedID,
	}
	if err := tx.Create(&bill).Error; err != nil {
		t.Fatalf("seed bill %s: %v", code, err)
	}
	item := entity.QuotationItem{QuotationID: bill.ID, Metal: metal, Weight: weight, Total: amount}
	if err := tx.Create(&item).Error; err != nil {
		t.Fatalf("seed item for %s: %v", code, err)
	}
	return bill.ID
}

// TestCountLineBacklogGroupsAndSplitsMetals is the fix this change rests on: the
// alert must count what the เคลียร์บิล page counts — issue-groups, one metal.
func TestCountLineBacklogGroupsAndSplitsMetals(t *testing.T) {
	db := openTestDB(t, false)
	tx := db.Begin()
	defer tx.Rollback()

	// Three gold bills, two of them issued together → 2 groups, not 3 rows.
	lead := seedBill(t, tx, "T-G1", "gold", nil, 1, 100)
	seedBill(t, tx, "T-G2", "gold", &lead, 2, 200)
	seedBill(t, tx, "T-G3", "gold", nil, 3, 300)
	// Silver must not land in the gold total.
	seedBill(t, tx, "T-S1", "silver", nil, 10, 1000)
	// A legacy row with no metal tag reads as gold (migration 85).
	seedBill(t, tx, "T-L1", "", nil, 4, 400)

	gold := CountLineBacklog(tx, "gold", nil)
	if gold.Count != 3 {
		t.Errorf("gold count = %d, want 3 (2 issue-groups + 1 legacy row)", gold.Count)
	}
	if gold.Weight != 10 || gold.Amount != 1000 {
		t.Errorf("gold weight/amount = %v/%v, want 10/1000", gold.Weight, gold.Amount)
	}

	silver := CountLineBacklog(tx, "silver", nil)
	if silver.Count != 1 {
		t.Errorf("silver count = %d, want 1", silver.Count)
	}
	if silver.Weight != 10 || silver.Amount != 1000 {
		t.Errorf("silver weight/amount = %v/%v, want 10/1000", silver.Weight, silver.Amount)
	}
}

// TestLatchFiresOncePerCrossing covers the quota complaint: 19 → 20 alerts, every
// approval above the threshold stays quiet, and dropping back under re-arms it.
func TestLatchFiresOncePerCrossing(t *testing.T) {
	db := openTestDB(t, false)
	tx := db.Begin()
	defer tx.Rollback()

	setConfigValue(tx, keyLineMetalThreshold("gold"), "2")
	setConfigValue(tx, keyLineMetalLatch("gold"), "false")
	latched := func() bool { return configValue(tx, keyLineMetalLatch("gold"), "false") == "true" }

	// Below threshold: nothing latches.
	seedBill(t, tx, "T-LA-1", "gold", nil, 1, 100)
	SyncLineBacklogAlert(tx, "gold", nil, true)
	if latched() {
		t.Fatal("latched below threshold")
	}

	// Crossing up (1 → 2): latches, i.e. the single alert went out.
	seedBill(t, tx, "T-LA-2", "gold", nil, 1, 100)
	SyncLineBacklogAlert(tx, "gold", nil, true)
	if !latched() {
		t.Fatal("did not latch on upward crossing")
	}

	// Still above: stays latched, so no second message is sent.
	seedBill(t, tx, "T-LA-3", "gold", nil, 1, 100)
	SyncLineBacklogAlert(tx, "gold", nil, true)
	if !latched() {
		t.Fatal("latch cleared while still above threshold")
	}

	// Cleared back under the threshold → re-arms for the next crossing.
	if err := tx.Model(&entity.Quotation{}).
		Where("code IN ?", []string{"T-LA-2", "T-LA-3"}).
		Update("status", 14).Error; err != nil {
		t.Fatalf("clear bills: %v", err)
	}
	SyncLineBacklogAlert(tx, "gold", nil, false)
	if latched() {
		t.Fatal("latch not released after dropping below threshold")
	}
}

// TestSilverApprovalDoesNotLatchGold — the two metals are independent channels.
func TestSilverApprovalDoesNotLatchGold(t *testing.T) {
	db := openTestDB(t, false)
	tx := db.Begin()
	defer tx.Rollback()

	setConfigValue(tx, keyLineMetalThreshold("gold"), "1")
	setConfigValue(tx, keyLineMetalThreshold("silver"), "1")
	setConfigValue(tx, keyLineMetalLatch("gold"), "false")
	setConfigValue(tx, keyLineMetalLatch("silver"), "false")

	seedBill(t, tx, "T-SEP-G", "gold", nil, 1, 100)
	seedBill(t, tx, "T-SEP-S", "silver", nil, 1, 100)

	// A silver approval evaluates silver only.
	SyncLineBacklogAlert(tx, "silver", nil, true)
	if configValue(tx, keyLineMetalLatch("silver"), "false") != "true" {
		t.Fatal("silver did not latch")
	}
	if configValue(tx, keyLineMetalLatch("gold"), "false") == "true" {
		t.Fatal("silver approval latched gold")
	}
}

// TestThresholdZeroDisables — 0 means off, and it must not leave a stale latch.
func TestThresholdZeroDisables(t *testing.T) {
	db := openTestDB(t, false)
	tx := db.Begin()
	defer tx.Rollback()

	setConfigValue(tx, keyLineMetalThreshold("silver"), "0")
	setConfigValue(tx, keyLineMetalLatch("silver"), "true")
	seedBill(t, tx, "T-Z-1", "silver", nil, 1, 100)

	SyncLineBacklogAlert(tx, "silver", nil, true)
	if configValue(tx, keyLineMetalLatch("silver"), "false") == "true" {
		t.Fatal("latch left set while alerts are disabled")
	}
}

// TestBacklogBubbleShape — the Flex payload must carry the metal's own numbers and
// a header/body LINE will accept.
func TestBacklogBubbleShape(t *testing.T) {
	b := backlogBubble("silver", LineBacklog{Metal: "silver", Count: 1234, Weight: 5678.5, Amount: 90123.25}, 20)
	if b["type"] != "bubble" {
		t.Fatalf("type = %v, want bubble", b["type"])
	}
	if _, ok := b["header"]; !ok {
		t.Fatal("missing header")
	}
	body, ok := b["body"].(map[string]interface{})
	if !ok {
		t.Fatal("missing body")
	}
	contents, ok := body["contents"].([]map[string]interface{})
	if !ok || len(contents) < 5 {
		t.Fatalf("body contents = %v", body["contents"])
	}
	if got := formatInt(1234567); got != "1,234,567" {
		t.Errorf("formatInt = %q", got)
	}
	if got := formatFloat(5678.5); got != "5,678.50" {
		t.Errorf("formatFloat = %q", got)
	}
}
