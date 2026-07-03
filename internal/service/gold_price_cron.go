package service

import (
	"log"
	"sync"
	"time"

	"jk-api/internal/entity"
	configRepo "jk-api/internal/module/config/repository"
	goldPriceRepo "jk-api/internal/module/gold_price/repository"
	metalPriceRepo "jk-api/internal/module/metal_price/repository"
	"jk-api/pkg/goldprice"
	"jk-api/pkg/silverprice"

	"github.com/robfig/cron/v3"
)

// GoldPriceCron manages the metal price scraping cron job (gold + silver).
// It reads config from DB and can be reloaded at runtime.
type GoldPriceCron struct {
	mu         sync.Mutex
	cron       *cron.Cron
	priceRepo  goldPriceRepo.GoldPriceRepository
	metalRepo  metalPriceRepo.MetalPriceRepository
	configRepo configRepo.ConfigRepository
}

func NewGoldPriceCron(priceRepo goldPriceRepo.GoldPriceRepository, metalRepo metalPriceRepo.MetalPriceRepository, cfgRepo configRepo.ConfigRepository) *GoldPriceCron {
	return &GoldPriceCron{
		priceRepo:  priceRepo,
		metalRepo:  metalRepo,
		configRepo: cfgRepo,
	}
}

// Start loads config and starts the cron job.
func (g *GoldPriceCron) Start() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.stop()
	g.start()
}

// Reload stops and restarts the cron job with fresh config.
func (g *GoldPriceCron) Reload() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.stop()
	g.start()
	log.Println("🔄 Gold price cron reloaded")
}

func (g *GoldPriceCron) stop() {
	if g.cron != nil {
		g.cron.Stop()
		g.cron = nil
	}
}

func (g *GoldPriceCron) start() {
	// Read config. Gold is polled often (default every minute) so a new
	// association round shows up within ~1 min; each poll is cheap thanks to
	// conditional GET + dedup. Silver keeps its own, slower cadence.
	autoFetch := true
	goldExpr := "* * * * *"
	silverExpr := "*/30 * * * *"

	if cfg, err := g.configRepo.GetByKey("gold_price_auto_fetch"); err == nil {
		autoFetch = cfg.Value == "true"
	}
	if cfg, err := g.configRepo.GetByKey("gold_price_cron"); err == nil && cfg.Value != "" {
		goldExpr = cfg.Value
	}
	if cfg, err := g.configRepo.GetByKey("silver_price_cron"); err == nil && cfg.Value != "" {
		silverExpr = cfg.Value
	}

	if !autoFetch {
		log.Println("⏸  Gold price auto-fetch is disabled")
		return
	}

	// A bad expression in DB config must never take the whole cron down — fall
	// back to the safe default so metal prices keep being fetched even if someone
	// saves a malformed schedule (e.g. "*/* * * * *").
	const goldDefault = "* * * * *"
	const silverDefault = "*/30 * * * *"

	g.cron = cron.New()
	if _, err := g.cron.AddFunc(goldExpr, g.fetchAndSave); err != nil {
		log.Printf("❌ Invalid gold cron expression '%s': %v — falling back to '%s'", goldExpr, err, goldDefault)
		goldExpr = goldDefault
		if _, err := g.cron.AddFunc(goldExpr, g.fetchAndSave); err != nil {
			log.Printf("❌ Gold cron default '%s' also failed: %v", goldExpr, err)
		}
	}
	if _, err := g.cron.AddFunc(silverExpr, g.fetchSilver); err != nil {
		log.Printf("❌ Invalid silver cron expression '%s': %v — falling back to '%s'", silverExpr, err, silverDefault)
		silverExpr = silverDefault
		if _, err := g.cron.AddFunc(silverExpr, g.fetchSilver); err != nil {
			log.Printf("❌ Silver cron default '%s' also failed: %v", silverExpr, err)
		}
	}
	g.cron.Start()
	log.Printf("⏰ Metal price cron started (gold: %s, silver: %s)", goldExpr, silverExpr)
}

func (g *GoldPriceCron) fetchAndSave() {
	data, err := goldprice.Fetch()
	if err != nil {
		log.Printf("⚠️  Gold price fetch failed: %v", err)
		return
	}
	// Skip the insert when the association hasn't published a new price — with
	// minute-level polling this makes almost every tick a cheap no-op instead of
	// bloating the table with duplicate rows.
	if prev, err := g.priceRepo.GetLatestAuto(); err == nil && prev != nil &&
		prev.BarBuy == data.BarBuy && prev.BarSell == data.BarSell &&
		prev.OrnamentBuy == data.OrnamentBuy && prev.OrnamentSell == data.OrnamentSell &&
		prev.GoldRound == data.GoldRound && prev.GoldTime == data.GoldTime &&
		prev.GoldDate == data.GoldDate {
		return
	}
	gp := &entity.GoldPrice{
		BarBuy:          data.BarBuy,
		BarSell:         data.BarSell,
		OrnamentBuy:     data.OrnamentBuy,
		OrnamentSell:    data.OrnamentSell,
		ChangeToday:     data.ChangeToday,
		ChangeYesterday: data.ChangeYesterday,
		GoldDate:        data.GoldDate,
		GoldTime:        data.GoldTime,
		GoldRound:       data.GoldRound,
		CreatedAt:       time.Now(),
	}
	if err := g.priceRepo.Create(gp); err != nil {
		log.Printf("⚠️  Gold price save failed: %v", err)
		return
	}
	log.Printf("✅ Gold price saved: bar_buy=%.2f bar_sell=%.2f", gp.BarBuy, gp.BarSell)
}

// fetchSilver pulls the latest silver bar price and stores it under symbol XAG.
func (g *GoldPriceCron) fetchSilver() {
	if g.metalRepo == nil {
		return
	}
	data, err := silverprice.Fetch()
	if err != nil {
		log.Printf("⚠️  Silver price fetch failed: %v", err)
		return
	}
	mp := &entity.MetalPrice{
		Symbol:    "XAG",
		Buy:       data.Buy,
		Sell:      data.Sell,
		Spot:      data.Spot,
		Exchange:  data.Exchange,
		Previous:  data.Previous,
		Round:     data.Round,
		PriceDate: data.Date,
		Source:    "auto",
		CreatedAt: time.Now(),
	}
	if err := g.metalRepo.Create(mp); err != nil {
		log.Printf("⚠️  Silver price save failed: %v", err)
		return
	}
	log.Printf("✅ Silver price saved: buy=%.2f sell=%.2f", mp.Buy, mp.Sell)
}
