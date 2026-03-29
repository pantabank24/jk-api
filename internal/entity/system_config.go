package entity

import "time"

type SystemConfig struct {
	ID          uint      `json:"id"          gorm:"primaryKey"`
	Key         string    `json:"key"         gorm:"type:varchar(100);uniqueIndex;not null"`
	Value       string    `json:"value"       gorm:"type:text;default:''"`
	Description string    `json:"description" gorm:"type:varchar(500);default:''"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
