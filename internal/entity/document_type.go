package entity

import "time"

// DocumentType labels a customer document (บัตรประชาชน, เล่มบัญชี, ...). Managed by
// the shop (ลูกค้า → ประเภทเอกสาร) rather than hard-coded, so the list can change
// without a deploy.
type DocumentType struct {
	ID        uint   `json:"id"         gorm:"primaryKey"`
	Name      string `json:"name"       gorm:"type:varchar(100);not null"`
	Code      string `json:"code"       gorm:"type:varchar(50);default:''"` // e.g. id_card, bank_book
	SortOrder int    `json:"sort_order" gorm:"default:0"`
	// High-priority types are identity papers (บัตรประชาชน, เล่มบัญชี). A customer
	// can replace one but never delete it, and each new copy waits for staff review
	// before the customer counts as verified.
	IsHighPriority bool      `json:"is_high_priority" gorm:"default:false"`
	IsActive       bool      `json:"is_active"  gorm:"default:true"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
