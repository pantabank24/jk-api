package entity

import "time"

// CustomerDocument is a file uploaded for a customer (User with role
// "customer") — e.g. ID copies, company papers. Stored under
// ./uploads/customers/{user_id}/ and served via the /uploads static route.
type CustomerDocument struct {
	ID       uint   `json:"id" gorm:"primaryKey"`
	UserID   uint   `json:"user_id" gorm:"index;not null"`
	FileName string `json:"file_name" gorm:"type:varchar(255);default:''"` // original name for display
	FilePath string `json:"file_path" gorm:"type:varchar(500);default:''"` // relative URL path (/uploads/...)
	FileExt  string `json:"file_ext" gorm:"type:varchar(10);default:''"`
	FileSize int64  `json:"file_size" gorm:"default:0"`
	// Nullable: documents uploaded before types existed stay untyped, and a type
	// deleted from the master list nulls out here (FK ON DELETE SET NULL) rather
	// than taking the document with it.
	DocumentTypeID *uint         `json:"document_type_id"`
	DocumentType   *DocumentType `json:"document_type,omitempty" gorm:"foreignKey:DocumentTypeID"`
	UploadedBy     *uint         `json:"uploaded_by"`
	// Review state. Only high-priority documents are ever written as
	// ApprovalPending; everything else is born ApprovalApproved because there is
	// nothing to check.
	ApprovalStatus string     `json:"approval_status" gorm:"type:varchar(20);default:'approved'"`
	ApprovedBy     *uint      `json:"approved_by"`
	ApprovedAt     *time.Time `json:"approved_at"`
	RejectReason   string     `json:"reject_reason" gorm:"type:varchar(500);default:''"`
	CreatedAt      time.Time  `json:"created_at"`
}

// Review states for CustomerDocument.ApprovalStatus.
const (
	ApprovalPending  = "pending"
	ApprovalApproved = "approved"
	ApprovalRejected = "rejected"
)
