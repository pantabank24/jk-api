package repository

import (
	"os"
	"testing"
	"time"

	"jk-api/internal/entity"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func testDSN() string {
	if dsn := os.Getenv("TEST_DB_DSN"); dsn != "" {
		return dsn
	}
	return "host=localhost port=5432 user=postgres password=postgres dbname=jk_db sslmode=disable"
}

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(postgres.Open(testDSN()), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Skipf("no test database available: %v", err)
	}
	return db
}

// seedDoc creates one quotation with a single item of the given metal, and
// returns its id. isBill/status let a caller build both halves of a bill pair.
func seedDoc(t *testing.T, tx *gorm.DB, code string, isBill bool, status int, metal string, weight, total float64) uint {
	t.Helper()
	q := entity.Quotation{
		Code: code, IsBill: isBill, Status: status, Metal: metal,
		TotalAmount: total, SignerName: "รายงานทดสอบ",
	}
	if err := tx.Create(&q).Error; err != nil {
		t.Fatalf("seed quotation %s: %v", code, err)
	}
	item := entity.QuotationItem{
		QuotationID: q.ID, TypeID: "9001", TypeName: "ทดสอบ " + metal,
		Metal: metal, Weight: weight, Total: total,
	}
	if err := tx.Create(&item).Error; err != nil {
		t.Fatalf("seed item for %s: %v", code, err)
	}
	return q.ID
}

// isolate removes every pre-existing document from the window the test measures,
// so the assertions are about the seeded rows only.
func isolate(t *testing.T, tx *gorm.DB, from time.Time) {
	t.Helper()
	if err := tx.Unscoped().Where("created_at >= ?", from).Delete(&entity.Quotation{}).Error; err != nil {
		t.Fatalf("isolate the window: %v", err)
	}
}

// TestSalesCountsOnlyFinishedDocuments is the rule the whole report rests on: a
// walk-in counts as soon as it is issued, a bill-issued quotation only once its
// bill has been cleared, and one still waiting on its bill counts for nothing.
func TestSalesCountsOnlyFinishedDocuments(t *testing.T) {
	db := openTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	// The seeded rows land at NOW(); isolate that instant forward.
	from := time.Now().Add(-time.Minute)
	isolate(t, tx, from)

	repo := NewReportRepository(tx)
	f := SalesFilter{Metal: "gold", From: &from, Bucket: "day"}

	// Walk-in, issued: counts.
	seedDoc(t, tx, "T-RPT-WALKIN", false, 1, "gold", 10, 1000)

	// Issued against a bill that has been cleared: counts.
	clearedQuote := seedDoc(t, tx, "T-RPT-CLEARED-Q", false, 1, "gold", 20, 2000)
	billCleared := seedDoc(t, tx, "T-RPT-CLEARED-B", true, 14, "gold", 20, 2000)
	if err := tx.Model(&entity.Quotation{}).Where("id = ?", billCleared).
		Update("issued_quotation_id", clearedQuote).Error; err != nil {
		t.Fatalf("link cleared bill: %v", err)
	}

	// Issued against a bill still at รอตรวจบิล: must NOT count.
	openQuote := seedDoc(t, tx, "T-RPT-OPEN-Q", false, 1, "gold", 40, 4000)
	billOpen := seedDoc(t, tx, "T-RPT-OPEN-B", true, 11, "gold", 40, 4000)
	if err := tx.Model(&entity.Quotation{}).Where("id = ?", billOpen).
		Update("issued_quotation_id", openQuote).Error; err != nil {
		t.Fatalf("link open bill: %v", err)
	}

	got, err := repo.Overview(f)
	if err != nil {
		t.Fatalf("overview: %v", err)
	}
	if got.DocCount != 2 {
		t.Errorf("doc_count = %d, want 2 (walk-in + cleared bill)", got.DocCount)
	}
	if got.Amount != 3000 {
		t.Errorf("amount = %.2f, want 3000", got.Amount)
	}
	if got.Weight != 30 {
		t.Errorf("weight = %.4f, want 30", got.Weight)
	}
	if got.AvgPerGram != 100 {
		t.Errorf("avg_per_gram = %.4f, want 100", got.AvgPerGram)
	}
}

