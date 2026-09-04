package entity

import (
	"time"

	"gorm.io/gorm"
)

// Intake statuses. A row starts at IntakeStatusOpen and leaves it exactly once —
// either consumed by a quotation, or cancelled by hand.
const (
	IntakeStatusOpen      = 0 // รอออกใบเสนอราคา
	IntakeStatusUsed      = 1 // ออกใบเสนอราคาแล้ว
	IntakeStatusCancelled = 2 // ยกเลิก
)

// Image categories on an intake. They match quotation_images' own type strings so
// the rows can be copied across verbatim when the quotation is issued.
const (
	IntakeImageBeforeMelt = "before_melt"
	IntakeImageIDCard     = "id_card"
)

// QuotationIntake is ใบเปิดงาน — the counter half of issuing a quotation, done
// before the metal is melted: photos of the goods as received, the customer's ID
// card, and their name/phone. It carries no items and no money; the quotation
// issued from it later is where all of that appears (see migration 000103).
type QuotationIntake struct {
	ID        uint    `json:"id" gorm:"primaryKey"`
	StoreID   *uint   `json:"store_id" gorm:"index"`
	Store     *Store  `json:"store,omitempty" gorm:"foreignKey:StoreID"`
	BranchID  *uint   `json:"branch_id" gorm:"index"`
	Branch    *Branch `json:"branch,omitempty" gorm:"foreignKey:BranchID"`
	CreatedBy *uint   `json:"created_by" gorm:"index"`
	Creator   *User   `json:"creator,omitempty" gorm:"foreignKey:CreatedBy"`
	// CustomerID links a registered customer when one was picked. Optional — a
	// walk-in is recorded by the typed name/phone alone.
	CustomerID    *uint  `json:"customer_id" gorm:"index"`
	Customer      *User  `json:"customer,omitempty" gorm:"foreignKey:CustomerID"`
	CustomerName  string `json:"customer_name"  gorm:"type:varchar(255);default:''"`
	CustomerPhone string `json:"customer_phone" gorm:"type:varchar(30);default:''"`
	Note          string `json:"note"           gorm:"type:text;default:''"`
	Status        int    `json:"status"         gorm:"default:0;index"`
	// QuotationID / UsedAt are stamped when a quotation consumes this intake.
	QuotationID *uint                  `json:"quotation_id" gorm:"index"`
	Quotation   *Quotation             `json:"quotation,omitempty" gorm:"foreignKey:QuotationID"`
	UsedAt      *time.Time             `json:"used_at,omitempty"`
	Images      []QuotationIntakeImage `json:"images,omitempty" gorm:"foreignKey:IntakeID"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	DeletedAt   gorm.DeletedAt         `json:"-" gorm:"index"`
}

func (QuotationIntake) TableName() string { return "quotation_intakes" }

type QuotationIntakeImage struct {
	ID       uint   `json:"id" gorm:"primaryKey"`
	IntakeID uint   `json:"intake_id" gorm:"not null;index"`
	ImageURL string `json:"image_url" gorm:"type:varchar(500);not null"`
	// before_melt | id_card — same vocabulary as QuotationImage.Type.
	Type      string         `json:"type" gorm:"type:varchar(50);default:''"`
	CreatedAt time.Time      `json:"created_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

func (QuotationIntakeImage) TableName() string { return "quotation_intake_images" }
