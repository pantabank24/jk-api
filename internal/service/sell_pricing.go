package service

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"jk-api/internal/entity"
	billUC "jk-api/internal/module/bill/usecase"
	goldPriceRepo "jk-api/internal/module/gold_price/repository"
	metalPriceRepo "jk-api/internal/module/metal_price/repository"

	"gorm.io/gorm"
)

// Weight bounds the sell screen enforces (billCalculate.tsx). Gold is in บาททอง,
// silver in grams.
const (
	goldWeightMax    = 1000.0
	goldWeightStep   = 5.0
	silverWeightMinG = 1000.0
	silverWeightMaxG = 100000.0
)

// SellRejection is a sale the server refused. Reason is written for the person
// who pressed the button; Item points at the line in a staff payload.
type SellRejection struct {
	Reason string
	Item   int
}

func (e *SellRejection) Error() string { return e.Reason }

func reject(reason string) error { return &SellRejection{Reason: reason} }

// SellQuoteRequest is what the customer's screen chose: a product and how much
// of it. Deliberately no price — that is the server's to decide.
type SellQuoteRequest struct {
	TypeID  string  `json:"type_id"`
	Metal   string  `json:"metal"`
	Weight  float64 `json:"weight"`
	Percent float64 `json:"percent"`
}

// SellQuote is a priced line the server stands behind.
type SellQuote struct {
	GoldTypeID uint
	TypeID     string
	TypeName   string
	Metal      string
	Weight     float64
	Percent    float64
	// Price is per บาททอง for gold and per kilogram for silver (the base price
	// the screen shows; a weight-tier surcharge is folded into the amounts).
	Price     float64
	PerGram   float64
	Total     float64
	PriceMode string
	// RealtimeBuy/RealtimeSell are set when a live tick priced the line.
	RealtimeBuy  float64
	RealtimeSell float64
}

// QuoteSell prices one customer sale from the shop's own feed: the type must be
// the one the sell screen offers, the weight must follow the screen's rules, and
// the price comes from the server, never from the request.
func QuoteSell(db *gorm.DB, req SellQuoteRequest) (*SellQuote, error) {
	metal := req.Metal
	if metal == "" {
		metal = "gold"
	}
	if !finite(req.Weight, req.Percent) {
		return nil, reject("ข้อมูลรายการขายไม่ถูกต้อง")
	}

	gt, err := sellTypeFor(db, metal)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.TypeID) != "" && strings.TrimSpace(req.TypeID) != strconv.FormatUint(uint64(gt.ID), 10) {
		return nil, reject("ประเภทสินค้าไม่ถูกต้อง กรุณารีเฟรชหน้าแล้วลองใหม่")
	}

	quote := &SellQuote{
		GoldTypeID: gt.ID,
		TypeID:     strconv.FormatUint(uint64(gt.ID), 10),
		TypeName:   gt.Name,
		Metal:      metal,
		Weight:     req.Weight,
	}

	if metal == "gold" {
		// Customers use the ±5 stepper unless the custom-weight schedule is open.
		if reason := checkGoldWeight(req.Weight, !GetCustomWeightStatus(db).Allowed); reason != "" {
			return nil, reject(reason)
		}
		mode := GetSalesStatus(db).PriceMode
		gq, reason := currentGoldQuote(db, mode)
		if reason != "" {
			return nil, reject(reason)
		}
		quote.PriceMode = mode
		quote.Price = gq.forSource(gt.PriceSource)
		if quote.Price <= 0 {
			return nil, reject("ยังไม่มีข้อมูลราคาทอง กรุณาติดต่อเจ้าหน้าที่")
		}
		quote.RealtimeBuy, quote.RealtimeSell = gq.realtimeBuy, gq.realtimeSell
		quote.PerGram, quote.Total = goldLineAmounts(gt, quote.Price, req.Weight)
		return quote, nil
	}

	if metal != "silver" {
		return nil, reject("ไม่รองรับการขายโลหะชนิดนี้")
	}
	if reason := checkSilverWeight(req.Weight); reason != "" {
		return nil, reject(reason)
	}
	status := GetSilverSellStatus(db)
	tier := ResolveSilverTier(status.Tiers, req.Weight/1000)
	if tier == nil || tier.Blocked {
		return nil, reject("น้ำหนักนี้ร้านไม่รับซื้อ กรุณาลดน้ำหนักหรือติดต่อเจ้าหน้าที่")
	}
	// A customer may declare a lower purity for scrap but never more than the
	// type's own figure (99.9 for เงินแท่ง).
	maxPercent := 100.0
	if gt.DefaultPercent > 0 {
		maxPercent = gt.DefaultPercent
	}
	percent := req.Percent
	if percent == 0 {
		percent = gt.DefaultPercent
	}
	if reason := checkSilverPercent(percent, maxPercent); reason != "" {
		return nil, reject(reason)
	}
	base, reason := currentSilverBase(db, &status, gt)
	if reason != "" {
		return nil, reject(reason)
	}
	quote.Percent = percent
	quote.Price = base
	quote.PriceMode = PriceModeAssociation
	quote.PerGram, quote.Total = silverLineAmounts(gt, base, tier.AddPerKg, percent, req.Weight)
	return quote, nil
}

