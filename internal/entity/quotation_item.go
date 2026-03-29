package entity

import "time"

type QuotationItem struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	QuotationID uint      `json:"quotation_id" gorm:"not null;index"`
	TypeID      string    `json:"type_id" gorm:"type:varchar(50);default:''"`
	TypeName    string    `json:"type_name" gorm:"type:varchar(100);not null"`
	Plus        float64   `json:"plus" gorm:"type:decimal(12,2);default:0"`
	Price       float64   `json:"price" gorm:"type:decimal(12,2);default:0"`
	Percent     float64   `json:"percent" gorm:"type:decimal(8,4);default:0"`
	Weight      float64   `json:"weight" gorm:"type:decimal(12,4);default:0"`
	PerGram     float64   `json:"per_gram" gorm:"type:decimal(12,2);default:0"`
	Total       float64   `json:"total" gorm:"type:decimal(12,2);default:0"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
