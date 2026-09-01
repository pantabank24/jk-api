package usecase

import (
	"testing"

	"jk-api/internal/entity"
)

func newEmptyReceipt() *entity.AdminReceipt { return &entity.AdminReceipt{} }

func newReceipt(t *testing.T, u *receiptUsecase, req *ReceiptRequest) *entity.AdminReceipt {
	t.Helper()
	r := newEmptyReceipt()
	if _, err := u.apply(r, req); err != nil {
		t.Fatalf("apply: %v", err)
	}
	return r
}
