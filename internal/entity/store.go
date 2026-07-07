package entity

import (
	"time"

	"gorm.io/gorm"
)

type Store struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	Code      string         `json:"code" gorm:"type:varchar(20);uniqueIndex;not null"`
	Name      string         `json:"name" gorm:"type:varchar(255);not null"`
	Address   string         `json:"address" gorm:"type:text;default:''"`
	Phone     string         `json:"phone" gorm:"type:varchar(20);default:''"`
	TaxID     string         `json:"tax_id" gorm:"type:varchar(50);default:''"`
	TaxName   string         `json:"tax_name" gorm:"type:varchar(255);default:''"`
	Website   string         `json:"website" gorm:"type:varchar(255);default:''"`
	Logo      string         `json:"logo" gorm:"type:varchar(500);default:''"`
	// IsMain marks the system's default store (exactly one), used when no store
	// is explicitly chosen — e.g. the master's receipt-header picker.
	IsMain    bool           `json:"is_main" gorm:"default:false;index"`
	IsActive  bool           `json:"is_active" gorm:"default:true"`
	Branches  []Branch       `json:"branches,omitempty" gorm:"foreignKey:StoreID"`
	Users     []User         `json:"users,omitempty" gorm:"foreignKey:StoreID"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}
