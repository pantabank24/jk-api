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
	Logo      string         `json:"logo" gorm:"type:varchar(500);default:''"`
	IsActive  bool           `json:"is_active" gorm:"default:true"`
	Branches  []Branch       `json:"branches,omitempty" gorm:"foreignKey:StoreID"`
	Users     []User         `json:"users,omitempty" gorm:"foreignKey:StoreID"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}
