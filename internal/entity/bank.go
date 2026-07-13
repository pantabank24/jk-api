package entity

import "time"

// Bank is a payout destination a customer's account can belong to. Managed by the
// shop (settings → ธนาคาร) rather than hard-coded, so the list can change without a deploy.
type Bank struct {
	ID        uint      `json:"id"         gorm:"primaryKey"`
	Name      string    `json:"name"       gorm:"type:varchar(100);not null"`
	Code      string    `json:"code"       gorm:"type:varchar(20);default:''"` // e.g. KBANK, SCB
	SortOrder int       `json:"sort_order" gorm:"default:0"`
	IsActive  bool      `json:"is_active"  gorm:"default:true"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
