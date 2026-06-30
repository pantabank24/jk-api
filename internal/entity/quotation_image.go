package entity

import (
	"time"

	"gorm.io/gorm"
)

type QuotationImage struct {
	ID          uint   `json:"id" gorm:"primaryKey"`
	QuotationID uint   `json:"quotation_id" gorm:"not null;index"`
	ImageURL    string `json:"image_url" gorm:"type:varchar(500);not null"`
	// Type categorises the image: before_melt, after_melt, signature, or "" (legacy/other).
	Type      string         `json:"type" gorm:"type:varchar(50);default:''"`
	CreatedAt time.Time      `json:"created_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}
