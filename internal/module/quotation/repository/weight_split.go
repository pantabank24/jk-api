package repository

import (
	"errors"
	"fmt"
	"math"

	"jk-api/internal/entity"
)

// weightEpsilon is half the last stored decimal of a weight (decimal(12,4)): two
// weights closer than this are the same weight once saved.
const weightEpsilon = 0.00005

// ErrBillNotPending is returned when a bill picked for a partial issuance has
// already left รอออกบิล — issued from another screen, deleted, or reverted into a
// different group — between the master opening the page and pressing save.
var ErrBillNotPending = errors.New("บิลบางใบไม่อยู่ในสถานะ 'รอออกบิล' แล้ว กรุณาเปิดหน้าออกบิลใหม่")

// splitSource is one pending bill's lines of the bill's own metal, summed. The
// sums are NET: a deduction line left by an earlier partial round is part of
// them, which is exactly what makes them describe what is still outstanding.
type splitSource struct {
	BillID   uint
	Weight   float64 // Σ weight
	Total    float64 // Σ total
	PriceW   float64 // Σ price × weight — silver's lock averages the base price
	PercentW float64 // Σ percent × weight
	PerGramW float64 // Σ per_gram × weight
}

// splitCut is what one bill gives up this round. Its deduction line carries the
// negation of Weight/Total; Price/Percent/PerGram are that bill's own averages.
type splitCut struct {
	BillID  uint
	Weight  float64
	Total   float64
	Price   float64
	Percent float64
	PerGram float64
}

// splitPlan is the whole round: the cut from each bill, and the single line the
// issued bill receives (the sum of the cuts, at the averages of all the bills).
type splitPlan struct {
	Cuts    []splitCut
	Weight  float64
	Total   float64
	Price   float64
	Percent float64
	PerGram float64
}

func round(v float64, places int) float64 {
	f := math.Pow(10, float64(places))
	return math.Round(v*f) / f
}

func billMetal(m string) string {
	if m == "" {
		return "gold"
	}
	return m
}

// splitSources sums each bill's lines of that bill's metal. Bills are
// single-metal; a legacy mixed bill's stray other-metal lines are weighed in a
// different unit and must not be averaged in — they stay where they are.
func splitSources(bills []entity.Quotation) ([]splitSource, string, error) {
	if len(bills) == 0 {
		return nil, "", errors.New("ไม่พบบิลที่จะออก")
	}
	metal := billMetal(bills[0].Metal)
	sources := make([]splitSource, 0, len(bills))
	for _, b := range bills {
		if billMetal(b.Metal) != metal {
			return nil, "", errors.New("ออกใบบางส่วนได้เฉพาะบิลโลหะเดียวกัน")
		}
		s := splitSource{BillID: b.ID}
		for _, it := range b.Items {
			if billMetal(it.Metal) != metal {
				continue
			}
			s.Weight += it.Weight
			s.Total += it.Total
			s.PriceW += it.Price * it.Weight
			s.PercentW += it.Percent * it.Weight
			s.PerGramW += it.PerGram * it.Weight
		}
		sources = append(sources, s)
	}
	return sources, metal, nil
}

// planWeightSplit shares `weight` out over the bills in proportion to what each
// still holds, cutting every bill at ITS OWN average. That keeps each bill's
// average unchanged, so the remainder as a whole keeps the combined average — the
// ราคาถัว this round is priced at — and the next round locks the same price.
// Taking whole bills oldest-first would not: the leftover bill's average would
// differ, and the customer would be paid a different rate for the rest.
//
// For the usual single bill this is simply "−weight at the bill's average".
//
// whole=true means the weight covers everything outstanding; the caller then
// issues the bills whole instead of splitting them.
func planWeightSplit(sources []splitSource, weight float64) (plan splitPlan, whole bool, err error) {
	if weight <= 0 {
		return plan, false, errors.New("น้ำหนักที่ออกใบต้องมากกว่า 0")
	}
	var live []splitSource
	var totalWeight, totalAmount, priceW, percentW, perGramW float64
	for _, s := range sources {
		if s.Weight <= weightEpsilon {
			continue // nothing outstanding to take from
		}
		live = append(live, s)
		totalWeight += s.Weight
		totalAmount += s.Total
		priceW += s.PriceW
		percentW += s.PercentW
		perGramW += s.PerGramW
	}
	if totalWeight <= weightEpsilon {
		return plan, false, errors.New("บิลนี้ไม่มีน้ำหนักคงค้างให้ออกใบ")
	}
	if weight > totalWeight+weightEpsilon {
		return plan, false, fmt.Errorf("น้ำหนักที่ออกใบ (%s) เกินที่ลูกค้าส่งมาในบิล (%s)",
			trimWeight(weight), trimWeight(totalWeight))
	}
	if weight >= totalWeight-weightEpsilon {
		return plan, true, nil
	}

	weight = round(weight, 4)
	amount := round(weight*totalAmount/totalWeight, 2)
	f := weight / totalWeight

	// Every bill but the last is cut by rounding its exact share; the last takes
	// whatever is left, so the cuts always add up to exactly the issued line.
	var cutWeight, cutAmount float64
	for i, s := range live {
		c := splitCut{
			BillID:  s.BillID,
			Price:   round(s.PriceW/s.Weight, 2),
			Percent: round(s.PercentW/s.Weight, 4),
			PerGram: round(s.PerGramW/s.Weight, 2),
		}
		if i == len(live)-1 {
			c.Weight = round(weight-cutWeight, 4)
			c.Total = round(amount-cutAmount, 2)
		} else {
			c.Weight = round(f*s.Weight, 4)
			c.Total = round(f*s.Total, 2)
		}
		if c.Weight == 0 && c.Total == 0 {
			continue
		}
		cutWeight += c.Weight
		cutAmount += c.Total
		plan.Cuts = append(plan.Cuts, c)
	}

	plan.Weight = weight
	plan.Total = amount
	plan.Price = round(priceW/totalWeight, 2)
	plan.Percent = round(percentW/totalWeight, 4)
	plan.PerGram = round(perGramW/totalWeight, 2)
	return plan, false, nil
}

// trimWeight prints a weight without trailing zeros (6, 6.5, 6.125).
func trimWeight(w float64) string {
	return fmt.Sprintf("%g", round(w, 4))
}
