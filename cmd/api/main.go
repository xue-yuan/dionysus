package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/compress"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/etag"
	"github.com/gofiber/fiber/v3/middleware/limiter"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/xue-yuan/dionysus/internal/config"
	"github.com/xue-yuan/dionysus/internal/database"
	"github.com/xue-yuan/dionysus/internal/handler"
	"github.com/xue-yuan/dionysus/internal/repository"
)

func main() {
	cfg := config.Load()

	db, err := database.Connect(cfg)
	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	defer db.Close()

	// A server whose schema did not apply answers every request with a 500.
	// Refusing to start is the honest failure. Where MIGRATE_ON_START is off,
	// running seed before deploying is what guarantees the schema instead.
	if cfg.MigrateOnStart {
		migrateCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		err := database.Migrate(migrateCtx, db)
		cancel()
		if err != nil {
			log.Fatalf("migrate: %v", err)
		}
	}

	h := handler.New(repository.New(db))

	app := fiber.New(fiber.Config{
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 20 * time.Second,
		IdleTimeout:  60 * time.Second,
		BodyLimit:    256 * 1024,
		// nginx forwards from the same host, so without this every visitor
		// shares 127.0.0.1's rate limit. The header is believed only from
		// loopback; anyone reaching the port directly cannot spoof it.
		ProxyHeader:      "X-Real-IP",
		TrustProxy:       true,
		TrustProxyConfig: fiber.TrustProxyConfig{Loopback: true},
	})
	// Fiber does not recover panics on its own, so without this one nil
	// dereference in a handler takes the whole process down with it.
	app.Use(recover.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: cfg.AllowedOrigins,
		AllowMethods: []string{fiber.MethodGet, fiber.MethodPost, fiber.MethodOptions},
	}))
	app.Use(compress.New())
	// The archive is read-only, so every response is a pure function of the
	// request. ETag turns a repeat visit into a 304 with no body.
	app.Use(etag.New())
	// Matching is the most expensive query in the app and is open to anyone.
	if cfg.RateLimit > 0 {
		app.Use(limiter.New(limiter.Config{
			Max:        cfg.RateLimit,
			Expiration: time.Minute,
		}))
	}

	h.Routes(app)

	go func() {
		log.Printf("listening on :%s (cors: %v)", cfg.Port, cfg.AllowedOrigins)
		if err := app.Listen(":" + cfg.Port); err != nil {
			log.Fatalf("listen: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("shutting down")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()
	if err := app.ShutdownWithContext(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}
