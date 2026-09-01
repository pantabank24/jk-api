package usecase

import (
	"math"
	"testing"
)

func approx(a, b float64) bool { return math.Abs(a-b) < 0.005 }

// The sample paper receipt this module was built from. Its first line reads
// 1000 กรัม x "4,350.59" = 4,350,592.00 — which multiplying out does NOT give
// (4,350,590.00). Line totals are therefore taken as typed and only added up,
// which is what lets an entry reproduce the paper exactly.
func TestBuildItemsMatchesPaperReceipt(t *testing.T) {
	items, total := buildItems([]ReceiptItemRequest{
		{Description: "AU 99.99", Quantity: 1000, Unit: "กรัม", UnitPrice: 4350.59, Amount: 4350592},
		{Description: "AU 99.99", Quantity: 2000, Unit: "กรัม", UnitPrice: 4346, Amount: 8692000},
	})
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2", len(items))
	}
	if !approx(items[0].Amount, 4350592) {
		t.Errorf("line 1 = %.2f, want 4350592.00", items[0].Amount)
	}
	if !approx(items[1].Amount, 8692000) {
		t.Errorf("line 2 = %.2f, want 8692000.00", items[1].Amount)
	}
	if !approx(total, 13042592) {
		t.Errorf("total = %.2f, want 13042592.00", total)
	}
	// Order is what the form shows, so the printed ลำดับ must follow the input.
	if items[0].SortOrder != 0 || items[1].SortOrder != 1 {
		t.Errorf("sort_order = %d,%d, want 0,1", items[0].SortOrder, items[1].SortOrder)
	}
}

// The editor keeps blank rows the way the paper form has blank rows; they must not
// reach the database as empty lines.
func TestBuildItemsDropsBlankRows(t *testing.T) {
	items, total := buildItems([]ReceiptItemRequest{
		{Description: "AU 99.99", Quantity: 10, UnitPrice: 100, Amount: 1000},
		{},
		{Description: "  "},
		{Description: "AU 96.5", Quantity: 5, UnitPrice: 200, Amount: 1000},
	})
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2", len(items))
	}
	if items[1].SortOrder != 1 {
		t.Errorf("sort_order = %d, want 1 (renumbered after the blanks)", items[1].SortOrder)
	}
	if !approx(total, 2000) {
		t.Errorf("total = %.2f, want 2000.00", total)
	}
}

// A line can carry nothing but its รวม figure (the paper sometimes writes only a
// lump sum), so the amount alone has to keep the row alive.
func TestBuildItemsKeepsAmountOnlyRows(t *testing.T) {
	items, total := buildItems([]ReceiptItemRequest{{Amount: 1500}})
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
	if !approx(total, 1500) {
		t.Errorf("total = %.2f, want 1500.00", total)
	}
}

func TestApplyPaymentBlock(t *testing.T) {
	u := &receiptUsecase{}
	req := &ReceiptRequest{
		IssuedDate: "2026-08-03",
		Items:      []ReceiptItemRequest{{Description: "AU", Quantity: 2, UnitPrice: 1000, Amount: 2000}},
	}

	t.Run("total is the sum of the typed lines", func(t *testing.T) {
		r := newReceipt(t, u, req)
		if !approx(r.TotalAmount, 2000) {
			t.Errorf("total = %.2f, want 2000.00", r.TotalAmount)
		}
	})

	t.Run("paid date is optional", func(t *testing.T) {
		r := newReceipt(t, u, req)
		if r.PaidDate != nil {
			t.Errorf("paid_date = %v, want nil", r.PaidDate)
		}
	})

	t.Run("paid date is taken when given", func(t *testing.T) {
		withDate := *req
		withDate.PaidDate = "2026-08-03"
		r := newReceipt(t, u, &withDate)
		if r.PaidDate == nil || r.PaidDate.Format("2006-01-02") != "2026-08-03" {
			t.Errorf("paid_date = %v, want 2026-08-03", r.PaidDate)
		}
	})
}

func TestApplyRejectsBadInput(t *testing.T) {
	u := &receiptUsecase{}
	cases := map[string]*ReceiptRequest{
		"missing date": {Items: []ReceiptItemRequest{{Description: "AU", Amount: 1}}},
		"bad date":     {IssuedDate: "03/08/2569", Items: []ReceiptItemRequest{{Description: "AU", Amount: 1}}},
		"no items":     {IssuedDate: "2026-08-03"},
		"blank items":  {IssuedDate: "2026-08-03", Items: []ReceiptItemRequest{{}, {}}},
		"bad paid date": {
			IssuedDate: "2026-08-03",
			PaidDate:   "3/8/2569",
			Items:      []ReceiptItemRequest{{Description: "AU", Amount: 1}},
		},
	}
	for name, req := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := u.apply(newEmptyReceipt(), req); err == nil {
				t.Fatal("want an error, got none")
			}
		})
	}
}
