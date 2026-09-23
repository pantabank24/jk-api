package repository

import (
	"math"
	"testing"

	"jk-api/internal/entity"
)

func goldLine(id uint, weight, price float64) entity.QuotationItem {
	return entity.QuotationItem{ID: id, Metal: "gold", TypeName: "ทองคำแท่ง 96.5%", Weight: weight, Price: price, Total: weight * price}
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

// The worked example the shop agreed on: 5 @ 39,000 + 3 @ 40,000 + 2 @ 42,000 is
// 10 baht at 39,900; issuing 6 leaves 4 at 39,900 = 159,600.
func TestPlanWeightSplit_SingleBill(t *testing.T) {
	bills := []entity.Quotation{{ID: 1, Metal: "gold", Items: []entity.QuotationItem{
		goldLine(1, 5, 39000), goldLine(2, 3, 40000), goldLine(3, 2, 42000),
	}}}
	sources, metal, err := splitSources(bills)
	if err != nil || metal != "gold" {
		t.Fatalf("splitSources: %v %q", err, metal)
	}
	plan, whole, err := planWeightSplit(sources, 6)
	if err != nil || whole {
		t.Fatalf("plan: err=%v whole=%v", err, whole)
	}
	if !near(plan.Weight, 6) || !near(plan.Total, 239400) || !near(plan.Price, 39900) {
		t.Fatalf("issued line = %+v, want 6 @ 39,900 = 239,400", plan)
	}
	if len(plan.Cuts) != 1 || !near(plan.Cuts[0].Weight, 6) || !near(plan.Cuts[0].Total, 239400) {
		t.Fatalf("cuts = %+v", plan.Cuts)
	}
	leftW := sources[0].Weight - plan.Cuts[0].Weight
	leftA := sources[0].Total - plan.Cuts[0].Total
	if !near(leftW, 4) || !near(leftA, 159600) || !near(leftA/leftW, 39900) {
		t.Fatalf("remainder = %v baht / %v, want 4 / 159,600 at 39,900", leftW, leftA)
	}
}

// A second round on a bill that already carries a deduction line works off the
// NET figures, and the average still does not move.
func TestPlanWeightSplit_SecondRound(t *testing.T) {
	cut := entity.QuotationItem{ID: 4, Metal: "gold", Weight: -6, Price: 39900, Total: -239400}
	one := uint(99)
	cut.SplitBillID = &one
	bills := []entity.Quotation{{ID: 1, Metal: "gold", Items: []entity.QuotationItem{
		goldLine(1, 5, 39000), goldLine(2, 3, 40000), goldLine(3, 2, 42000), cut,
	}}}
	sources, _, _ := splitSources(bills)
	if !near(sources[0].Weight, 4) || !near(sources[0].Total, 159600) {
		t.Fatalf("net source = %+v", sources[0])
	}
	plan, whole, err := planWeightSplit(sources, 1.5)
	if err != nil || whole {
		t.Fatalf("plan: err=%v whole=%v", err, whole)
	}
	if !near(plan.Total, 59850) || !near(plan.Total/plan.Weight, 39900) {
		t.Fatalf("second round = %+v, want 1.5 @ 39,900 = 59,850", plan)
	}
}

// Two pending bills (e.g. a BILL and a P bill) at different averages: each is
// cut at its own average, and the cuts add up exactly to the issued line.
func TestPlanWeightSplit_TwoBillsKeepCombinedAverage(t *testing.T) {
	bills := []entity.Quotation{
		{ID: 1, Metal: "gold", Items: []entity.QuotationItem{goldLine(1, 7, 39000)}},
		{ID: 2, Metal: "gold", Items: []entity.QuotationItem{goldLine(2, 3, 41000)}},
	}
	sources, _, _ := splitSources(bills)
	plan, whole, err := planWeightSplit(sources, 3.3333)
	if err != nil || whole {
		t.Fatalf("plan: err=%v whole=%v", err, whole)
	}
	var w, a float64
	for _, c := range plan.Cuts {
		w += c.Weight
		a += c.Total
	}
	if !near(w, plan.Weight) || !near(a, plan.Total) {
		t.Fatalf("cuts %v / %v do not add up to issued %v / %v", w, a, plan.Weight, plan.Total)
	}
	combined := (7*39000.0 + 3*41000.0) / 10
	if math.Abs(plan.Total/plan.Weight-combined) > 0.01 {
		t.Fatalf("issued rate %v, want combined average %v", plan.Total/plan.Weight, combined)
	}
	// What is left must still average the same, or the next round locks a new price.
	leftW := 10 - w
	leftA := 7*39000.0 + 3*41000.0 - a
	if math.Abs(leftA/leftW-combined) > 0.01 {
		t.Fatalf("remainder rate %v, want %v", leftA/leftW, combined)
	}
	if !near(plan.Cuts[0].Price, 39000) || !near(plan.Cuts[1].Price, 41000) {
		t.Fatalf("each cut should carry its own bill's price: %+v", plan.Cuts)
	}
}

func TestPlanWeightSplit_Bounds(t *testing.T) {
	sources := []splitSource{{BillID: 1, Weight: 10, Total: 399000, PriceW: 399000}}
	if _, whole, err := planWeightSplit(sources, 10); err != nil || !whole {
		t.Fatalf("full weight should report whole: whole=%v err=%v", whole, err)
	}
	if _, whole, err := planWeightSplit(sources, 10.00004); err != nil || !whole {
		t.Fatalf("within rounding of full should report whole: whole=%v err=%v", whole, err)
	}
	if _, _, err := planWeightSplit(sources, 10.5); err == nil {
		t.Fatal("more than outstanding must be refused")
	}
	if _, _, err := planWeightSplit(sources, 0); err == nil {
		t.Fatal("zero must be refused")
	}
	if _, _, err := planWeightSplit([]splitSource{{BillID: 1}}, 1); err == nil {
		t.Fatal("a bill with nothing outstanding must be refused")
	}
}

// Silver locks on the price-weighted BASE price (baht/kg) while the totals carry
// purity and tier surcharges; both must survive the cut unchanged.
func TestPlanWeightSplit_Silver(t *testing.T) {
	bills := []entity.Quotation{{ID: 1, Metal: "silver", Items: []entity.QuotationItem{
		{ID: 1, Metal: "silver", Weight: 2000, Price: 60000, Percent: 99.9, Total: 119880},
		{ID: 2, Metal: "silver", Weight: 1000, Price: 63000, Percent: 99.9, Total: 62937},
		// A stray gold line on a legacy mixed bill is weighed in baht — ignored.
		goldLine(3, 1, 40000),
	}}}
	sources, metal, err := splitSources(bills)
	if err != nil || metal != "silver" || !near(sources[0].Weight, 3000) {
		t.Fatalf("sources %+v metal %q err %v", sources, metal, err)
	}
	plan, _, err := planWeightSplit(sources, 1500)
	if err != nil {
		t.Fatal(err)
	}
	if !near(plan.Price, 61000) || !near(plan.Total, 91408.5) || !near(plan.Percent, 99.9) {
		t.Fatalf("silver plan = %+v", plan)
	}
}

func TestSplitSources_MixedMetalBillsRefused(t *testing.T) {
	_, _, err := splitSources([]entity.Quotation{{ID: 1, Metal: "gold"}, {ID: 2, Metal: "silver"}})
	if err == nil {
		t.Fatal("gold and silver bills must not be split together")
	}
}
