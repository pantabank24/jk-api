package entity

import (
	"time"

	"gorm.io/gorm"
)

type Quotation struct {
	ID          uint            `json:"id" gorm:"primaryKey"`
	StoreID     *uint           `json:"store_id" gorm:"index"`
	Store       *Store          `json:"store,omitempty" gorm:"foreignKey:StoreID"`
	BranchID    *uint           `json:"branch_id" gorm:"index"`
	Branch      *Branch         `json:"branch,omitempty" gorm:"foreignKey:BranchID"`
	MemberID    *uint           `json:"member_id" gorm:"index"`
	Member      *Member         `json:"member,omitempty" gorm:"foreignKey:MemberID"`
	CreatedBy   *uint           `json:"created_by" gorm:"index"`
	Creator     *User           `json:"creator,omitempty" gorm:"foreignKey:CreatedBy"`
	Code         string          `json:"code"          gorm:"type:varchar(20);uniqueIndex;not null"`
	Status       int             `json:"status"        gorm:"default:0;index"`
	IsBill       bool            `json:"is_bill"       gorm:"default:false;index"`
	Note         string          `json:"note"          gorm:"type:text;default:''"`
	RejectReason string          `json:"reject_reason" gorm:"type:text;default:''"`
	TotalAmount  float64         `json:"total_amount"  gorm:"type:decimal(12,2);default:0"`
	GoldRound    string          `json:"gold_round"    gorm:"type:varchar(50);default:''"`
	GoldPriceID  *uint           `json:"gold_price_id" gorm:"index"`
	SignerName   string          `json:"signer_name"   gorm:"type:varchar(255);default:''"`
	SignerPhone  string          `json:"signer_phone"  gorm:"type:varchar(30);default:''"`
	PDPAConsent  bool            `json:"pdpa_consent"  gorm:"default:false"`
	// BillID links a master-issued quotation back to the customer's bill it was
	// created for. IssuedQuotation is the reverse: a bill's issued quotation
	// (loaded on demand so the customer can view it). Not persisted.
	BillID          *uint       `json:"bill_id" gorm:"index"`
	IssuedQuotationID *uint     `json:"issued_quotation_id" gorm:"index"`
	IssuedQuotation *Quotation  `json:"issued_quotation,omitempty" gorm:"foreignKey:IssuedQuotationID;references:ID"`
	Items       []QuotationItem  `json:"items,omitempty" gorm:"foreignKey:QuotationID"`
	Images      []QuotationImage `json:"images,omitempty" gorm:"foreignKey:QuotationID"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	DeletedAt   gorm.DeletedAt  `json:"-" gorm:"index"`
}
