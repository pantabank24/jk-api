package entity

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID        uint    `json:"id" gorm:"primaryKey"`
	StoreID   *uint   `json:"store_id" gorm:"index"`
	Store     *Store  `json:"store,omitempty" gorm:"foreignKey:StoreID"`
	BranchID  *uint   `json:"branch_id" gorm:"index"`
	Branch    *Branch `json:"branch,omitempty" gorm:"foreignKey:BranchID"`
	RoleID    *uint   `json:"role_id" gorm:"index"`
	Role      *Role   `json:"role,omitempty" gorm:"foreignKey:RoleID"`
	StoreName string  `json:"store_name" gorm:"type:text;default:''"`
	Name      string  `json:"name" gorm:"type:text;not null"`
	Email     string  `json:"email" gorm:"type:text;uniqueIndex;not null"`
	Password  string  `json:"-" gorm:"type:varchar(255);not null"`
	Phone     string  `json:"phone" gorm:"type:text;default:''"`
	Address   string  `json:"address" gorm:"type:text;default:''"`
	TaxID     string  `json:"tax_id" gorm:"type:text;default:''"`
	// Payout account (customers). BankID is nullable — a customer may have no bank on file.
	BankID          *uint          `json:"bank_id" gorm:"index"`
	Bank            *Bank          `json:"bank,omitempty" gorm:"foreignKey:BankID"`
	BankAccountNo   string         `json:"bank_account_no" gorm:"type:text;default:''"`
	BankAccountName string         `json:"bank_account_name" gorm:"type:text;default:''"`
	Avatar          string         `json:"avatar" gorm:"type:varchar(500);default:''"`
	IsActive        bool           `json:"is_active" gorm:"default:true"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `json:"-" gorm:"index"`
}
