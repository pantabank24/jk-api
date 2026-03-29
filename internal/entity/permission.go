package entity

import "time"

type Permission struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	Code        string    `json:"code" gorm:"type:varchar(100);uniqueIndex;not null"`
	Name        string    `json:"name" gorm:"type:varchar(255);not null"`
	GroupName   string    `json:"group_name" gorm:"type:varchar(100);not null;index"`
	Description string    `json:"description" gorm:"type:text;default:''"`
	CreatedAt   time.Time `json:"created_at"`
}
