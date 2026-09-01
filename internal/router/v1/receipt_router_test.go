package v1

import (
	"os"
	"strings"
	"testing"

	"jk-api/config"

	"github.com/gofiber/fiber/v2"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Fiber matches routes in registration order, so "/receipts/settings" has to be
// registered before "/receipts/:id" or every settings read is handled as a lookup
// of a receipt with id "settings". This pins that order.
func TestReceiptSettingsRoutesComeBeforeTheIDRoute(t *testing.T) {
	dsn := os.Getenv("TEST_DB_DSN")
	if dsn == "" {
		dsn = "host=localhost port=5432 user=postgres password=postgres dbname=jk_db sslmode=disable"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{DryRun: true, Logger: logger.Discard})
	if err != nil {
		t.Skipf("no test database available: %v", err)
	}

	app := fiber.New()
	SetupReceiptRoutes(app.Group("/api/v1"), db, &config.Config{})

	var getPaths []string
	for _, r := range app.GetRoutes() {
		if r.Method == fiber.MethodGet && strings.Contains(r.Path, "/receipts") {
			getPaths = append(getPaths, r.Path)
		}
	}

	settingsAt, idAt := -1, -1
	for i, p := range getPaths {
		if strings.HasSuffix(p, "/receipts/settings") && settingsAt < 0 {
			settingsAt = i
		}
		if strings.HasSuffix(p, "/receipts/:id") && idAt < 0 {
			idAt = i
		}
	}
	if settingsAt < 0 || idAt < 0 {
		t.Fatalf("expected both routes, got %v", getPaths)
	}
	if settingsAt > idAt {
		t.Fatalf("/receipts/settings is registered after /receipts/:id and would never match: %v", getPaths)
	}
}
