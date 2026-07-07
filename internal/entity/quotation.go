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
	// ProcessedWeight/ProcessedAmount track partial deliveries by the master
	// (รอส่งเพิ่ม). Accumulated each time the master records a batch of melted gold
	// without issuing the full quotation yet. Only meaningful for bills (is_bill=true).
	ProcessedWeight float64 `json:"processed_weight" gorm:"type:decimal(10,4);default:0"`
	ProcessedAmount float64 `json:"processed_amount" gorm:"type:decimal(14,2);default:0"`
	// CreditsRefunded tracks whether the credit charged for this quotation on
	// approval has been returned to the creator's member profile (via reject's
	// refund_credits, edit's adjust_credits, or the bulk credit-reset action).
	CreditsRefunded bool `json:"credits_refunded" gorm:"default:false"`
	// Store header snapshot — copied from the Store/Branch at creation time so
	// reprinting an old quotation later (after the store's info changes) still
	// shows the header as it was on the day it was issued, instead of live-joining
	// the Store relation above (which always reflects current data).
	StoreName    string `json:"store_name"    gorm:"type:varchar(255);default:''"`
	StoreBranch  string `json:"store_branch"  gorm:"type:varchar(255);default:''"`
	StoreAddress string `json:"store_address" gorm:"type:text;default:''"`
	StorePhone   string `json:"store_phone"   gorm:"type:varchar(20);default:''"`
	StoreTaxID   string `json:"store_tax_id"  gorm:"type:varchar(50);default:''"`
	StoreTaxName string `json:"store_tax_name" gorm:"type:varchar(255);default:''"`
	StoreWebsite string `json:"store_website" gorm:"type:varchar(255);default:''"`
	StoreLogo    string `json:"store_logo"    gorm:"type:varchar(500);default:''"`
	// NoHeader marks a document intentionally issued without a receipt header —
	// readers must NOT fall back to the live store relation (that fallback is
	// only for legacy quotations that predate the snapshot columns).
	NoHeader     bool   `json:"no_header"     gorm:"default:false"`
	Items       []QuotationItem  `json:"items,omitempty" gorm:"foreignKey:QuotationID"`
	Images      []QuotationImage `json:"images,omitempty" gorm:"foreignKey:QuotationID"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	DeletedAt   gorm.DeletedAt  `json:"-" gorm:"index"`
}
