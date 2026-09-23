package entity

import (
	"time"

	"gorm.io/gorm"
)

type QuotationItem struct {
	ID          uint    `json:"id" gorm:"primaryKey"`
	QuotationID uint    `json:"quotation_id" gorm:"not null;index"`
	TypeID      string  `json:"type_id" gorm:"type:varchar(50);default:''"`
	TypeName    string  `json:"type_name" gorm:"type:varchar(100);not null"`
	Metal       string  `json:"metal" gorm:"type:varchar(20);default:'gold'"`
	Plus        float64 `json:"plus" gorm:"type:decimal(12,2);default:0"`
	Price       float64 `json:"price" gorm:"type:decimal(12,2);default:0"`
	Percent     float64 `json:"percent" gorm:"type:decimal(8,4);default:0"`
	Weight      float64 `json:"weight" gorm:"type:decimal(12,4);default:0"`
	PerGram     float64 `json:"per_gram" gorm:"type:decimal(12,2);default:0"`
	Total       float64 `json:"total" gorm:"type:decimal(12,2);default:0"`
	// SellOrderID marks the line as one the auto-sell engine sold, and names the
	// order it filled. A fill accumulates into the customer's open bill like a
	// manual sell, so this — not the bill's auto_sell flag — is what says which
	// lines nobody pressed a button for. Nil on every line entered by hand.
	SellOrderID *uint `json:"sell_order_id,omitempty" gorm:"index"`
	// SplitBillID marks the NEGATIVE line a partial (by-weight) issuance leaves in
	// the customer's pending bill, and names the bill that took that portion to be
	// issued. The pending bill keeps every line the customer sold; this line is what
	// makes its sum the amount still outstanding. It must never be deleted or edited
	// on its own — that would hand the customer back weight already issued.
	SplitBillID *uint          `json:"split_bill_id,omitempty" gorm:"index"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
}
