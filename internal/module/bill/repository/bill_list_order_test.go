package repository

import (
	"os"
	"strings"
	"testing"

	"jk-api/internal/entity"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func listOrderTestDSN() string {
	if dsn := os.Getenv("TEST_DB_DSN"); dsn != "" {
		return dsn
	}
	return "host=localhost port=5432 user=postgres password=postgres dbname=jk_db sslmode=disable"
}

// TestListOrderSQL pins how each bills tab is sorted. Every tab must sort by the
// event being waited on rather than by the bill's id — a รอออกบิล bill is reused as
// the customer keeps selling, so its id is only when it was first opened, and a
// bill opened in July but cleared today must not sink to the bottom of เคลียร์แล้ว.
func TestListOrderSQL(t *testing.T) {
	db, err := gorm.Open(postgres.Open(listOrderTestDSN()), &gorm.Config{DryRun: true, Logger: logger.Discard})
	if err != nil {
		t.Skipf("no test database available: %v", err)
	}
	r := &billRepository{db: db}

	pendingIssue := StatusPendingIssue
	pendingReview := StatusPendingReview
	completed := StatusCompleted
	cancelled := StatusCancelled
	cleared := StatusCleared

	const (
		newestItem      = "MAX(qi.created_at)"
		issuedAt        = "iq.id = quotations.issued_quotation_id"
		statusChangedAt = "COALESCE(quotations.status_changed_at, quotations.created_at) DESC"
	)

	cases := []struct {
		name   string
		filter BillFilter
		want   []string
		reject []string
	}{
		{
			name:   "รอออกบิล sorts by the customer's newest item",
			filter: BillFilter{Status: &pendingIssue},
			want:   []string{newestItem, "qi.deleted_at IS NULL", "quotations.created_at) DESC", "quotations.id DESC"},
			reject: []string{issuedAt},
		},
		{
			// รอตรวจบิล/สำเร็จ/เคลียร์แล้ว are all "after the bill was issued", so they
			// share one key and a bill holds its place as it moves between them.
			name:   "รอตรวจบิล sorts by when the bill was issued",
			filter: BillFilter{Status: &pendingReview},
			want:   []string{issuedAt, "quotations.id DESC"},
			reject: []string{newestItem, "ORDER BY quotations.id DESC"},
		},
		{
			name:   "สำเร็จ sorts by when the bill was issued, not by id or updated_at",
			filter: BillFilter{Status: &completed},
			want:   []string{issuedAt},
			reject: []string{newestItem, "ORDER BY quotations.id DESC", "quotations.updated_at"},
		},
		{
			name:   "เคลียร์แล้ว sorts by when the bill was issued",
			filter: BillFilter{Status: &cleared},
			want:   []string{issuedAt},
			reject: []string{newestItem, "ORDER BY quotations.id DESC", "quotations.updated_at"},
		},
		{
			// A cancelled bill has no issuance to date it by.
			name:   "ยกเลิก sorts by when it was cancelled",
			filter: BillFilter{Status: &cancelled},
			want:   []string{statusChangedAt, "quotations.id DESC"},
			reject: []string{newestItem, issuedAt, "ORDER BY quotations.id DESC", "quotations.updated_at"},
		},
		{
			// ทั้งหมด mixes statuses, so it dates every row by that row's own rule.
			name:   "no status filter dates each row by its own status",
			filter: BillFilter{},
			want:   []string{"CASE quotations.status", newestItem, issuedAt, "quotations.status_changed_at", "quotations.id DESC"},
			reject: []string{"ORDER BY quotations.id DESC"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var bills []entity.Quotation
			sql := r.scope(tc.filter).Order(r.listOrder(tc.filter)).Find(&bills).Statement.SQL.String()
			for _, want := range tc.want {
				if !strings.Contains(sql, want) {
					t.Fatalf("SQL missing %q:\n%s", want, sql)
				}
			}
			for _, bad := range tc.reject {
				if strings.Contains(sql, bad) {
					t.Fatalf("SQL should not contain %q:\n%s", bad, sql)
				}
			}
		})
	}
}
