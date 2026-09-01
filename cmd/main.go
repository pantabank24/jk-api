package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"jk-api/config"
	"jk-api/internal/router"
	v1 "jk-api/internal/router/v1"
	"jk-api/pkg/database"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

func main() {
	// Load config
	cfg := config.LoadConfig()

	// Connect to database
	db, err := database.NewPostgresDB(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Run migrations
	if err := database.RunMigrations(cfg); err != nil {
		log.Printf("⚠️  Migration warning: %v", err)
	}

	// Create Fiber app
	app := fiber.New(fiberConfig(cfg))

	// Global middleware
	app.Use(recover.New())
	app.Use(logger.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: cfg.CorsOrigins,
		AllowMethods: "GET,POST,PUT,PATCH,DELETE,OPTIONS",
		AllowHeaders: "Origin,Content-Type,Accept,Authorization",
	}))

	// Static files (uploads)
	app.Static("/uploads", "./uploads")

	// Health check
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "ok",
			"message": "Server is running",
		})
	})

	// Start gold price cron service
	cronSvc := v1.NewCronService(db)
	cronSvc.Start()

	// Start the auto-sell engine (fills customers' target-price orders). It also
	// resolves any fill interrupted by the previous shutdown as it starts.
	sellEngine := v1.NewSellOrderEngine(db)
	sellEngine.Start()

	// Setup routes
	router.SetupRoutes(app, db, cfg, cronSvc, sellEngine)

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		addr := fmt.Sprintf(":%s", cfg.AppPort)
		log.Printf("🚀 Server starting on %s", addr)
		if err := app.Listen(addr); err != nil {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	<-quit
	log.Println("🛑 Shutting down server...")
	// Stop matching before the HTTP server: it waits for a tick in flight, so no
	// order is left mid-fill for the boot recovery to clean up.
	sellEngine.Stop()
	if err := app.Shutdown(); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}
	log.Println("✅ Server exited gracefully")
}

func customErrorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
	}
	return c.Status(code).JSON(fiber.Map{
		"success": false,
		"message": err.Error(),
	})
}

// fiberConfig builds the server's configuration.
//
// The app never sees a client directly: nginx on the host proxies to the
// container's published port, so the socket's peer address is the docker
// bridge gateway. Left alone, c.IP() records that gateway for every login,
// every activity log and every PDPA consent — the same 172.x for everyone,
// which is worthless as evidence.
//
// The three settings only work together:
//
//	ProxyHeader             — read the client from X-Forwarded-For instead
//	EnableTrustedProxyCheck — but only when the peer is a proxy we listed,
//	                          otherwise anyone could post their own header
//	                          and sign a consent under a forged IP
//	EnableIPValidation      — X-Forwarded-For is a LIST ("client, proxy1").
//	                          Without this, Fiber stores the raw header
//	                          verbatim; with it, the first valid IP — the
//	                          client — is what comes back.
//
// If the header is missing or the peer is untrusted, c.IP() falls back to
// the socket address, so this can only ever improve on what was stored.
func fiberConfig(cfg *config.Config) fiber.Config {
	return fiber.Config{
		AppName:                 cfg.AppName,
		ErrorHandler:            customErrorHandler,
		BodyLimit:               10 * 1024 * 1024, // 10MB for file uploads
		ProxyHeader:             fiber.HeaderXForwardedFor,
		EnableTrustedProxyCheck: true,
		TrustedProxies:          cfg.TrustedProxyList(),
		EnableIPValidation:      true,
	}
}
