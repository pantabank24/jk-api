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

// containsInt reports whether v holds the given number, whatever integer type
// the driver bound it as.
func containsInt(vars []any, want int64) bool {
	for _, v := range vars {
		switch n := v.(type) {
		case int:
			if int64(n) == want {
				return true
			}
		case int64:
			if n == want {
				return true
			}
		}
	}
	return false
}

func scopeTestDSN() string {
	if dsn := os.Getenv("TEST_DB_DSN"); dsn != "" {
		return dsn
	}
	return "host=localhost port=5432 user=postgres password=postgres dbname=jk_db sslmode=disable"
}

// TestFindAllScope pins the two halves of how customers are scoped: they ARE
// filtered by store (one store must not see another's customers) and are NEVER
// filtered by branch. Branch belongs to staff only — role `employee` is the sole
// role requiring one — so a branch filter would match no customer at all and
// leave every branch's customer list empty.
func TestFindAllScope(t *testing.T) {
	db, err := gorm.Open(postgres.Open(scopeTestDSN()), &gorm.Config{DryRun: true, Logger: logger.Discard})
	if err != nil {
		t.Skipf("no test database available: %v", err)
	}
	r := &customerRepository{db: db}

	var users []entity.User
	build := func(storeID *uint) string {
		q := r.db.Model(&entity.User{}).Where("role_id = ?", 1)
		if storeID != nil {
			q = q.Where("store_id = ?", *storeID)
		}
		return q.Find(&users).Statement.SQL.String()
	}

	store := uint(7)
	scoped := build(&store)
	if !strings.Contains(scoped, "store_id = ") {
		t.Fatalf("a store-bound caller must be filtered by store_id:\n%s", scoped)
	}
	if strings.Contains(scoped, "branch_id") {
		t.Fatalf("customers must never be filtered by branch_id — it is always NULL "+
			"for them, so the list would come back empty:\n%s", scoped)
	}

	unscoped := build(nil)
	if strings.Contains(unscoped, "store_id") || strings.Contains(unscoped, "branch_id") {
		t.Fatalf("master's listing must not be scoped:\n%s", unscoped)
	}
}

// TestFindSoleStoreIDQuery pins that resolving "the shop's only store" reads at
// most two rows — it answers "is there exactly one?", not "how many are there".
func TestFindSoleStoreIDQuery(t *testing.T) {
	db, err := gorm.Open(postgres.Open(scopeTestDSN()), &gorm.Config{DryRun: true, Logger: logger.Discard})
	if err != nil {
		t.Skipf("no test database available: %v", err)
	}
	r := &customerRepository{db: db}
	var ids []uint
	stmt := r.db.Model(&entity.Store{}).Order("id").Limit(2).Find(&ids).Statement
	sql := stmt.SQL.String()
	// LIMIT is bound as a parameter, so the cap lives in Vars, not the SQL text.
	if !strings.Contains(sql, "LIMIT") || !containsInt(stmt.Vars, 2) {
		t.Fatalf("expected the sole-store probe to stop at 2 rows:\n%s %v", sql, stmt.Vars)
	}
	if !strings.Contains(sql, `"stores"."deleted_at" IS NULL`) {
		t.Fatalf("a soft-deleted store must not count as the shop's only store:\n%s", sql)
	}
}

// TestNameOrderUsesThaiCollation pins that the customer list is ordered by name
// with the Thai ICU collation. Ordinary byte order gets the consonants right but
// throws every leading-vowel name (เ แ โ ใ ไ) to the bottom of the list, so
// "เกียรติ" would sort after "อนันต์" instead of under ก.
func TestNameOrderUsesThaiCollation(t *testing.T) {
	db, err := gorm.Open(postgres.Open(scopeTestDSN()), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Skipf("no test database available: %v", err)
	}
	r := &customerRepository{db: db}

	order := r.nameOrder()
	if !strings.Contains(order, thaiCollation) {
		t.Fatalf("expected the Thai ICU collation in the order clause, got %q", order)
	}

	// The clause has to survive contact with the real planner — a collation that
	// does not exist only fails when the query runs, not when it is built.
	var names []string
	if err := db.Model(&entity.User{}).Order(order).Pluck("name", &names).Error; err != nil {
		t.Fatalf("ordering by %q is not a valid query: %v", order, err)
	}

	// And it must actually put a leading-vowel name under its consonant.
	var got []string
	if err := db.Raw(`SELECT * FROM (VALUES ('อนันต์'),('เกียรติ'),('กมล')) AS t(name)
		ORDER BY name COLLATE "`+thaiCollation+`"`).Pluck("name", &got).Error; err != nil {
		t.Fatalf("collation probe failed: %v", err)
	}
	want := []string{"กมล", "เกียรติ", "อนันต์"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("Thai order wrong: got %v, want %v", got, want)
	}
}

// TestNameOrderFallsBack pins the safety net: on a database that cannot answer
// the collation probe, listing customers must still work — unsorted-by-Thai is
// acceptable, a broken customer page is not.
func TestNameOrderFallsBack(t *testing.T) {
	db, err := gorm.Open(postgres.Open(scopeTestDSN()), &gorm.Config{DryRun: true, Logger: logger.Discard})
	if err != nil {
		t.Skipf("no test database available: %v", err)
	}
	// DryRun never executes the probe, so it stands in for "collation not found".
	r := &customerRepository{db: db}
	order := r.nameOrder()
	if strings.Contains(order, "COLLATE") {
		t.Fatalf("expected the plain fallback order, got %q", order)
	}
	if !strings.Contains(order, "users.name") {
		t.Fatalf("the fallback must still order by name, got %q", order)
	}
}
