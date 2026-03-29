package entity

import "time"

type Role struct {
	ID          uint             `json:"id" gorm:"primaryKey"`
	Name        string           `json:"name" gorm:"type:varchar(100);uniqueIndex;not null"`
	DisplayName string           `json:"display_name" gorm:"type:varchar(255);not null"`
	Description string           `json:"description" gorm:"type:text;default:''"`
	IsSystem    bool             `json:"is_system" gorm:"default:false"`
	Permissions []RolePermission `json:"permissions,omitempty" gorm:"foreignKey:RoleID"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
}
