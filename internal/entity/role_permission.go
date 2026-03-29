package entity

type RolePermission struct {
	ID           uint        `json:"id" gorm:"primaryKey"`
	RoleID       uint        `json:"role_id" gorm:"not null;uniqueIndex:idx_role_perm"`
	PermissionID uint        `json:"permission_id" gorm:"not null;uniqueIndex:idx_role_perm"`
	Permission   *Permission `json:"permission,omitempty" gorm:"foreignKey:PermissionID"`
}
