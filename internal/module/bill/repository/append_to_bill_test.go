package repository

import (
	"testing"

	"gorm.io/gorm/clause"
)

// TestAppendUpdates pins the two things an append must do to the bill row. The
// รอออกบิล bill is shared: the customer keeps selling into it and the auto-sell
// engine fills into it too, so the total has to move in SQL and a fill has to
// leave its order's link behind.
func TestAppendUpdates(t *testing.T) {
	t.Run("total is incremented in SQL, never written from a stale read", func(t *testing.T) {
		got := appendUpdates(1500, nil)

		expr, ok := got["total_amount"].(clause.Expr)
		if !ok {
			t.Fatalf("total_amount is %T, want a SQL expression — assigning a number here\n"+
				"lets a concurrent sell's amount be overwritten", got["total_amount"])
		}
		if expr.SQL != "total_amount + ?" {
			t.Errorf("total_amount SQL = %q, want %q", expr.SQL, "total_amount + ?")
		}
		if len(expr.Vars) != 1 || expr.Vars[0] != float64(1500) {
			t.Errorf("total_amount vars = %v, want [1500]", expr.Vars)
		}
	})

	t.Run("a manual sell leaves the auto-sell flags untouched", func(t *testing.T) {
		got := appendUpdates(1500, nil)

		if _, present := got["auto_sell"]; present {
			t.Error("auto_sell was written by a manual sell — appending to a bill an\n" +
				"earlier fill created must not restate, or clear, its flag")
		}
		if _, present := got["sell_order_id"]; present {
			t.Error("sell_order_id was written by a manual sell")
		}
	})

	t.Run("a fill stamps its order onto the bill its items joined", func(t *testing.T) {
		orderID := uint(42)
		got := appendUpdates(1500, &orderID)

		if got["auto_sell"] != true {
			t.Errorf("auto_sell = %v, want true", got["auto_sell"])
		}
		// The boot recovery finds the bill by this column. Without the stamp an
		// interrupted fill looks like one that never happened, and the next tick
		// appends the same items a second time.
		if got["sell_order_id"] != orderID {
			t.Errorf("sell_order_id = %v, want %d", got["sell_order_id"], orderID)
		}
	})
}
