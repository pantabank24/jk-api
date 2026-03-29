package entity

import "time"

type QuotationImage struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	QuotationID uint      `json:"quotation_id" gorm:"not null;index"`
	ImageURL    string    `json:"image_url" gorm:"type:varchar(500);not null"`
	CreatedAt   time.Time `json:"created_at"`
}
