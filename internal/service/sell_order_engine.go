package service

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"jk-api/internal/entity"
	billUC "jk-api/internal/module/bill/usecase"
	notificationRepo "jk-api/internal/module/notification/repository"
	sellOrderRepo "jk-api/internal/module/sell_order/repository"

	"gorm.io/gorm"
)

// SellOrderEngine fills customers' auto-sell orders: every tick it reads the live
// price, claims the orders whose target it has reached, and turns each into a
// normal "รอออกบิล" bill flagged auto_sell.
//
// Correctness rests on three things:
//   - the claim is a single UPDATE ... RETURNING, so no order is filled twice even
//     if a tick overruns into the next one;
//   - a tick that finds the feed stale or the shop closed fills nothing (a stale
//     price is the one failure a human seller would catch and the engine cannot);
//   - an order left 'filling' by a crash is resolved on the next boot against
//     whether its bill actually exists.
//
// jk-api runs as a single container (the Next.js app is the clustered one), so one
// in-process ticker is the whole engine. Should the API ever be scaled out, the
// claim's SKIP LOCKED keeps two engines from double-filling, but the tick would
// then run N times as often — revisit the interval before doing that.
type SellOrderEngine struct {
	db        *gorm.DB
	orderRepo sellOrderRepo.SellOrderRepository
	billUC    billUC.BillUsecase
	notifRepo notificationRepo.NotificationRepository

	mu      sync.Mutex
	stop    chan struct{}
	running bool
	// tickBusy guards against a slow tick overlapping the next one. The claim
	// already makes that safe; this just stops the pile-up.
	tickBusy sync.Mutex
	// lastSkip dedupes the "why nothing fired" log so a closed shop or a dead feed
	// does not write a line every few seconds all night.
	lastSkip string
	// slippageWarned remembers which orders already logged a slippage skip. An
	// order sitting far above its target is re-claimed and released on EVERY tick,
	// which would otherwise flood the log with the same line all day.
	slippageWarned map[uint]bool
}

func NewSellOrderEngine(
	db *gorm.DB,
	orderRepo sellOrderRepo.SellOrderRepository,
	billUsecase billUC.BillUsecase,
	notifRepo notificationRepo.NotificationRepository,
) *SellOrderEngine {
	return &SellOrderEngine{
		db: db, orderRepo: orderRepo, billUC: billUsecase, notifRepo: notifRepo,
		slippageWarned: map[uint]bool{},
	}
}

// Start recovers anything left mid-fill and begins ticking. Safe to call twice.
func (e *SellOrderEngine) Start() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.running {
		return
	}
	e.recoverStuck()

	cfg := GetAutoSellConfig(e.db)
	e.stop = make(chan struct{})
	e.running = true
	go e.loop(e.stop, time.Duration(cfg.TickSeconds)*time.Second)
	log.Printf("⏱  Auto-sell engine started (tick: %ds, enabled: %v)", cfg.TickSeconds, cfg.Enabled)
}

// Stop halts the ticker. A tick already in flight is allowed to finish so it
// never leaves an order stuck in 'filling' on a graceful shutdown.
func (e *SellOrderEngine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.running {
		return
	}
	close(e.stop)
	e.running = false
	e.tickBusy.Lock()
	e.tickBusy.Unlock()
	log.Println("⏹  Auto-sell engine stopped")
}

// Reload restarts the ticker so a changed auto_sell_tick_seconds takes effect
// without a deploy. Called when the settings page saves that key.
func (e *SellOrderEngine) Reload() {
	e.Stop()
	e.Start()
}

func (e *SellOrderEngine) loop(stop chan struct{}, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			e.Tick()
		}
	}
}

// Tick runs one matching pass. Exported so the settings page can trigger a
// deliberate pass ("ตรวจเดี๋ยวนี้") instead of waiting out the interval.
func (e *SellOrderEngine) Tick() {
	if !e.tickBusy.TryLock() {
		return // previous tick still running
	}
	defer e.tickBusy.Unlock()

	cfg := GetAutoSellConfig(e.db)
	tick, tickErr := FetchRealtimeTick()

	if reason := autoSellGate(e.db, cfg, tick, tickErr); reason != "" {
		e.noteSkip(reason)
		return
	}

	pricing := GetRealtimePricing(e.db)
	_, buy, sell := pricing.Quote(tick.Spot, tick.USDTHB)
	if buy <= 0 || sell <= 0 {
		e.noteSkip("ราคาเรียลไทม์คำนวณไม่ได้")
		return
	}
	e.lastSkip = ""

	claimed, err := e.orderRepo.ClaimDue(buy)
	if err != nil {
		log.Printf("⚠️  Auto-sell: claiming due orders failed: %v", err)
		return
	}
	if len(claimed) == 0 {
		return
	}

	// One price round per tick, shared by every bill it produces: they were all
	// matched against this same reading, so they must all point at one row.
	round, priceID := SaveRealtimeRound(e.db, buy, sell)

	for i := range claimed {
		e.fill(&claimed[i], buy, cfg, round, priceID)
	}
}

