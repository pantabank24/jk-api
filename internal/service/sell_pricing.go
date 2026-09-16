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

// Config keys holding how far the price a customer submits may sit from the
// price the server derives at the same moment. The customer is paid the price
// they confirmed as long as it is inside the band; outside it the sell is
// refused. The band exists because the confirm dialog freezes the price for up
// to ten seconds while the realtime feed keeps moving — without it a curl call
// could name any price at all (BILL1740).
const (
	KeySellToleranceRealtime    = "sell_price_tolerance_realtime_thb"
	KeySellToleranceAssociation = "sell_price_tolerance_association_thb"
	KeySellToleranceSilver      = "sell_price_tolerance_silver_thb_per_kg"
)

// Accepted ranges. 0 means the prices must match exactly — unlike the auto-sell
// slippage setting, it never means "unlimited".
const (
	SellToleranceGoldMin   = 0.0
	SellToleranceGoldMax   = 1000.0
	SellToleranceSilverMin = 0.0
	SellToleranceSilverMax = 10000.0
)

// Defaults when a key was never saved. Realtime needs slack for the confirm
// dialog; the association and silver prices only change between rounds, so a
// mismatch there means the page was left open across one.
const (
	defaultSellToleranceRealtime    = 30.0
	defaultSellToleranceAssociation = 0.0
	defaultSellToleranceSilver      = 0.0
)

// Weight bounds the sell screen enforces (billCalculate.tsx). Gold is in บาททอง,
// silver in grams.
const (
	goldWeightMax    = 1000.0
	goldWeightStep   = 5.0
	silverWeightMinG = 1000.0
	silverWeightMaxG = 100000.0
)

// priceEpsilon absorbs float noise in JSON numbers so 68278 and 68278.0000001
// compare equal under a zero tolerance.
const priceEpsilon = 0.005

// SellRejection is a sell the server refused. Reason is written for the person
// who pressed the button; the other fields are evidence for the activity log.
type SellRejection struct {
	Reason string
	// Item is the index of the offending line in the payload.
	Item int
	// ServerPrice is set only when the refusal is about the price.
	ServerPrice float64
	ClientPrice float64
	Tolerance   float64
}

func (e *SellRejection) Error() string { return e.Reason }

// PricedSellLine records how one line was checked, for the sell log.
type PricedSellLine struct {
	// ServerPrice and Tolerance are nil for a staff sell, whose typed price is
	// deliberately not compared against the market.
	ServerPrice *float64
	Tolerance   *float64
	// What the browser computed, kept so a log can show when it disagreed with
	// the server's own arithmetic.
	ClientPerGram float64
	ClientTotal   float64
}

// SellPricing is a payload after the server has checked and re-priced it.
type SellPricing struct {
	Items []billUC.CreateBillItemRequest
	Lines []PricedSellLine
	// RealtimeBuy/RealtimeSell are set when a live tick priced a gold line, so
	// the bill's price round can be stamped from that very reading.
	RealtimeBuy  float64
	RealtimeSell float64
}

// SellPriceTolerance returns the configured band for a metal and price mode,
// clamped on read like the realtime pricing policy: a value written straight
// into the table never went past the API's validation.
func SellPriceTolerance(db *gorm.DB, metal, priceMode string) float64 {
	switch {
	case metal == "silver":
		return clamp(configFloat(db, KeySellToleranceSilver, defaultSellToleranceSilver),
			SellToleranceSilverMin, SellToleranceSilverMax)
	case priceMode == PriceModeRealtime:
		return clamp(configFloat(db, KeySellToleranceRealtime, defaultSellToleranceRealtime),
			SellToleranceGoldMin, SellToleranceGoldMax)
	default:
		return clamp(configFloat(db, KeySellToleranceAssociation, defaultSellToleranceAssociation),
			SellToleranceGoldMin, SellToleranceGoldMax)
	}
}

