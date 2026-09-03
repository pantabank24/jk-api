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

// TestListOrderSQL pins how each bills tab is sorted. Both working tabs must sort
// by the event being waited on rather than by the bill's id — a รอออกบิล bill is
// reused as the customer keeps selling, so its id is only when it was first opened.
func TestListOrderSQL(t *testing.T) {
	db, err := gorm.Open(postgres.Open(listOrderTestDSN()), &gorm.Config{DryRun: true, Logger: logger.Discard})
	if err != nil {
		t.Skipf("no test database available: %v", err)
	}
	r := &billRepository{db: db}

	pendingIssue := StatusPendingIssue
	pendingReview := StatusPendingReview
	completed := StatusCompleted
	cleared := StatusCleared

	cases := []struct {
		name   string
		filter BillFilter
		want   []string
		reject []string
	}{
		{
			name:   "รอออกบิล sorts by the customer's newest item",
			filter: BillFilter{Status: &pendingIssue},
			want:   []string{"MAX(qi.created_at)", "qi.deleted_at IS NULL", "quotations.created_at) DESC", "quotations.id DESC"},
		},
		{
			name:   "รอตรวจบิล sorts by when it was put into that status",
			filter: BillFilter{Status: &pendingReview},
			want:   []string{"quotations.status_changed_at DESC NULLS LAST", "quotations.id DESC"},
			reject: []string{"MAX(qi.created_at)"},
		},
		{
			// id is when the bill was first OPENED. A bill opened in July but cleared
			// today must not sink to the bottom of เคลียร์แล้ว, and updated_at is no
			// good either: it moves when a closed bill's note is edited.
			name:   "สำเร็จ sorts by when it was completed, not by id or updated_at",
			filter: BillFilter{Status: &completed},
			want:   []string{"quotations.status_changed_at DESC NULLS LAST"},
			reject: []string{"MAX(qi.created_at)", "ORDER BY quotations.id DESC", "quotations.updated_at"},
		},
		{
			name:   "เคลียร์แล้ว sorts by when it was cleared",
			filter: BillFilter{Status: &cleared},
			want:   []string{"quotations.status_changed_at DESC NULLS LAST"},
			reject: []string{"ORDER BY quotations.id DESC", "quotations.updated_at"},
		},
		{
			name:   "no status filter keeps the original id order",
			filter: BillFilter{},
			want:   []string{"ORDER BY quotations.id DESC"},
			reject: []string{"MAX(qi.created_at)"},
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
