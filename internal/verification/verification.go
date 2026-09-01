// Package verification derives a customer's document-verification badge from the
// review state of their high-priority documents. It is computed rather than stored
// so toggling a document type's is_high_priority flag takes effect immediately,
// with no back-fill.
package verification

import (
	"jk-api/internal/entity"

	"gorm.io/gorm"
)

// Badge states, mirrored by the frontend's VerifyBadge.
const (
	StatusNone     = "none"     // no high-priority document on file (grey)
	StatusPending  = "pending"  // at least one copy waiting for staff review (yellow)
	StatusVerified = "verified" // reviewed and accepted (blue)
	StatusRejected = "rejected" // reviewed and turned down, nothing newer (red)
)

// StatusFor computes the badge for many users in one query. Users with no
// high-priority documents are returned as StatusNone, so the map always has an
// entry for every id asked for.
func StatusFor(db *gorm.DB, userIDs []uint) map[uint]string {
	out := make(map[uint]string, len(userIDs))
	for _, id := range userIDs {
		out[id] = StatusNone
	}
	if len(userIDs) == 0 {
		return out
	}

	type row struct {
		UserID uint
		Status string
	}
	var rows []row
	err := db.Model(&entity.CustomerDocument{}).
		Select("customer_documents.user_id AS user_id, customer_documents.approval_status AS status").
		Joins("JOIN document_types ON document_types.id = customer_documents.document_type_id").
		Where("document_types.is_high_priority = ? AND customer_documents.user_id IN ?", true, userIDs).
		Group("customer_documents.user_id, customer_documents.approval_status").
		Scan(&rows).Error
	if err != nil {
		return out
	}

	// Precedence: anything still waiting outranks a past decision, and an accepted
	// copy outranks a rejected one — a customer who fixed a bad scan reads as
	// verified rather than staying flagged.
	rank := map[string]int{StatusNone: 0, StatusRejected: 1, StatusVerified: 2, StatusPending: 3}
	for _, r := range rows {
		status := StatusNone
		switch r.Status {
		case entity.ApprovalPending:
			status = StatusPending
		case entity.ApprovalApproved:
			status = StatusVerified
		case entity.ApprovalRejected:
			status = StatusRejected
		}
		if rank[status] > rank[out[r.UserID]] {
			out[r.UserID] = status
		}
	}
	return out
}

// StatusOf is the single-user form of StatusFor.
func StatusOf(db *gorm.DB, userID uint) string {
	return StatusFor(db, []uint{userID})[userID]
}

// Apply stamps the computed badge onto a slice of users in one query.
func Apply(db *gorm.DB, users []entity.User) {
	ids := make([]uint, 0, len(users))
	for _, u := range users {
		ids = append(ids, u.ID)
	}
	statuses := StatusFor(db, ids)
	for i := range users {
		users[i].VerificationStatus = statuses[users[i].ID]
	}
}

// ApplyToUsers stamps the badge onto users reached by pointer — the preloaded
// Creator hanging off a list of bills, say. Still a single query for the whole
// page, and nil entries are skipped.
func ApplyToUsers(db *gorm.DB, users []*entity.User) {
	ids := make([]uint, 0, len(users))
	for _, u := range users {
		if u != nil {
			ids = append(ids, u.ID)
		}
	}
	if len(ids) == 0 {
		return
	}
	statuses := StatusFor(db, ids)
	for _, u := range users {
		if u != nil {
			u.VerificationStatus = statuses[u.ID]
		}
	}
}
