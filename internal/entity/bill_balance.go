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
	Description string         `json:"description"  gorm:"type:text;default:''"`
	CreatedAt   time.Time      `json:"created_at"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
}
