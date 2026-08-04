package entity

import "time"

// LinePendingSellAlert is the anti-spam latch for the "มีของรอออกบิลถึงเกณฑ์"
// LINE alert: how many whole lots of the shop's pending pile have already been
// announced. One row per metal — the pile is counted across every customer's
// bills together, and ทอง/เงิน are weighed and announced separately.
type LinePendingSellAlert struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	Metal       string    `json:"metal" gorm:"type:varchar(20);not null;uniqueIndex:line_pending_sell_alerts_metal_key"`
	AlertedLots int       `json:"alerted_lots" gorm:"not null;default:0"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (LinePendingSellAlert) TableName() string { return "line_pending_sell_alerts" }
