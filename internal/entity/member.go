package entity

import (
	"time"

	"gorm.io/gorm"
)

type Member struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	UserID    *uint          `json:"user_id,omitempty" gorm:"uniqueIndex;index"`
	User      *User          `json:"user,omitempty" gorm:"foreignKey:UserID"`
	StoreID   uint           `json:"store_id" gorm:"not null;index"`
	Store     *Store         `json:"store,omitempty" gorm:"foreignKey:StoreID"`
	BranchID  uint           `json:"branch_id" gorm:"not null;index"`
	Branch    *Branch        `json:"branch,omitempty" gorm:"foreignKey:BranchID"`
	Code      string         `json:"code" gorm:"type:varchar(20);uniqueIndex;not null"`
	Image     string         `json:"image" gorm:"type:varchar(500);default:''"`
	Fname     string         `json:"fname" gorm:"type:varchar(255);not null"`
	Lname     string         `json:"lname" gorm:"type:varchar(255);not null"`
	Phone     string         `json:"phone" gorm:"type:varchar(20);default:''"`
	Credits   float64        `json:"credits" gorm:"type:decimal(12,2);default:0"`
	Status    int            `json:"status" gorm:"default:0"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}