// PriceSellItems checks a sell payload and recomputes every amount the server
// can derive, so nothing that lands in a bill rests on arithmetic the browser
// did. The input slice is left untouched — callers log it as submitted.
//
// customer=true is the self-service flow: the type must be the one the sell
// screen offers, the weight must follow the screen's rules, and the price must
// sit within the configured band of the server's own price. A staff sell keeps
// its typed price (the screen lets staff override it) but still has its type,
// weight and totals checked.
func PriceSellItems(db *gorm.DB, items []billUC.CreateBillItemRequest, customer bool, priceMode string) (*SellPricing, error) {
	p := &sellPricer{db: db, customer: customer, priceMode: priceMode}
	out := &SellPricing{
		Items: make([]billUC.CreateBillItemRequest, len(items)),
		Lines: make([]PricedSellLine, len(items)),
	}
	for i, item := range items {
		line := PricedSellLine{ClientPerGram: item.PerGram, ClientTotal: item.Total}
		priced, err := p.price(i, item, &line)
		if err != nil {
			return nil, err
		}
		out.Items[i] = priced
		out.Lines[i] = line
	}
	if p.gold != nil && p.gold.realtime {
		out.RealtimeBuy, out.RealtimeSell = p.gold.barBuy, p.gold.barSell
	}
	return out, nil
}

// sellPricer caches the lookups one payload needs, so a multi-line sell reads
// the price and the schedules once.
type sellPricer struct {
	db        *gorm.DB
	customer  bool
	priceMode string

	gold         *goldQuote
	silverStatus *SilverSellStatus
	customWeight *CustomWeightStatus
}

func (p *sellPricer) price(i int, item billUC.CreateBillItemRequest, line *PricedSellLine) (billUC.CreateBillItemRequest, error) {
	reject := func(reason string) (billUC.CreateBillItemRequest, error) {
		return item, &SellRejection{Reason: reason, Item: i, ClientPrice: item.Price}
	}
	if !finite(item.Price, item.Weight, item.Percent, item.Plus) {
		return reject("ข้อมูลรายการขายไม่ถูกต้อง")
	}

	metal := item.Metal
	if metal == "" {
		metal = "gold"
	}
	var gt *entity.GoldType
	var err error
	switch metal {
	case "gold":
		gt, err = sellGoldType(p.db)
	case "silver":
		gt, err = sellSilverType(p.db)
	default:
		return reject("ไม่รองรับการขายโลหะชนิดนี้")
	}
	if err != nil {
		return reject("ไม่พบประเภทสินค้าที่เปิดขาย กรุณาติดต่อเจ้าหน้าที่")
	}
	// The screen only offers these two types. Any other id is a hand-built
	// request — and for silver, a different type would bring its own formula.
	if strings.TrimSpace(item.TypeID) != strconv.FormatUint(uint64(gt.ID), 10) {
		return reject("ประเภทสินค้าไม่ถูกต้อง กรุณารีเฟรชหน้าแล้วลองใหม่")
	}

	if metal == "gold" {
		return p.priceGold(i, item, gt, line)
	}
	return p.priceSilver(i, item, gt, line)
}

func (p *sellPricer) priceGold(i int, item billUC.CreateBillItemRequest, gt *entity.GoldType, line *PricedSellLine) (billUC.CreateBillItemRequest, error) {
	reject := func(reason string) (billUC.CreateBillItemRequest, error) {
		return item, &SellRejection{Reason: reason, Item: i, ClientPrice: item.Price}
	}

	// Customers use the ±5 stepper unless the custom-weight schedule is open;
	// staff always type the weight.
	stepped := false
	if p.customer {
		if p.customWeight == nil {
			st := GetCustomWeightStatus(p.db)
			p.customWeight = &st
		}
		stepped = !p.customWeight.Allowed
	}
	if reason := checkGoldWeight(item.Weight, stepped); reason != "" {
		return reject(reason)
	}

	if p.customer {
		if p.gold == nil {
			q, reason := currentGoldQuote(p.db, p.priceMode)
			if reason != "" {
				return reject(reason)
			}
			p.gold = q
		}
		server := p.gold.forSource(gt.PriceSource)
		tol := SellPriceTolerance(p.db, "gold", p.priceMode)
		if !priceWithinTolerance(item.Price, server, tol) {
			return item, &SellRejection{
				Reason:      fmt.Sprintf("ราคาเปลี่ยนแล้ว (ราคาปัจจุบัน %s บาท) กรุณากดขายใหม่", formatThaiAmount(server)),
				Item:        i,
				ServerPrice: server,
				ClientPrice: item.Price,
				Tolerance:   tol,
			}
		}
		line.ServerPrice, line.Tolerance = &server, &tol
	} else if item.Price <= 0 {
		return reject("ราคารับซื้อต้องมากกว่า 0")
	}

	perGram, total := goldLineAmounts(gt, item.Price, item.Weight)
	item.TypeName = gt.Name
	item.Metal = "gold"
	item.Percent = 0
	item.Plus = 0
	item.PerGram = perGram
	item.Total = total
	return item, nil
}

