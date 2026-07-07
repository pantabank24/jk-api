package entity

import (
	"time"

	"gorm.io/gorm"
)

type Branch struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	StoreID   uint           `json:"store_id" gorm:"not null;index"`
	Store     *Store         `json:"store,omitempty" gorm:"foreignKey:StoreID"`
	Code      string         `json:"code" gorm:"type:varchar(20);uniqueIndex;not null"`
	Name      string         `json:"name" gorm:"type:varchar(255);not null"`
	Address   string         `json:"address" gorm:"type:text;default:''"`
	Phone     string         `json:"phone" gorm:"type:varchar(20);default:''"`
	// Receipt-header fields — each branch prints its own header. HeaderName is the
	// shop name shown as the big title (the branch's own Name is the "สาขา" line);
	// Website is the subtitle under the name. IsMain marks the store's default
	// branch (one per store), used when no branch is explicitly chosen.
	HeaderName string        `json:"header_name" gorm:"type:varchar(255);default:''"`
	TaxID      string        `json:"tax_id" gorm:"type:varchar(50);default:''"`
	TaxName    string        `json:"tax_name" gorm:"type:varchar(255);default:''"`
	Website    string        `json:"website" gorm:"type:varchar(255);default:''"`
	Logo       string        `json:"logo" gorm:"type:varchar(500);default:''"`
	IsMain     bool          `json:"is_main" gorm:"default:false;index"`
	IsActive  bool           `json:"is_active" gorm:"default:true"`
	Users     []User         `json:"users,omitempty" gorm:"foreignKey:BranchID"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}
