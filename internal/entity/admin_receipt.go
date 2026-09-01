package entity

import (
	"time"

	"gorm.io/gorm"
)

// ReceiptSettings is the single row (id = 1) behind every printed receipt header:
// the parts of the form that never change from one receipt to the next. Edited
// from the ตั้งค่าเริ่มต้น modal on the receipts list.
type ReceiptSettings struct {
	ID uint8 `json:"id" gorm:"primaryKey"`
	// Relative path under /uploads, e.g. "/uploads/receipts/logo.png".
	LogoURL string `json:"logo_url" gorm:"type:varchar(500);not null;default:''"`
	// One address line per newline — printed verbatim.
	CompanyName    string `json:"company_name" gorm:"type:varchar(255);not null;default:''"`
	CompanyAddress string `json:"company_address" gorm:"type:text;not null;default:''"`
	CompanyTaxID   string `json:"company_tax_id" gorm:"type:varchar(50);not null;default:''"`
	CompanyPhone   string `json:"company_phone" gorm:"type:varchar(50);not null;default:''"`
	DocTitle       string `json:"doc_title" gorm:"type:varchar(255);not null;default:''"`
	SellerName     string `json:"seller_name" gorm:"type:varchar(255);not null;default:''"`
	AccountName    string `json:"account_name" gorm:"type:varchar(255);not null;default:''"`
	// The shop's own bank line on รับชำระโดย — identical on every receipt, so both
	// the bank and its ACCOUNT number (the form's "เลขที่") live here.
	BankName      string    `json:"bank_name" gorm:"type:varchar(255);not null;default:''"`
	BankAccountNo string    `json:"bank_account_no" gorm:"type:varchar(100);not null;default:''"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (ReceiptSettings) TableName() string { return "receipt_settings" }

// AdminReceipt is a receipt issued outside this system, typed in afterwards. It
// records only what is on the paper — nothing here feeds bills, credit or stock.
type AdminReceipt struct {
	ID uint `json:"id" gorm:"primaryKey"`
	// เลขที่ as written on the paper. Not generated, not unique.
	Code            string    `json:"code" gorm:"type:varchar(100);not null;default:''"`
	IssuedDate      time.Time `json:"issued_date" gorm:"type:date;not null"`
	Reference       string    `json:"reference" gorm:"type:varchar(255);not null;default:''"`
	CustomerName    string    `json:"customer_name" gorm:"type:varchar(255);not null;default:''"`
	CustomerAddress string    `json:"customer_address" gorm:"type:text;not null;default:''"`
	CustomerTaxID   string    `json:"customer_tax_id" gorm:"type:varchar(50);not null;default:''"`
	// รับชำระโดย — two independent ticks on the form, so both can be set. The bank
	// and account number on that line come from ReceiptSettings; only the date is
	// chosen per receipt.
	PayCash   bool       `json:"pay_cash" gorm:"not null;default:false"`
	PayCheque bool       `json:"pay_cheque" gorm:"not null;default:false"`
	PaidDate  *time.Time `json:"paid_date" gorm:"type:date"`
	// Sum of the item lines, recomputed on every save. The จำนวน on the payment
	// line prints this same figure rather than storing a second copy of it.
	TotalAmount float64            `json:"total_amount" gorm:"type:decimal(16,2);not null;default:0"`
	Items       []AdminReceiptItem `json:"items,omitempty" gorm:"foreignKey:ReceiptID"`
	CreatedBy   *uint              `json:"created_by" gorm:"index"`
	Creator     *User              `json:"creator,omitempty" gorm:"foreignKey:CreatedBy"`
	CreatedAt   time.Time          `json:"created_at"`
	UpdatedAt   time.Time          `json:"updated_at"`
	DeletedAt   gorm.DeletedAt     `json:"-" gorm:"index"`
}

func (AdminReceipt) TableName() string { return "admin_receipts" }

// AdminReceiptItem is one row of the printed รายการ table.
type AdminReceiptItem struct {
	ID          uint    `json:"id" gorm:"primaryKey"`
	ReceiptID   uint    `json:"receipt_id" gorm:"not null;index"`
	SortOrder   int     `json:"sort_order" gorm:"not null;default:0"`
	Description string  `json:"description" gorm:"type:varchar(255);not null;default:''"`
	Quantity    float64 `json:"quantity" gorm:"type:decimal(16,4);not null;default:0"`
	// Printed right after the quantity ("1000 กรัม").
	Unit string `json:"unit" gorm:"type:varchar(50);not null;default:''"`
	// Held to 4 decimals, printed at 2 — the paper's own unit prices carry more
	// precision than the column they are printed in.
	UnitPrice float64 `json:"unit_price" gorm:"type:decimal(16,4);not null;default:0"`
	// The รวม column exactly as written on the paper. Not derived from quantity ×
	// unit price: the two disagree on real receipts, and this records the paper.
	Amount    float64        `json:"amount" gorm:"type:decimal(16,2);not null;default:0"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

func (AdminReceiptItem) TableName() string { return "admin_receipt_items" }
