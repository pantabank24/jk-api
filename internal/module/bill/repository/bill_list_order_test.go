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
			name:   "รอตรวจบิล sorts by when staff issued it",
			filter: BillFilter{Status: &pendingReview},
			want:   []string{"iq.id = quotations.issued_quotation_id", "quotations.updated_at) DESC", "quotations.id DESC"},
			reject: []string{"MAX(qi.created_at)"},
		},
		{
			name:   "other tabs keep the original id order",
			filter: BillFilter{Status: &completed},
			want:   []string{"ORDER BY quotations.id DESC"},
			reject: []string{"MAX(qi.created_at)", "issued_quotation_id"},
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
