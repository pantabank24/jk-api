package entity

import "time"

// MetalPrice is the latest-wins price snapshot for a non-gold metal (silver via
// cron). Mirrors GoldPrice: a new row is inserted per fetch; the latest row for
// a symbol is the current price.
type MetalPrice struct {
	ID          uint      `json:"id"           gorm:"primaryKey"`
	Symbol      string    `json:"symbol"       gorm:"type:varchar(10);not null;index"` // XAG, XPT, XPD
	Buy         float64   `json:"buy"          gorm:"type:decimal(12,2);default:0"`
	Sell        float64   `json:"sell"         gorm:"type:decimal(12,2);default:0"`
	Spot        float64   `json:"spot"         gorm:"type:decimal(12,2);default:0"`
	Exchange    float64   `json:"exchange"     gorm:"type:decimal(12,4);default:0"`
	Previous    float64   `json:"previous"     gorm:"type:decimal(12,2);default:0"`
	ChangeToday float64   `json:"change_today" gorm:"type:decimal(10,2);default:0"`
	PriceDate   string    `json:"price_date"   gorm:"type:varchar(100);default:''"`
	PriceTime   string    `json:"price_time"   gorm:"type:varchar(50);default:''"`
	Round       string    `json:"round"        gorm:"type:varchar(50);default:''"`
	Source      string    `json:"source"       gorm:"type:varchar(20);default:'auto'"` // auto|manual
	// Validity window for manual overrides (null for auto rows).
	ValidFrom  *time.Time `json:"valid_from"`
	ValidUntil *time.Time `json:"valid_until"`
	CreatedAt  time.Time  `json:"created_at"`
}
