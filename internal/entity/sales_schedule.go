package entity

import "time"

// SalesSchedule is a per-weekday rule or a datetime-range rule that overrides
// the default sales window. Resolution precedence: range rule (covering now) >
// weekday rule > default config keys.
type SalesSchedule struct {
	ID                 uint       `json:"id" gorm:"primaryKey"`
	Scope              string     `json:"scope" gorm:"type:varchar(10);not null"` // weekday|range
	Weekday            *int       `json:"weekday"`                                // 0=Sun..6=Sat
	StartAt            *time.Time `json:"start_at"`                               // range start
	EndAt              *time.Time `json:"end_at"`                                 // range end
	Enabled            bool       `json:"enabled" gorm:"default:true"`
	OpenTime           string     `json:"open_time" gorm:"type:varchar(5);default:'09:30'"`
	CloseTime          string     `json:"close_time" gorm:"type:varchar(5);default:'16:30'"`
	RealtimeAfterHours bool       `json:"realtime_after_hours" gorm:"default:false"`
	Note               string     `json:"note" gorm:"type:varchar(255);default:''"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

func (SalesSchedule) TableName() string { return "sales_schedules" }