// noteSkip logs why a tick filled nothing, but only when the reason changes.
func (e *SellOrderEngine) noteSkip(reason string) {
	if e.lastSkip == reason {
		return
	}
	e.lastSkip = reason
	log.Printf("⏸  Auto-sell idle: %s", reason)
}

// fill turns one claimed order into a bill. Every exit path either completes the
// order or releases the claim — an order must never be left in 'filling'.
func (e *SellOrderEngine) fill(order *entity.SellOrder, price float64, cfg AutoSellConfig, round string, priceID *uint) {
	// Slippage: the market can jump past a target between ticks. The shop sells at
	// the price actually captured (both are recorded), but a jump beyond the
	// configured tolerance is more likely a bad feed reading than a real move, so
	// the order goes back on the market instead of filling at a suspect price.
	if cfg.MaxSlippageTHB > 0 && price-order.TargetPrice > cfg.MaxSlippageTHB {
		// Logged once per order: while the price stays this far above the target the
		// order is re-claimed and released every single tick.
		if !e.slippageWarned[order.ID] {
			e.slippageWarned[order.ID] = true
			log.Printf("⚠️  Auto-sell: order #%d held — price %.2f exceeds target %.2f by more than %.2f (raise auto_sell_max_slippage_thb or cancel the order)",
				order.ID, price, order.TargetPrice, cfg.MaxSlippageTHB)
		}
		e.release(order.ID)
		return
	}
	delete(e.slippageWarned, order.ID)

	goldType, err := e.loadGoldType(order)
	if err != nil {
		log.Printf("⚠️  Auto-sell: order #%d skipped — %v", order.ID, err)
		e.release(order.ID)
		return
	}

	// Price it exactly as the sell screen does: per-gram comes from the type's
	// formula at weight=1, and the line total for gold is price × weight because
	// gold is quoted per baht-weight (billCalculate.tsx's `total`).
	perGram, _ := goldType.ComputeItem(price, 0, 0, 1)
	total := price * order.Weight

	req := &billUC.CreateBillRequest{
		StoreID:         order.StoreID,
		BranchID:        order.BranchID,
		CreatedByUserID: order.UserID,
		GoldRound:       round,
		GoldPriceID:     priceID,
		AutoSell:        true,
		SellOrderID:     &order.ID,
		Note:            fmt.Sprintf("ขายอัตโนมัติ (เป้า %s บาท)", formatThaiAmount(order.TargetPrice)),
		Items: []billUC.CreateBillItemRequest{{
			TypeID:   fmt.Sprintf("%d", goldType.ID),
			TypeName: goldType.Name,
			Metal:    "gold",
			Plus:     0,
			Price:    price,
			Percent:  0,
			Weight:   order.Weight,
			PerGram:  perGram,
			Total:    total,
		}},
	}

	bill, err := e.billUC.CreateBill(req)
	if err != nil {
		log.Printf("⚠️  Auto-sell: order #%d bill creation failed: %v", order.ID, err)
		e.release(order.ID)
		return
	}

	if err := e.orderRepo.MarkFilled(order.ID, price, bill.ID); err != nil {
		// The bill exists; only the bookkeeping failed. Leave the row in 'filling'
		// so the boot recovery finds the bill and completes it — releasing here
		// would let the next tick create a SECOND bill for the same order.
		log.Printf("❌ Auto-sell: order #%d filled as bill %s but marking it failed: %v",
			order.ID, bill.Code, err)
		return
	}

	log.Printf("✅ Auto-sell: order #%d filled at %.2f (target %.2f) → bill %s",
		order.ID, price, order.TargetPrice, bill.Code)

	e.notifyFilled(order, bill, price)
	// Keep the shop's LINE alerts honest: the engine just added a bill nobody
	// pressed a button for — it lands at รอออกบิล like any other sell, so it
	// counts towards the customer's pending pile too.
	go SyncLineBacklogAlert(e.db, "gold", bill.StoreID, true)
	go SyncPendingSellAlert(e.db, order.UserID, "gold")
	e.logActivity(order, bill, price)
}

func (e *SellOrderEngine) release(id uint) {
	if err := e.orderRepo.ReleaseClaim(id); err != nil {
		log.Printf("❌ Auto-sell: releasing claim on order #%d failed: %v", id, err)
	}
}

