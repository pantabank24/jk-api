package entity

import "time"

type LoginLog struct {
	ID         uint      `gorm:"primarykey"              json:"id"`
	UserID     *uint     `gorm:"index"                   json:"user_id"`
	User       *User     `gorm:"foreignKey:UserID"       json:"user,omitempty"`
	Email      string    `                               json:"email"`
	IP         string    `                               json:"ip"`
	UserAgent  string    `                               json:"user_agent"`
	Device     string    `                               json:"device"`
	Success    bool      `                               json:"success"`
	FailReason string    `                               json:"fail_reason"`
	CreatedAt  time.Time `                               json:"created_at"`
}