func (p *sellPricer) priceSilver(i int, item billUC.CreateBillItemRequest, gt *entity.GoldType, line *PricedSellLine) (billUC.CreateBillItemRequest, error) {
	reject := func(reason string) (billUC.CreateBillItemRequest, error) {
		return item, &SellRejection{Reason: reason, Item: i, ClientPrice: item.Price}
	}

	if reason := checkSilverWeight(item.Weight); reason != "" {
		return reject(reason)
	}
	if p.silverStatus == nil {
		st := GetSilverSellStatus(p.db)
		p.silverStatus = &st
	}
	tier := ResolveSilverTier(p.silverStatus.Tiers, item.Weight/1000)
	if tier == nil || tier.Blocked {
		return reject("น้ำหนักนี้ร้านไม่รับซื้อ กรุณาลดน้ำหนักหรือติดต่อเจ้าหน้าที่")
	}

	// A customer may lower the purity for scrap but never declare more than the
	// type's own figure (99.9 for เงินแท่ง). Staff can enter what they measured.
	maxPercent := 100.0
	if p.customer && gt.DefaultPercent > 0 {
		maxPercent = gt.DefaultPercent
	}
	if reason := checkSilverPercent(item.Percent, maxPercent); reason != "" {
		return reject(reason)
	}

	if p.customer {
		server, reason := currentSilverBase(p.db, p.silverStatus, gt)
		if reason != "" {
			return reject(reason)
		}
		tol := SellPriceTolerance(p.db, "silver", p.priceMode)
		if !priceWithinTolerance(item.Price, server, tol) {
			return item, &SellRejection{
				Reason:      fmt.Sprintf("ราคาเปลี่ยนแล้ว (ราคาปัจจุบัน %s บาท/กก.) กรุณากดขายใหม่", formatThaiAmount(server)),
				Item:        i,
				ServerPrice: server,
				ClientPrice: item.Price,
				Tolerance:   tol,
			}
		}
		line.ServerPrice, line.Tolerance = &server, &tol
	} else if item.Price <= 0 {
		return reject("ราคารับซื้อต้องมากกว่า 0")
	}

	// item.Price stays the base price the screen shows; the weight-tier
	// surcharge is folded into the amounts only (billCalculate.tsx handleAdd).
	perGram, total := silverLineAmounts(gt, item.Price, tier.AddPerKg, item.Percent, item.Weight)
	item.TypeName = gt.Name
	item.Metal = "silver"
	item.Plus = 0
	item.PerGram = perGram
	item.Total = total
	return item, nil
}

// sellGoldType finds the type the sell screen offers for gold: the first type,
// in the order /gold-types lists them, named as a 96.5% bar.
func sellGoldType(db *gorm.DB) (*entity.GoldType, error) {
	var gt entity.GoldType
	err := db.Where("metal = ? AND name LIKE ? AND name LIKE ?", "gold", "%แท่ง%", "%96.5%").
		Order("sort_order ASC, id ASC").First(&gt).Error
	return &gt, err
}

// sellSilverType finds the first silver type, as the sell screen does.
func sellSilverType(db *gorm.DB) (*entity.GoldType, error) {
	var gt entity.GoldType
	err := db.Where("metal = ?", "silver").Order("sort_order ASC, id ASC").First(&gt).Error
	return &gt, err
}

// goldQuote is one reading of the gold price in the shape the sell screen
// indexes by a type's price_source.
type goldQuote struct {
	barBuy, barSell           float64
	ornamentBuy, ornamentSell float64
	realtime                  bool
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
// mode. Returns a customer-facing reason when there is none to compare with.
func currentGoldQuote(db *gorm.DB, priceMode string) (*goldQuote, string) {
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
		return &goldQuote{barBuy: buy, barSell: sell, ornamentBuy: buy, ornamentSell: sell, realtime: true}, ""
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

func priceWithinTolerance(client, server, tolerance float64) bool {
	return math.Abs(client-server) <= tolerance+priceEpsilon
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
