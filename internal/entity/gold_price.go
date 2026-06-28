package entity

import "time"

type GoldPrice struct {
	ID              uint      `json:"id"               gorm:"primaryKey"`
	BarBuy          float64   `json:"bar_buy"          gorm:"type:decimal(12,2);default:0"`
	BarSell         float64   `json:"bar_sell"         gorm:"type:decimal(12,2);default:0"`
	OrnamentBuy     float64   `json:"ornament_buy"     gorm:"type:decimal(12,2);default:0"`
	OrnamentSell    float64   `json:"ornament_sell"    gorm:"type:decimal(12,2);default:0"`
	ChangeToday     float64   `json:"change_today"     gorm:"type:decimal(10,2);default:0"`
	ChangeYesterday float64   `json:"change_yesterday" gorm:"type:decimal(10,2);default:0"`
	GoldDate        string    `json:"gold_date"        gorm:"type:varchar(100);default:''"`
	GoldTime        string    `json:"gold_time"        gorm:"type:varchar(50);default:''"`
	GoldRound       string    `json:"gold_round"       gorm:"type:varchar(50);default:''"`
	Source          string    `json:"source"           gorm:"type:varchar(20);default:'auto'"` // auto|manual
	// Validity window for manual overrides (null for auto rows).
	ValidFrom  *time.Time `json:"valid_from"`
	ValidUntil *time.Time `json:"valid_until"`
	CreatedAt  time.Time  `json:"created_at"`
}
