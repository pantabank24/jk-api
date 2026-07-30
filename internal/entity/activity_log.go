package entity

import (
	"encoding/json"
	"time"
)

type ActivityLog struct {
	ID     uint  `gorm:"primarykey"        json:"id"`
	UserID *uint `gorm:"index"             json:"user_id"`
	User   *User `gorm:"foreignKey:UserID" json:"user,omitempty"`
	// TargetUserID is WHOM the action was about, as opposed to UserID (who did
	// it). Set by controllers via middleware.SetActivityTarget so that staff
	// actions on a customer's bill — ออกบิล, อนุมัติ, ยกเลิก, ลบ — appear on that
	// customer's timeline even though staff performed them. Nil for actions that
	// concern no particular customer.
	TargetUserID *uint  `gorm:"index"                   json:"target_user_id"`
	TargetUser   *User  `gorm:"foreignKey:TargetUserID" json:"target_user,omitempty"`
	Method       string `                               json:"method"`
	Path         string `                               json:"path"`
	// Description is an optional human-readable summary of the business action
	// (e.g. "อนุมัติใบเสนอราคา P2607001") set by the controller via
	// middleware.SetActivityDescription. Empty when the route doesn't set one —
	// the frontend falls back to showing the raw method+path in that case.
	Description string `json:"description"`
	// RefCode is the document the action touched (bill / quotation code), so a
	// row is traceable to a specific piece of paper without parsing Description.
	RefCode string `json:"ref_code"`
	// Detail is a structured snapshot of the action taken at the moment it
	// happened — for a sell, the per-item price, weight and total the customer
	// actually clicked. Written once and never updated, so it stays valid
	// evidence even after the bill itself is edited. See middleware.SetActivityDetail.
	Detail     json.RawMessage `gorm:"type:jsonb" json:"detail,omitempty"`
	StatusCode int             `                  json:"status_code"`
	IP         string          `                  json:"ip"`
	UserAgent  string          `                  json:"user_agent"`
	DurationMs int64           `                  json:"duration_ms"`
	CreatedAt  time.Time       `                  json:"created_at"`
}
