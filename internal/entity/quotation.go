package entity

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

type Quotation struct {
	ID        uint    `json:"id" gorm:"primaryKey"`
	StoreID   *uint   `json:"store_id" gorm:"index"`
	Store     *Store  `json:"store,omitempty" gorm:"foreignKey:StoreID"`
	BranchID  *uint   `json:"branch_id" gorm:"index"`
	Branch    *Branch `json:"branch,omitempty" gorm:"foreignKey:BranchID"`
	MemberID  *uint   `json:"member_id" gorm:"index"`
	Member    *Member `json:"member,omitempty" gorm:"foreignKey:MemberID"`
	CreatedBy *uint   `json:"created_by" gorm:"index"`
	Creator   *User   `json:"creator,omitempty" gorm:"foreignKey:CreatedBy"`
	Code      string  `json:"code"          gorm:"type:varchar(20);uniqueIndex;not null"`
	// DisplayCode is the stable, user-facing number. An Admin-issued quotation
	// linked to an existing bill keeps the originating bill's number in previews,
	// even though the quotation row has its own internal unique Code.
	DisplayCode string `json:"display_code,omitempty" gorm:"-"`
	Status      int    `json:"status"        gorm:"default:0;index"`
	IsBill      bool   `json:"is_bill"       gorm:"default:false;index"`
	// Metal tags the whole document (gold|silver|…). Bills are single-metal: a new
	// sell only accumulates into an open "รอออกบิล" bill of the SAME metal, and the
	// รายการขายทอง / รายการขายเงิน lists filter on this. Legacy mixed bills read 'gold'.
	Metal        string  `json:"metal"         gorm:"type:varchar(20);default:'gold';index"`
	Note         string  `json:"note"          gorm:"type:text;default:''"`
	RejectReason string  `json:"reject_reason" gorm:"type:text;default:''"`
	TotalAmount  float64 `json:"total_amount"  gorm:"type:decimal(12,2);default:0"`
	GoldRound    string  `json:"gold_round"    gorm:"type:varchar(50);default:''"`
	GoldPriceID  *uint   `json:"gold_price_id" gorm:"index"`
	SignerName   string  `json:"signer_name"   gorm:"type:varchar(255);default:''"`
	SignerPhone  string  `json:"signer_phone"  gorm:"type:varchar(30);default:''"`
	PDPAConsent  bool    `json:"pdpa_consent"  gorm:"default:false"`
	// PaymentMethod is the "ชำระโดย" tick on the printed document:
	// "" (ยังไม่ระบุ) | "cash" | "transfer". Persisted so a reprint shows what was chosen.
	PaymentMethod string `json:"payment_method" gorm:"type:varchar(20);default:''"`
	// BillID links a master-issued quotation back to the customer's bill it was
	// created for. IssuedQuotation is the reverse: a bill's issued quotation
	// (loaded on demand so the customer can view it). Not persisted.
	BillID            *uint      `json:"bill_id" gorm:"index"`
	IssuedQuotationID *uint      `json:"issued_quotation_id" gorm:"index"`
	IssuedQuotation   *Quotation `json:"issued_quotation,omitempty" gorm:"foreignKey:IssuedQuotationID;references:ID"`
	// AutoSell marks a bill the auto-sell engine created when a customer's target
	// price was reached (SellOrderID links back to the order). Set only on bills.
	// It is a display/audit flag: such a bill is otherwise an ordinary "รอออกบิล"
	// bill and follows the same issue/approve flow.
	AutoSell    bool  `json:"auto_sell" gorm:"default:false;index"`
	SellOrderID *uint `json:"sell_order_id" gorm:"index"`
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
	NoHeader bool            `json:"no_header"     gorm:"default:false"`
	Items    []QuotationItem `json:"items,omitempty" gorm:"foreignKey:QuotationID"`
	// Page1Items holds the detailed per-item lines (JSON array) as issued, so the
	// printed quotation's page 1 can always list each item — even though `Items`
	// above is stored consolidated (one line per metal). Kept on the issued
	// quotation itself so it survives regardless of partial-ticking / delivery logs.
	Page1Items json.RawMessage  `json:"page1_items,omitempty" gorm:"type:jsonb"`
	Images     []QuotationImage `json:"images,omitempty" gorm:"foreignKey:QuotationID"`
	// AfterMeltClearedAt removes a quotation from the after-melt upload queue
	// without pretending that a photo was uploaded. Nil means it still requires
	// an after-melt photo unless one already exists in Images.
	AfterMeltClearedAt *time.Time `json:"after_melt_cleared_at,omitempty" gorm:"index"`
	CreatedAt  time.Time        `json:"created_at"`
	UpdatedAt  time.Time        `json:"updated_at"`
	// StatusChangedAt คือเวลาที่ Status ถูกเปลี่ยนครั้งล่าสุด — คนละอย่างกับ UpdatedAt
	// ซึ่งขยับทุกครั้งที่มีการเขียนอะไรก็ได้ลงแถวนี้ (แก้โน้ต, บันทึกส่งของเพิ่ม, ฯลฯ)
	// แม้บิลจะปิดไปแล้ว. ลิสต์บิลแท็บ ยกเลิก เรียงด้วยค่านี้ เพราะบิลที่ถูกยกเลิกไม่มีใบเสนอ
	// ราคาให้ยึดวันที่ ส่วนแท็บที่ออกบิลไปแล้วเรียงตามวันที่ออกใบเสนอราคาและถอยมาใช้ค่านี้
	// เป็น fallback (ดู listOrder ใน bill_repository.go)
	StatusChangedAt *time.Time     `json:"status_changed_at" gorm:"index"`
	DeletedAt       gorm.DeletedAt `json:"-" gorm:"index"`
}

// TouchStatus stamps the moment the status changed. Call it wherever Status is
// assigned, right next to the assignment, so the two never drift apart.
func (q *Quotation) TouchStatus() {
	now := time.Now()
	q.StatusChangedAt = &now
}
