package entity

import "time"

type CreditTransaction struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	MemberID    uint      `json:"member_id" gorm:"not null;index"`
	Member      *Member   `json:"member,omitempty" gorm:"foreignKey:MemberID"`
	StoreID     uint      `json:"store_id" gorm:"not null;index"`
	BranchID    uint      `json:"branch_id" gorm:"not null;index"`
	Action      int       `json:"action" gorm:"not null;default:0"`
	Amount      float64   `json:"amount" gorm:"type:decimal(12,2);not null;default:0"`
	Balance     float64   `json:"balance" gorm:"type:decimal(12,2);not null;default:0"`
	Description string    `json:"description" gorm:"type:text;default:''"`
	CreatedBy   *uint     `json:"created_by"`
	Creator     *User     `json:"creator,omitempty" gorm:"foreignKey:CreatedBy"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
