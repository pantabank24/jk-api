package service

import (
	"log"
	"sync"
	"time"

	"jk-api/internal/entity"
	goldPriceRepo "jk-api/internal/module/gold_price/repository"
	configRepo "jk-api/internal/module/config/repository"
	"jk-api/pkg/goldprice"

	"github.com/robfig/cron/v3"
)

// GoldPriceCron manages the gold price scraping cron job.
// It reads config from DB and can be reloaded at runtime.
type GoldPriceCron struct {
	mu         sync.Mutex
	cron       *cron.Cron
	priceRepo  goldPriceRepo.GoldPriceRepository
	configRepo configRepo.ConfigRepository
}

func NewGoldPriceCron(priceRepo goldPriceRepo.GoldPriceRepository, cfgRepo configRepo.ConfigRepository) *GoldPriceCron {
	return &GoldPriceCron{
		priceRepo:  priceRepo,
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
	// Read config
	autoFetch := true
	cronExpr := "*/30 * * * *"

	if cfg, err := g.configRepo.GetByKey("gold_price_auto_fetch"); err == nil {
		autoFetch = cfg.Value == "true"
	}
	if cfg, err := g.configRepo.GetByKey("gold_price_cron"); err == nil && cfg.Value != "" {
		cronExpr = cfg.Value
	}

	if !autoFetch {
		log.Println("⏸  Gold price auto-fetch is disabled")
		return
	}

	g.cron = cron.New()
	_, err := g.cron.AddFunc(cronExpr, func() {
		g.fetchAndSave()
	})
	if err != nil {
		log.Printf("❌ Invalid cron expression '%s': %v", cronExpr, err)
		return
	}
	g.cron.Start()
	log.Printf("⏰ Gold price cron started with expression: %s", cronExpr)
}

func (g *GoldPriceCron) fetchAndSave() {
	data, err := goldprice.Fetch()
	if err != nil {
		log.Printf("⚠️  Gold price fetch failed: %v", err)
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
