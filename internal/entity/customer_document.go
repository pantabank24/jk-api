package entity

import "time"

// CustomerDocument is a file uploaded for a customer (User with role
// "customer") — e.g. ID copies, company papers. Stored under
// ./uploads/customers/{user_id}/ and served via the /uploads static route.
type CustomerDocument struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	UserID     uint      `json:"user_id" gorm:"index;not null"`
	FileName   string    `json:"file_name" gorm:"type:varchar(255);default:''"` // original name for display
	FilePath   string    `json:"file_path" gorm:"type:varchar(500);default:''"` // relative URL path (/uploads/...)
	FileExt    string    `json:"file_ext" gorm:"type:varchar(10);default:''"`
	FileSize   int64     `json:"file_size" gorm:"default:0"`
	UploadedBy *uint     `json:"uploaded_by"`
	CreatedAt  time.Time `json:"created_at"`
}
