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
	IsActive  bool           `json:"is_active" gorm:"default:true"`
	Users     []User         `json:"users,omitempty" gorm:"foreignKey:BranchID"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}