// PriceStaffItems checks a staff sale and recomputes every amount. Staff keep
// the price they typed — the sell screen lets them override it — but the type,
// the weight and the arithmetic are the server's.
func PriceStaffItems(db *gorm.DB, items []billUC.CreateBillItemRequest) ([]billUC.CreateBillItemRequest, error) {
	out := make([]billUC.CreateBillItemRequest, len(items))
	var silverStatus *SilverSellStatus
	for i, item := range items {
		fail := func(reason string) error { return &SellRejection{Reason: reason, Item: i} }
		if !finite(item.Price, item.Weight, item.Percent, item.Plus) {
			return nil, fail("ข้อมูลรายการขายไม่ถูกต้อง")
		}
		metal := item.Metal
		if metal == "" {
			metal = "gold"
		}
		gt, err := sellTypeFor(db, metal)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(item.TypeID) != strconv.FormatUint(uint64(gt.ID), 10) {
			return nil, fail("ประเภทสินค้าไม่ถูกต้อง กรุณารีเฟรชหน้าแล้วลองใหม่")
		}
		if item.Price <= 0 {
			return nil, fail("ราคารับซื้อต้องมากกว่า 0")
		}

		item.TypeName = gt.Name
		item.Metal = metal
		item.Plus = 0
		switch metal {
		case "gold":
			// Staff type the weight rather than stepping it.
			if reason := checkGoldWeight(item.Weight, false); reason != "" {
				return nil, fail(reason)
			}
			item.Percent = 0
			item.PerGram, item.Total = goldLineAmounts(gt, item.Price, item.Weight)
		case "silver":
			if reason := checkSilverWeight(item.Weight); reason != "" {
				return nil, fail(reason)
			}
			if silverStatus == nil {
				st := GetSilverSellStatus(db)
				silverStatus = &st
			}
			tier := ResolveSilverTier(silverStatus.Tiers, item.Weight/1000)
			if tier == nil || tier.Blocked {
				return nil, fail("น้ำหนักนี้ร้านไม่รับซื้อ กรุณาลดน้ำหนักหรือติดต่อเจ้าหน้าที่")
			}
			if reason := checkSilverPercent(item.Percent, 100); reason != "" {
				return nil, fail(reason)
			}
			item.PerGram, item.Total = silverLineAmounts(gt, item.Price, tier.AddPerKg, item.Percent, item.Weight)
		default:
			return nil, fail("ไม่รองรับการขายโลหะชนิดนี้")
		}
		out[i] = item
	}
	return out, nil
}

// sellTypeFor resolves the type the sell screen offers for a metal: the first
// one, in the order /gold-types lists them, that the screen would pick.
func sellTypeFor(db *gorm.DB, metal string) (*entity.GoldType, error) {
	var gt entity.GoldType
	var err error
	switch metal {
	case "gold":
		err = db.Where("metal = ? AND name LIKE ? AND name LIKE ?", "gold", "%แท่ง%", "%96.5%").
			Order("sort_order ASC, id ASC").First(&gt).Error
	case "silver":
		err = db.Where("metal = ?", "silver").Order("sort_order ASC, id ASC").First(&gt).Error
	default:
		return nil, reject("ไม่รองรับการขายโลหะชนิดนี้")
	}
	if err != nil {
		return nil, reject("ไม่พบประเภทสินค้าที่เปิดขาย กรุณาติดต่อเจ้าหน้าที่")
	}
	return &gt, nil
}

// goldQuote is one reading of the gold price in the shape the sell screen
// indexes by a type's price_source.
type goldQuote struct {
	barBuy, barSell           float64
	ornamentBuy, ornamentSell float64
	// realtimeBuy/realtimeSell are non-zero only for a live reading, which the
	// document's price round is stamped from.
	realtimeBuy, realtimeSell float64
}

// forSource mirrors the screen's sourceMap, which falls back to bar_buy.
func (q goldQuote) forSource(source string) float64 {
	switch source {
	case "bar_sell":
		return q.barSell
	case "ornament_buy":
		return q.ornamentBuy
	case "ornament_sell":
		return q.ornamentSell
	}
	return q.barBuy
}

