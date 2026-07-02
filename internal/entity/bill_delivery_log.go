package entity

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

// BillDeliveryLog records each individual partial-delivery session ("รอส่งเพิ่ม")
// so the master and staff can review every melt batch for a bill.
type BillDeliveryLog struct {
	ID        uint           `json:"id"         gorm:"primaryKey"`
	BillID    uint           `json:"bill_id"    gorm:"index;not null"`
	Weight    float64        `json:"weight"     gorm:"type:decimal(10,4);default:0"`
	Amount    float64        `json:"amount"     gorm:"type:decimal(14,2);default:0"`
	Note      string         `json:"note"       gorm:"type:text;default:''"`
	// Items holds the round's itemised quote lines (JSON array) so the detailed
	// preview can list each item even after reload / when reprinted later.
	Items     json.RawMessage `json:"items"      gorm:"type:jsonb;default:'[]'"`
	CreatedAt time.Time      `json:"created_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}