// loadGoldType re-reads the pricing type at fill time (rather than trusting a
// snapshot from when the order was placed) so an edited formula applies to the
// sale, exactly as it would for a manual one.
func (e *SellOrderEngine) loadGoldType(order *entity.SellOrder) (*entity.GoldType, error) {
	var gt entity.GoldType
	if order.GoldTypeID != nil {
		if err := e.db.First(&gt, *order.GoldTypeID).Error; err == nil {
			return &gt, nil
		}
	}
	err := e.db.Where("metal = ? AND is_active = ? AND name LIKE ? AND name LIKE ?",
		"gold", true, "%แท่ง%", "%96.5%").Order("sort_order ASC").First(&gt).Error
	if err != nil {
		return nil, fmt.Errorf("ไม่พบประเภททองคำแท่ง 96.5%% สำหรับคิดราคา")
	}
	return &gt, nil
}

func (e *SellOrderEngine) notifyFilled(order *entity.SellOrder, bill *entity.Quotation, price float64) {
	body := fmt.Sprintf("ราคารับซื้อถึง %s บาท ระบบขายทอง %s บาท ให้อัตโนมัติแล้ว ยอดรวม %s บาท (บิล %s)",
		formatThaiAmount(price), formatWeight(order.Weight),
		formatThaiAmount(bill.TotalAmount), bill.Code)
	if price > order.TargetPrice {
		body += fmt.Sprintf(" · สูงกว่าเป้าที่ตั้งไว้ %s บาท", formatThaiAmount(order.TargetPrice))
	}
	_ = e.notifRepo.Create(&entity.Notification{
		UserID: order.UserID,
		Type:   "auto_sell_filled",
		Title:  "ขายอัตโนมัติสำเร็จ",
		Body:   body,
	})
}

// logActivity records the fill in the same audit trail as human actions. The
// engine has no request context, so the row is written directly with the customer
// as the actor and the engine's route as the path.
func (e *SellOrderEngine) logActivity(order *entity.SellOrder, bill *entity.Quotation, price float64) {
	userID := order.UserID
	_ = e.db.Create(&entity.ActivityLog{
		UserID: &userID,
		Method: "SYSTEM",
		Path:   "/auto-sell/fill",
		Description: fmt.Sprintf("ขายอัตโนมัติ %s บาท ที่ราคา %s (เป้า %s) → บิล %s",
			formatWeight(order.Weight), formatThaiAmount(price),
			formatThaiAmount(order.TargetPrice), bill.Code),
		StatusCode: 200,
		CreatedAt:  time.Now(),
	}).Error
}

// recoverStuck resolves orders left in 'filling' by a crash or a hard restart. The
// bill is the source of truth: if one exists the fill did happen and only the
// bookkeeping is missing; if not, the order never sold and goes back on the market.
func (e *SellOrderEngine) recoverStuck() {
	stuck, err := e.orderRepo.FindStuckFilling()
	if err != nil {
		log.Printf("⚠️  Auto-sell: checking for interrupted fills failed: %v", err)
		return
	}
	for _, order := range stuck {
		bill, findErr := e.orderRepo.FindBillBySellOrder(order.ID)
		if findErr == nil && bill != nil {
			price := bill.TotalAmount
			if order.Weight > 0 {
				price = bill.TotalAmount / order.Weight
			}
			if err := e.orderRepo.MarkFilled(order.ID, price, bill.ID); err != nil {
				log.Printf("❌ Auto-sell: completing interrupted order #%d failed: %v", order.ID, err)
				continue
			}
			log.Printf("🔧 Auto-sell: order #%d completed from interrupted fill (bill %s)", order.ID, bill.Code)
			continue
		}
		if err := e.orderRepo.ReleaseClaim(order.ID); err != nil {
			log.Printf("❌ Auto-sell: releasing interrupted order #%d failed: %v", order.ID, err)
			continue
		}
		log.Printf("🔧 Auto-sell: order #%d returned to waiting (interrupted before a bill was created)", order.ID)
	}
}

// formatThaiAmount renders a money figure with thousands separators, matching how
// the customer sees amounts on screen.
func formatThaiAmount(v float64) string {
	intPart, decPart, _ := strings.Cut(fmt.Sprintf("%.2f", v), ".")
	neg := strings.HasPrefix(intPart, "-")
	intPart = strings.TrimPrefix(intPart, "-")

	var b strings.Builder
	for i, c := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	out := b.String() + "." + decPart
	if neg {
		return "-" + out
	}
	return out
}

func formatWeight(v float64) string { return fmt.Sprintf("%g", v) }