// currentGoldQuote reads the price the customer's screen shows in this price
// mode. Returns a customer-facing reason when there is none.
func currentGoldQuote(db *gorm.DB, priceMode string) (*goldQuote, string) {
	if priceMode == PriceModeClosed {
		return nil, "ขณะนี้ปิดทำการ (ทอง) ไม่สามารถขายได้"
	}
	if priceMode == PriceModeRealtime {
		tick, err := FetchRealtimeTick()
		if err != nil {
			return nil, "เชื่อมต่อราคาเรียลไทม์ไม่ได้ กรุณาลองใหม่อีกครั้ง"
		}
		_, buy, sell := GetRealtimePricing(db).Quote(tick.Spot, tick.USDTHB)
		if buy <= 0 || sell <= 0 {
			return nil, "เชื่อมต่อราคาเรียลไทม์ไม่ได้ กรุณาลองใหม่อีกครั้ง"
		}
		// The screen shows the bar figures for ornaments too while realtime.
		return &goldQuote{
			barBuy: buy, barSell: sell, ornamentBuy: buy, ornamentSell: sell,
			realtimeBuy: buy, realtimeSell: sell,
		}, ""
	}
	// /gold-prices/latest: a manual price inside its window, else the latest auto.
	gp, err := goldPriceRepo.NewGoldPriceRepository(db).GetLatest()
	if err != nil {
		return nil, "ยังไม่มีข้อมูลราคาทอง กรุณาติดต่อเจ้าหน้าที่"
	}
	return &goldQuote{
		barBuy: gp.BarBuy, barSell: gp.BarSell,
		ornamentBuy: gp.OrnamentBuy, ornamentSell: gp.OrnamentSell,
	}, ""
}

// currentSilverBase is the base buy price (baht/kg) the silver screen shows:
// the shop's manual price, or the XAG feed field named by the type.
func currentSilverBase(db *gorm.DB, status *SilverSellStatus, gt *entity.GoldType) (float64, string) {
	const noPrice = "ยังไม่มีข้อมูลราคาเงิน กรุณาติดต่อเจ้าหน้าที่"
	var base float64
	if status.PriceMode == "manual" {
		base = status.ManualPrice
	} else {
		mp, err := metalPriceRepo.NewMetalPriceRepository(db).GetLatest("XAG")
		if err != nil {
			return 0, noPrice
		}
		switch gt.PriceSource {
		case "sell":
			base = mp.Sell
		case "spot":
			base = mp.Spot
		default:
			base = mp.Buy
		}
	}
	if base <= 0 {
		return 0, noPrice
	}
	return base, ""
}

// ResolveSilverTier picks the weight tier for a single sale of kg kilograms: the
// first tier whose upper bound covers it, the catch-all (nil bound) last. nil
// means the weight is past every bound and there is no catch-all — not sellable.
// Mirrors resolveSilverTier in billCalculate.tsx.
func ResolveSilverTier(tiers []SilverTier, kg float64) *SilverTier {
	if len(tiers) == 0 {
		return &SilverTier{}
	}
	sorted := make([]SilverTier, len(tiers))
	copy(sorted, tiers)
	sort.SliceStable(sorted, func(a, b int) bool {
		if sorted[a].UpToKg == nil {
			return false
		}
		if sorted[b].UpToKg == nil {
			return true
		}
		return *sorted[a].UpToKg < *sorted[b].UpToKg
	})
	for i := range sorted {
		if sorted[i].UpToKg == nil || kg <= *sorted[i].UpToKg {
			return &sorted[i]
		}
	}
	return nil
}

// checkGoldWeight applies the sell screen's weight rules. stepped is the
// customer ±5 stepper; otherwise the weight is typed as a whole number.
func checkGoldWeight(w float64, stepped bool) string {
	if stepped {
		if w < goldWeightStep || w > goldWeightMax || math.Mod(w, goldWeightStep) != 0 {
			return "น้ำหนักต้องเป็นขั้นละ 5 บาท ตั้งแต่ 5 ถึง 1,000 บาท"
		}
		return ""
	}
	if w < 1 || w > goldWeightMax || w != math.Trunc(w) {
		return "น้ำหนักต้องเป็นจำนวนเต็มตั้งแต่ 1 ถึง 1,000 บาท"
	}
	return ""
}

func checkSilverWeight(w float64) string {
	if w < silverWeightMinG || w > silverWeightMaxG {
		return "น้ำหนักเงินต้องอยู่ระหว่าง 1,000 ถึง 100,000 กรัม"
	}
	return ""
}

func checkSilverPercent(percent, max float64) string {
	if percent <= 0 || percent > max+1e-9 {
		return fmt.Sprintf("เปอร์เซ็นต์ความบริสุทธิ์ต้องมากกว่า 0 และไม่เกิน %s%%", strconv.FormatFloat(max, 'f', -1, 64))
	}
	return ""
}

// goldLineAmounts prices a gold line the way the sell screen and the auto-sell
// engine do: per-gram from the type's formula at weight 1, and the total as
// price × weight because gold bars are quoted per บาททอง.
func goldLineAmounts(gt *entity.GoldType, price, weight float64) (perGram, total float64) {
	perGram, _ = gt.ComputeItem(price, 0, 0, 1)
	return perGram, price * weight
}

// silverLineAmounts adds the weight-tier surcharge to the base price before the
// type's formula (price ÷ 1000 × % for เงินแท่ง), as the screen does.
func silverLineAmounts(gt *entity.GoldType, basePrice, addPerKg, percent, weight float64) (perGram, total float64) {
	return gt.ComputeItem(basePrice+addPerKg, percent, 0, weight)
}

func finite(vs ...float64) bool {
	for _, v := range vs {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return false
		}
	}
	return true
}
