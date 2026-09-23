package usecase

import (
	"testing"

	"jk-api/internal/entity"
)

// A bill that sold 10 baht and had 6 issued by weight: three sales and the
// deduction line the partial issuance left behind (net 4 baht outstanding).
func partlyIssuedBill() []entity.QuotationItem {
	split := uint(77)
	return []entity.QuotationItem{
		{ID: 1, Metal: "gold", Weight: 5, Total: 195000},
		{ID: 2, Metal: "gold", Weight: 3, Total: 120000},
		{ID: 3, Metal: "gold", Weight: 2, Total: 84000},
		{ID: 4, Metal: "gold", Weight: -6, Total: -239400, SplitBillID: &split},
	}
}

func TestCheckRemovable(t *testing.T) {
	items := partlyIssuedBill()
	if err := checkRemovable(items, 4); err == nil {
		t.Fatal("the deduction line must never be removable")
	}
	// Dropping the 5-baht sale would leave 4 − 5 = −1 outstanding.
	if err := checkRemovable(items, 1); err == nil {
		t.Fatal("removing a sale that turns the remainder negative must be refused")
	}
	// Dropping the 3-baht sale leaves 1 baht — still a valid pending bill.
	if err := checkRemovable(items, 2); err != nil {
		t.Fatalf("a sale that leaves weight outstanding should be removable: %v", err)
	}
	// Bills never partly issued are not affected at all.
	plain := items[:3]
	if err := checkRemovable(plain, 1); err != nil {
		t.Fatalf("plain bill: %v", err)
	}
}
