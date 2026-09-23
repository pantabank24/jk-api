package usecase

import (
	"math"
	"os"
	"testing"

	"jk-api/internal/entity"
	billRepo "jk-api/internal/module/bill/repository"
	memberRepo "jk-api/internal/module/member/repository"
	notificationRepo "jk-api/internal/module/notification/repository"
	"jk-api/internal/module/quotation/repository"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// The whole by-weight issuance through CreateQuotation, against the dev database
// inside a transaction that is always rolled back.
func TestCreateQuotation_ByWeight_DB(t *testing.T) {
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

	uc := NewQuotationUsecase(
		repository.NewQuotationRepository(tx),
		memberRepo.NewMemberRepository(tx),
		notificationRepo.NewNotificationRepository(tx),
		billRepo.NewBillBalanceRepository(tx),
	)

	var staff entity.User
	if err := tx.First(&staff).Error; err != nil {
		t.Skipf("dev database has no users: %v", err)
	}

	// 5 @ 39,000 + 3 @ 40,000 + 2 @ 42,000 = 10 baht at 39,900.
	src := entity.Quotation{Code: "BILLT9101", IsBill: true, Status: 10, Metal: "gold", TotalAmount: 399000, CreatedBy: &staff.ID}
	if err := tx.Create(&src).Error; err != nil {
		t.Fatal(err)
	}
	for _, l := range [][2]float64{{5, 39000}, {3, 40000}, {2, 42000}} {
		tx.Create(&entity.QuotationItem{QuotationID: src.ID, TypeID: "9001", TypeName: "ทองคำแท่ง 96.5%",
			Metal: "gold", Weight: l[0], Price: l[1], Total: l[0] * l[1]})
	}

	issue := func(weight float64) (*entity.Quotation, error) {
		return uc.CreateQuotation(&CreateQuotationRequest{
			CreatedByUserID: staff.ID,
			PDPAConsent:     true,
			UseBillCode:     true,
			BillIDs:         []uint{src.ID},
			BillWeight:      weight,
			Items: []CreateQuotationItemRequest{{TypeID: "9001", TypeName: "ทอง 96.5%", Metal: "gold",
				Price: 39900, Weight: 6, Total: 239400}},
		})
	}
	outstanding := func() (w, a float64) {
		var agg struct{ W, A float64 }
		tx.Model(&entity.QuotationItem{}).
			Where("quotation_id IN (?)", tx.Model(&entity.Quotation{}).
				Where("is_bill = ? AND status = ? AND code LIKE ?", true, 10, "BILL%").
				Where("id >= ?", src.ID).Select("id")).
			Select("COALESCE(SUM(weight),0) AS w, COALESCE(SUM(total),0) AS a").Scan(&agg)
		return agg.W, agg.A
	}

	// Round 1: 6 of 10.
	q1, err := issue(6)
	if err != nil {
		t.Fatalf("round 1: %v", err)
	}
	var issued []entity.Quotation
	tx.Preload("Items").Where("is_bill = ? AND issued_quotation_id = ?", true, q1.ID).Find(&issued)
	if len(issued) != 1 || issued[0].ID == src.ID || issued[0].Status != 11 || issued[0].Items[0].Weight != 6 {
		t.Fatalf("round 1 should issue one split-off bill of 6 baht, got %+v", issued)
	}
	// The round-1 document is numbered after the bill it issued, not the source —
	// otherwise every partial round would print the source bill's number.
	if q1.BillID == nil || *q1.BillID != issued[0].ID || q1.DisplayCode != issued[0].Code {
		t.Fatalf("round 1 should carry the split bill (%d %s), got bill_id %v display %q",
			issued[0].ID, issued[0].Code, q1.BillID, q1.DisplayCode)
	}
	if reread, err := repository.NewQuotationRepository(tx).FindByID(q1.ID); err != nil || reread.DisplayCode != issued[0].Code {
		t.Fatalf("re-read display code = %q (err %v), want %s", reread.DisplayCode, err, issued[0].Code)
	}
	var srcNow entity.Quotation
	tx.First(&srcNow, src.ID)
	if srcNow.Status != 10 || math.Abs(srcNow.TotalAmount-159600) > 0.001 {
		t.Fatalf("source should stay รอออกบิล at 159,600, got status %d total %v", srcNow.Status, srcNow.TotalAmount)
	}
	if w, a := outstanding(); math.Abs(w-4) > 1e-6 || math.Abs(a-159600) > 0.001 {
		t.Fatalf("outstanding after round 1 = %v / %v, want 4 / 159,600", w, a)
	}

	// More than is left: refused, and nothing is created.
	var before, after int64
	tx.Model(&entity.Quotation{}).Where("is_bill = ?", false).Count(&before)
	if _, err := issue(6); err == nil {
		t.Fatal("6 of 4 outstanding must be refused")
	}
	tx.Model(&entity.Quotation{}).Where("is_bill = ?", false).Count(&after)
	if before != after {
		t.Fatalf("a refused save still created %d quotation(s)", after-before)
	}

	// Reverting round 1 (ดึงกลับไปแก้ไข) returns its 6 to รอออกบิล: 10 again.
	if err := billRepo.NewBillRepository(tx).RevertIssuance(issued[0].ID); err != nil {
		t.Fatal(err)
	}
	if w, a := outstanding(); math.Abs(w-10) > 1e-6 || math.Abs(a-399000) > 0.001 {
		t.Fatalf("outstanding after revert = %v / %v, want 10 / 399,000", w, a)
	}
	// Re-issue exactly as the edit screen does: the reverted bill, whole.
	if _, err := uc.CreateQuotation(&CreateQuotationRequest{
		CreatedByUserID: staff.ID, PDPAConsent: true, UseBillCode: true, BillIDs: []uint{issued[0].ID},
		Items: []CreateQuotationItemRequest{{TypeID: "9001", TypeName: "ทอง 96.5%", Metal: "gold", Weight: 6, Total: 239400}},
	}); err != nil {
		t.Fatal(err)
	}

	// Round 2: all that is left (4) — no split, the source bill is issued whole.
	q2, err := issue(4)
	if err != nil {
		t.Fatalf("round 2: %v", err)
	}
	tx.First(&srcNow, src.ID)
	if srcNow.Status != 11 || srcNow.IssuedQuotationID == nil || *srcNow.IssuedQuotationID != q2.ID {
		t.Fatalf("remaining 4 should issue the source bill whole, got status %d", srcNow.Status)
	}
	// The final round issues the source bill itself, so it keeps the source number.
	if q2.DisplayCode != src.Code || q2.DisplayCode == q1.DisplayCode {
		t.Fatalf("round 2 display %q, want %s (and distinct from round 1's %q)", q2.DisplayCode, src.Code, q1.DisplayCode)
	}
	if w, _ := outstanding(); math.Abs(w) > 1e-6 {
		t.Fatalf("nothing should be outstanding, got %v", w)
	}
}
