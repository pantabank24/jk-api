package entity

import "time"

type Notification struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	UserID    uint      `json:"user_id" gorm:"not null;index"`
	Type      string    `json:"type" gorm:"type:varchar(50);not null"`
	Title     string    `json:"title" gorm:"type:varchar(255);not null"`
	Body      string    `json:"body" gorm:"type:text;default:''"`
	IsRead    bool      `json:"is_read" gorm:"default:false"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
