package repository

import (
	"os"
	"testing"

	"jk-api/internal/entity"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TEST_DB_DSN points these tests at a dev database. Every test runs inside a
// transaction that is rolled back, so nothing is left behind.
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

// seedOrder inserts a waiting order for a synthetic user id.
func seedOrder(t *testing.T, tx *gorm.DB, userID uint, weight, target float64) *entity.SellOrder {
	t.Helper()
	o := &entity.SellOrder{
		UserID:      userID,
		Metal:       "gold",
		Weight:      weight,
		TargetPrice: target,
		Status:      entity.SellOrderActive,
	}
	if err := tx.Create(o).Error; err != nil {
		t.Fatalf("seeding order failed: %v", err)
	}
	return o
}

// TestClaimDueOnlyClaimsOnce is the double-fill guarantee: a second sweep at the
// same price must find nothing, because the first one already moved the rows out
// of 'active'. Without it, a tick overrunning into the next would sell twice.
func TestClaimDueOnlyClaimsOnce(t *testing.T) {
	db := openTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	repo := NewSellOrderRepository(tx)
	const user = uint(9_900_001)

	due := seedOrder(t, tx, user, 10, 60000)    // target reached
	alsoDue := seedOrder(t, tx, user, 5, 59500) // reached, further below
	notDue := seedOrder(t, tx, user, 5, 61000)  // still above the market

	claimed, err := repo.ClaimDue(60000)
	if err != nil {
		t.Fatalf("ClaimDue: %v", err)
	}
	got := map[uint]bool{}
	for _, o := range claimed {
		got[o.ID] = true
		if o.Status != entity.SellOrderFilling {
			t.Fatalf("claimed order #%d has status %q, want %q", o.ID, o.Status, entity.SellOrderFilling)
		}
	}
	if !got[due.ID] || !got[alsoDue.ID] {
		t.Fatalf("expected both due orders claimed, got %v", got)
	}
	if got[notDue.ID] {
		t.Fatal("claimed an order whose target the price has not reached")
	}

	// The second sweep is the actual assertion.
	again, err := repo.ClaimDue(60000)
	if err != nil {
		t.Fatalf("second ClaimDue: %v", err)
	}
	for _, o := range again {
		if o.ID == due.ID || o.ID == alsoDue.ID {
			t.Fatalf("order #%d claimed twice — it would be sold twice", o.ID)
		}
	}
}

// TestCancelOnlyWhenWaiting: an order the engine has already claimed is becoming a
// bill, so cancelling it would leave a bill with no order behind it.
func TestCancelOnlyWhenWaiting(t *testing.T) {
	db := openTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	repo := NewSellOrderRepository(tx)
	const user = uint(9_900_002)

	waiting := seedOrder(t, tx, user, 5, 60000)
	claimed := seedOrder(t, tx, user, 5, 60000)
	if err := tx.Model(claimed).Update("status", entity.SellOrderFilling).Error; err != nil {
		t.Fatalf("setting up claimed order: %v", err)
	}

	if err := repo.Cancel(waiting.ID, "ยกเลิกโดยลูกค้า"); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	after, err := repo.FindByID(waiting.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if after.Status != entity.SellOrderCancelled {
		t.Fatalf("waiting order status = %q, want %q", after.Status, entity.SellOrderCancelled)
	}
	if after.CancelledAt == nil {
		t.Fatal("cancelled order has no cancelled_at")
	}

	if err := repo.Cancel(claimed.ID, "ควรไม่มีผล"); err != nil {
		t.Fatalf("Cancel on claimed order: %v", err)
	}
	stillFilling, err := repo.FindByID(claimed.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if stillFilling.Status != entity.SellOrderFilling {
		t.Fatalf("claimed order was cancelled (status %q) — a bill could be left orphaned", stillFilling.Status)
	}
}

// TestActiveTotalsCountsClaimed: the caps exist to bound how much can fill at
// once, so an order mid-fill still counts against them.
func TestActiveTotalsCountsClaimed(t *testing.T) {
	db := openTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	repo := NewSellOrderRepository(tx)
	const user = uint(9_900_003)

	seedOrder(t, tx, user, 10, 60000)
	claimed := seedOrder(t, tx, user, 15, 60500)
	if err := tx.Model(claimed).Update("status", entity.SellOrderFilling).Error; err != nil {
		t.Fatalf("setting up claimed order: %v", err)
	}
	cancelled := seedOrder(t, tx, user, 100, 61000)
	if err := repo.Cancel(cancelled.ID, "ยกเลิก"); err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	totals, err := repo.ActiveTotalsByUser(user)
	if err != nil {
		t.Fatalf("ActiveTotalsByUser: %v", err)
	}
	if totals.Count != 2 {
		t.Fatalf("count = %d, want 2 (active + filling, excluding cancelled)", totals.Count)
	}
	if totals.Weight != 25 {
		t.Fatalf("weight = %g, want 25", totals.Weight)
	}
}

// TestFindBillBySellOrderFollowsTheItems is the recovery's safety net. A fill
// accumulates into whatever bill the customer has open, so several orders can end
// up in one bill and the bill's own sell_order_id only remembers the last of them.
// If the lookup asked the bill, an earlier order interrupted mid-fill would look
// like one that never happened — and the next tick would sell it all over again.
func TestFindBillBySellOrderFollowsTheItems(t *testing.T) {
	db := openTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	repo := NewSellOrderRepository(tx)
	const customer = uint(9_900_101)

	first := seedOrder(t, tx, customer, 5, 60000)
	second := seedOrder(t, tx, customer, 3, 60500)

	// One shared bill: the customer's own line, then both fills appended to it.
	// Its bill-level link names only the order that landed last.
	bill := &entity.Quotation{
		Code: "BILL-T-9900101", IsBill: true, Status: 10, Metal: "gold",
		AutoSell: true, SellOrderID: &second.ID,
		Items: []entity.QuotationItem{
			{TypeName: "ขายเอง", Metal: "gold", Weight: 2, Price: 59000},
			{TypeName: "ฟิลแรก", Metal: "gold", Weight: 5, Price: 60000, SellOrderID: &first.ID},
			{TypeName: "ฟิลสอง", Metal: "gold", Weight: 3, Price: 60500, SellOrderID: &second.ID},
		},
	}
	if err := tx.Create(bill).Error; err != nil {
		t.Fatalf("seeding bill failed: %v", err)
	}

	for _, o := range []*entity.SellOrder{first, second} {
		got, err := repo.FindBillBySellOrder(o.ID)
		if err != nil {
			t.Fatalf("FindBillBySellOrder(#%d): %v — the recovery would resell this order", o.ID, err)
		}
		if got.ID != bill.ID {
			t.Errorf("order #%d resolved to bill %d, want %d", o.ID, got.ID, bill.ID)
		}
	}

	// An order that never reached a bill must still come back empty, or the
	// recovery would mark an unsold order as filled.
	never := seedOrder(t, tx, customer, 1, 70000)
	if got, err := repo.FindBillBySellOrder(never.ID); err == nil {
		t.Errorf("an unfilled order resolved to bill %d", got.ID)
	}
}
