package entity

import "time"

// Sell-order lifecycle. Filling is transient: the engine sets it inside the
// claim transaction, so a crash mid-fill leaves a row the boot recovery can
// resolve (bill exists → filled, otherwise → active) instead of a row that is
// silently double-filled on the next tick.
const (
	SellOrderActive    = "active"
	SellOrderFilling   = "filling"
	SellOrderFilled    = "filled"
	SellOrderCancelled = "cancelled"
)

// SellOrder is a customer's standing instruction to sell a fixed weight of metal
// once the shop's real-time buy price reaches TargetPrice — a limit order. The
// engine fills it by creating a normal "รอออกบิล" bill flagged auto_sell.
//
// TargetPrice is compared against bar_buy (the price on the customer's screen,
// after premium and spread), NOT the spot or mid price.
type SellOrder struct {
	ID         uint   `json:"id" gorm:"primaryKey"`
	UserID     uint   `json:"user_id" gorm:"not null;index"`
	User       *User  `json:"user,omitempty" gorm:"foreignKey:UserID"`
	CreatedBy  *uint  `json:"created_by"`
	StoreID    *uint  `json:"store_id"`
	BranchID   *uint  `json:"branch_id"`
	Metal      string `json:"metal" gorm:"type:varchar(20);default:'gold'"`
	GoldTypeID *uint  `json:"gold_type_id"`
	TypeName   string `json:"type_name" gorm:"type:varchar(100);default:''"`
	// Weight is in บาททอง — the same unit the sell screen's stepper uses.
	Weight      float64 `json:"weight" gorm:"type:decimal(10,4);not null"`
	TargetPrice float64 `json:"target_price" gorm:"type:decimal(12,2);not null"`
	Status      string  `json:"status" gorm:"type:varchar(12);default:'active';index"`

	// Pricing policy in force when the order was placed, kept for audit: changing
	// the spread later moves what every live target means, and this is the only
	// record of what the customer was looking at.
	PremiumAtCreate float64 `json:"premium_at_create" gorm:"type:decimal(10,2);default:0"`
	SpreadAtCreate  float64 `json:"spread_at_create"  gorm:"type:decimal(10,2);default:0"`
	PriceAtCreate   float64 `json:"price_at_create"   gorm:"type:decimal(12,2);default:0"`

	// FilledPrice is the price actually captured, which can exceed TargetPrice
	// when the market jumped between ticks (see auto_sell_max_slippage_thb).
	FilledPrice *float64   `json:"filled_price" gorm:"type:decimal(12,2)"`
	FilledAt    *time.Time `json:"filled_at"`
	BillID      *uint      `json:"bill_id"`
	Bill        *Quotation `json:"bill,omitempty" gorm:"foreignKey:BillID"`

	CancelledAt  *time.Time `json:"cancelled_at"`
	CancelReason string     `json:"cancel_reason" gorm:"type:varchar(255);default:''"`
	Note         string     `json:"note" gorm:"type:varchar(255);default:''"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

func (SellOrder) TableName() string { return "sell_orders" }

// EstimatedAmount is what the customer would receive if the order filled exactly
// at its target: gold is quoted per baht-weight, so amount = price × weight
// (mirrors the sell screen — see billCalculate.tsx's `total`).
func (o *SellOrder) EstimatedAmount() float64 {
	return o.TargetPrice * o.Weight
}
