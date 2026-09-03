package repository

import (
	"regexp"
	"strings"
	"testing"

	"jk-api/internal/entity"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TestSearchSQL pins the ค้นหา filter on the bills list: it matches the bill code,
// the signer on the issued document, AND the customer who opened the bill — the
// name the list actually renders. The OR group must stay parenthesised, otherwise
// a search would leak bills past the metal/status filters it is combined with.
func TestSearchSQL(t *testing.T) {
	db, err := gorm.Open(postgres.Open(listOrderTestDSN()), &gorm.Config{DryRun: true, Logger: logger.Discard})
	if err != nil {
		t.Skipf("no test database available: %v", err)
	}
	r := &billRepository{db: db}

	metal := "gold"
	status := StatusCompleted
	var bills []entity.Quotation
	sql := r.scope(BillFilter{Search: "สมชาย", Metal: &metal, Status: &status}).
		Find(&bills).Statement.SQL.String()

	for _, want := range []string{
		"quotations.code ILIKE",
		"quotations.signer_name ILIKE",
		"EXISTS (",
		"u.id = quotations.created_by",
		"u.deleted_at IS NULL",
		"u.name ILIKE",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("SQL missing %q:\n%s", want, sql)
		}
	}

	// The whole OR group is one AND-ed term — GORM parenthesises it. Without the
	// wrap, `status = ? AND code ILIKE ? OR name ILIKE ?` would return other
	// statuses (and other metals) entirely.
	if !regexp.MustCompile(`AND \(\s*quotations\.code ILIKE`).MatchString(sql) {
		t.Fatalf("search OR group is not parenthesised — it would break the other filters:\n%s", sql)
	}
	// The metal/status filters must survive alongside it.
	for _, want := range []string{"quotations.metal =", "quotations.status ="} {
		if !strings.Contains(sql, want) {
			t.Fatalf("SQL missing %q:\n%s", want, sql)
		}
	}
}
