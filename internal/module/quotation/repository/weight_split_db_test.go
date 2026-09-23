package repository

import (
	"errors"
	"math"
	"os"
	"testing"

	"jk-api/internal/entity"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Runs against the dev database inside a transaction that is always rolled back.
func openSplitTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DB_DSN")
	if dsn == "" {
		dsn = "host=localhost port=5432 user=postgres password=postgres dbname=jk_db sslmode=disable"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Skipf("no test database available: %v", err)
	}
	tx := db.Begin()
	t.Cleanup(func() { tx.Rollback() })
	return tx
}

func seedPendingBill(t *testing.T, tx *gorm.DB, code string, lines ...[2]float64) uint {
	t.Helper()
	var total float64
	for _, l := range lines {
		total += l[0] * l[1]
	}
	creator := uint(0)
	var anyUser entity.User
	if err := tx.First(&anyUser).Error; err == nil {
		creator = anyUser.ID
	}
	bill := entity.Quotation{Code: code, IsBill: true, Status: 10, Metal: "gold", TotalAmount: total}
	if creator != 0 {
		bill.CreatedBy = &creator
	}
	if err := tx.Create(&bill).Error; err != nil {
		t.Fatalf("seed bill: %v", err)
	}
	for _, l := range lines {
		it := entity.QuotationItem{QuotationID: bill.ID, TypeID: "9001", TypeName: "ทองคำแท่ง 96.5%",
			Metal: "gold", Weight: l[0], Price: l[1], Total: l[0] * l[1]}
		if err := tx.Create(&it).Error; err != nil {
			t.Fatalf("seed item: %v", err)
		}
	}
	return bill.ID
}

func TestSplitBillWeight_DB(t *testing.T) {
	tx := openSplitTestDB(t)
	repo := &quotationRepository{db: tx}
	src := seedPendingBill(t, tx, "BILLT9001", [2]float64{5, 39000}, [2]float64{3, 40000}, [2]float64{2, 42000})

	newID, whole, err := repo.SplitBillWeight([]uint{src}, src, 6)
	if err != nil || whole || newID == 0 {
		t.Fatalf("split: id=%d whole=%v err=%v", newID, whole, err)
	}

	var srcBill, newBill entity.Quotation
	tx.Preload("Items").First(&srcBill, src)
	tx.Preload("Items").First(&newBill, newID)

	if srcBill.Status != 10 || len(srcBill.Items) != 4 {
		t.Fatalf("source bill should stay pending with its 3 sales + 1 cut, got status %d, %d lines", srcBill.Status, len(srcBill.Items))
	}
	if math.Abs(srcBill.TotalAmount-159600) > 0.001 {
		t.Fatalf("source total_amount = %v, want 159,600", srcBill.TotalAmount)
	}
	cut := srcBill.Items[3]
	if cut.SplitBillID == nil || *cut.SplitBillID != newID || cut.Weight != -6 || cut.Total != -239400 {
		t.Fatalf("cut line = %+v", cut)
	}
	if newBill.Status != 10 || !newBill.IsBill || newBill.Metal != "gold" || newBill.CreatedBy == nil && srcBill.CreatedBy != nil {
		t.Fatalf("new bill header = %+v", newBill)
	}
	if len(newBill.Items) != 1 || newBill.Items[0].Weight != 6 || newBill.Items[0].Total != 239400 ||
		newBill.Items[0].TypeName != "ทองคำแท่ง 96.5%" || newBill.TotalAmount != 239400 {
		t.Fatalf("new bill line = %+v total %v", newBill.Items, newBill.TotalAmount)
	}
	if cut.TypeName != "ตัดออกใบ → "+newBill.Code+" · ทองคำแท่ง 96.5%" {
		t.Fatalf("cut label = %q", cut.TypeName)
	}

	// Asking for everything that is left is a whole issuance — no further split.
	if _, whole, err := repo.SplitBillWeight([]uint{src}, src, 4); err != nil || !whole {
		t.Fatalf("remaining 4 should report whole: whole=%v err=%v", whole, err)
	}
	// And more than is left is refused.
	if _, _, err := repo.SplitBillWeight([]uint{src}, src, 4.5); err == nil {
		t.Fatal("4.5 of 4 outstanding must be refused")
	}

	// A bill that already left รอออกบิล cannot be split.
	tx.Model(&entity.Quotation{}).Where("id = ?", src).Update("status", 11)
	if _, _, err := repo.SplitBillWeight([]uint{src}, src, 1); !errors.Is(err, ErrBillNotPending) {
		t.Fatalf("issued bill: err = %v, want ErrBillNotPending", err)
	}
}