// TestSalesSeparatesMetals — each report is one metal, and a document that mixes
// them contributes only its own lines to each.
func TestSalesSeparatesMetals(t *testing.T) {
	db := openTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	from := time.Now().Add(-time.Minute)
	isolate(t, tx, from)
	repo := NewReportRepository(tx)

	mixed := seedDoc(t, tx, "T-RPT-MIXED", false, 1, "gold", 5, 500)
	if err := tx.Create(&entity.QuotationItem{
		QuotationID: mixed, TypeID: "9002", TypeName: "ทดสอบ silver",
		Metal: "silver", Weight: 100, Total: 3000,
	}).Error; err != nil {
		t.Fatalf("seed silver line: %v", err)
	}

	gold, err := repo.Overview(SalesFilter{Metal: "gold", From: &from})
	if err != nil {
		t.Fatalf("gold overview: %v", err)
	}
	if gold.Amount != 500 || gold.Weight != 5 {
		t.Errorf("gold = %.2f / %.4f, want 500 / 5", gold.Amount, gold.Weight)
	}

	silver, err := repo.Overview(SalesFilter{Metal: "silver", From: &from})
	if err != nil {
		t.Fatalf("silver overview: %v", err)
	}
	if silver.Amount != 3000 || silver.Weight != 100 {
		t.Errorf("silver = %.2f / %.4f, want 3000 / 100", silver.Amount, silver.Weight)
	}
}

// TestSalesRowsAndBreakdownsRun exercises every remaining query end to end: they
// are hand-written SQL, so "does it execute at all" is worth pinning.
func TestSalesRowsAndBreakdownsRun(t *testing.T) {
	db := openTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	from := time.Now().Add(-time.Minute)
	isolate(t, tx, from)
	repo := NewReportRepository(tx)
	seedDoc(t, tx, "T-RPT-RUN", false, 1, "gold", 8, 800)

	f := SalesFilter{Metal: "gold", From: &from, Bucket: "day"}

	if _, err := repo.Series(f); err != nil {
		t.Fatalf("series: %v", err)
	}
	if _, err := repo.Series(SalesFilter{Metal: "gold", From: &from, Bucket: "month"}); err != nil {
		t.Fatalf("monthly series: %v", err)
	}
	byType, err := repo.ByType(f)
	if err != nil {
		t.Fatalf("by type: %v", err)
	}
	if len(byType) != 1 || byType[0].Amount != 800 {
		t.Errorf("by_type = %+v, want one row of 800", byType)
	}
	if _, err := repo.ByEmployee(f); err != nil {
		t.Fatalf("by employee: %v", err)
	}

	rows, total, err := repo.Rows(f, 1, 50)
	if err != nil {
		t.Fatalf("rows: %v", err)
	}
	if total != 1 || len(rows) != 1 {
		t.Fatalf("rows = %d (total %d), want 1", len(rows), total)
	}
	if rows[0].Code != "T-RPT-RUN" || rows[0].Source != "walkin" || rows[0].Amount != 800 {
		t.Errorf("row = %+v, want the seeded walk-in", rows[0])
	}
	if _, err := repo.ItemRows(f); err != nil {
		t.Fatalf("item rows: %v", err)
	}
}

// TestSalesFiltersNarrow — the filter bar must actually reach the SQL, including
// the OR-joined search, which has to stay parenthesised or it would swallow the
// scope's AND conditions and report the whole shop.
func TestSalesFiltersNarrow(t *testing.T) {
	db := openTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	from := time.Now().Add(-time.Minute)
	isolate(t, tx, from)
	repo := NewReportRepository(tx)

	seedDoc(t, tx, "T-RPT-KEEP", false, 1, "gold", 1, 100)
	seedDoc(t, tx, "T-RPT-DROP", false, 1, "gold", 1, 100)
	// A cancelled document that the search term also matches — if the OR were
	// unparenthesised this would come back too.
	seedDoc(t, tx, "T-RPT-KEEP-CANCELLED", false, 2, "gold", 1, 100)

	got, err := repo.Overview(SalesFilter{Metal: "gold", From: &from, Search: "T-RPT-KEEP"})
	if err != nil {
		t.Fatalf("overview: %v", err)
	}
	if got.DocCount != 1 {
		t.Errorf("doc_count = %d, want 1 — only the issued document matching the search", got.DocCount)
	}

	byType, err := repo.ByType(SalesFilter{Metal: "gold", From: &from, TypeID: "no-such-type"})
	if err != nil {
		t.Fatalf("by type: %v", err)
	}
	if len(byType) != 0 {
		t.Errorf("by_type with an unknown type = %+v, want empty", byType)
	}
}
