package entity

import "time"

// LinePendingSellAlert is the anti-spam latch for the "ลูกค้ามีของรอออกบิลถึงเกณฑ์"
// LINE alert: how many whole lots of this customer's pending pile have already
// been announced. One row per (customer, metal) — the same split every other
// LINE rule uses.
type LinePendingSellAlert struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	UserID      uint      `json:"user_id" gorm:"not null;uniqueIndex:line_pending_sell_alerts_user_metal_key"`
	Metal       string    `json:"metal" gorm:"type:varchar(20);not null;uniqueIndex:line_pending_sell_alerts_user_metal_key"`
	AlertedLots int       `json:"alerted_lots" gorm:"not null;default:0"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (LinePendingSellAlert) TableName() string { return "line_pending_sell_alerts" }
