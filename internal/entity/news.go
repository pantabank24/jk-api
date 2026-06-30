package entity

import (
	"time"

	"gorm.io/gorm"
)

type News struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	Title     string         `json:"title" gorm:"type:varchar(255);not null"`
	Body      string         `json:"body" gorm:"type:text;not null"`
	ImageURL  string         `json:"image_url" gorm:"type:varchar(500);default:''"`
	// Audience: "all" | "customer" | "staff" (owner+employee) — used to filter
	// what shows up on each role's home page (see news usecase GetVisibleNews).
	Audience  string         `json:"audience" gorm:"type:varchar(20);not null;default:'all'"`
	CreatedBy *uint          `json:"created_by" gorm:"index"`
	Creator   *User          `json:"creator,omitempty" gorm:"foreignKey:CreatedBy"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}
