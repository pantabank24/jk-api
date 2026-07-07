package entity

import (
	"time"

	"gorm.io/gorm"
)

// BillBalance records the running debt/credit balance for a customer that arises
// when a master-issued quotation amount differs from the customer's locked total.
// Positive amount = credit (customer gets more next round).
// Negative amount = debt (customer gets less next round).
type BillBalance struct {
	ID          uint           `json:"id"           gorm:"primaryKey"`
	UserID      uint           `json:"user_id"      gorm:"index;not null"`
	StoreID     *uint          `json:"store_id"`
	QuotationID *uint          `json:"quotation_id"`
	Amount      float64        `json:"amount"       gorm:"type:decimal(14,2);default:0"`
	Weight      float64        `json:"weight"       gorm:"type:decimal(12,4);default:0"`
	AvgPrice    float64        `json:"avg_price"    gorm:"type:decimal(14,4);default:0"`
	Description string         `json:"description"  gorm:"type:text;default:''"`
	// Set by เคลียร์บิล: a settled row is kept for history but no longer
	// contributes to the customer's balance / average-price calculation.
	SettledAt *time.Time `json:"settled_at"`
	CreatedAt time.Time  `json:"created_at"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
}
