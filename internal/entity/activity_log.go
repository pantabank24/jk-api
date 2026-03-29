package entity

import "time"

type ActivityLog struct {
	ID         uint      `gorm:"primarykey"              json:"id"`
	UserID     *uint     `gorm:"index"                   json:"user_id"`
	User       *User     `gorm:"foreignKey:UserID"       json:"user,omitempty"`
	Method     string    `                               json:"method"`
	Path       string    `                               json:"path"`
	StatusCode int       `                               json:"status_code"`
	IP         string    `                               json:"ip"`
	UserAgent  string    `                               json:"user_agent"`
	DurationMs int64     `                               json:"duration_ms"`
	CreatedAt  time.Time `                               json:"created_at"`
}
