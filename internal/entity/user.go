package entity

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	StoreID   *uint          `json:"store_id" gorm:"index"`
	Store     *Store         `json:"store,omitempty" gorm:"foreignKey:StoreID"`
	BranchID  *uint          `json:"branch_id" gorm:"index"`
	Branch    *Branch        `json:"branch,omitempty" gorm:"foreignKey:BranchID"`
	RoleID    *uint          `json:"role_id" gorm:"index"`
	Role      *Role          `json:"role,omitempty" gorm:"foreignKey:RoleID"`
	StoreName string         `json:"store_name" gorm:"type:varchar(255);default:''"`
	Name      string         `json:"name" gorm:"type:varchar(255);not null"`
	Email     string         `json:"email" gorm:"type:varchar(255);uniqueIndex;not null"`
	Password  string         `json:"-" gorm:"type:varchar(255);not null"`
	Phone     string         `json:"phone" gorm:"type:varchar(20);default:''"`
	Address   string         `json:"address" gorm:"type:text;default:''"`
	TaxID     string         `json:"tax_id" gorm:"type:varchar(50);default:''"`
	Avatar    string         `json:"avatar" gorm:"type:varchar(500);default:''"`
	IsActive  bool           `json:"is_active" gorm:"default:true"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}
