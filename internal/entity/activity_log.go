package entity

import "time"

type ActivityLog struct {
	ID         uint      `gorm:"primarykey"              json:"id"`
	UserID     *uint     `gorm:"index"                   json:"user_id"`
	User       *User     `gorm:"foreignKey:UserID"       json:"user,omitempty"`
	Method     string    `                               json:"method"`
	Path       string    `                               json:"path"`
	// Description is an optional human-readable summary of the business action
	// (e.g. "อนุมัติใบเสนอราคา P2607001") set by the controller via
	// middleware.SetActivityDescription. Empty when the route doesn't set one —
	// the frontend falls back to showing the raw method+path in that case.
	Description string    `                               json:"description"`
	StatusCode  int       `                               json:"status_code"`
	IP          string    `                               json:"ip"`
	UserAgent   string    `                               json:"user_agent"`
	DurationMs  int64     `                               json:"duration_ms"`
	CreatedAt   time.Time `                               json:"created_at"`
}
