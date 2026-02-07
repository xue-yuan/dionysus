package main

import (
	"log"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/xue-yuan/dionysus/internal/config"
	"github.com/xue-yuan/dionysus/internal/database"
	"github.com/xue-yuan/dionysus/internal/handler"
	"github.com/xue-yuan/dionysus/internal/repository"
)

func main() {
	cfg := config.Load()

	if err := database.Connect(cfg); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	if err := database.Migrate("../../schema.sql"); err != nil {
		log.Printf("Warning: Migration failed (might be path issue if running from binary): %v", err)
		if err := database.Migrate("schema.sql"); err != nil {
			log.Printf("Warning: Migration retry failed: %v", err)
		}
	}

	repo := repository.New(database.DB)
	handler := handler.New(repo)

	app := fiber.New()
	app.Use(cors.New())

	handler.Routes(app)

	log.Printf("Server starting on port %s", cfg.Port)
	log.Fatal(app.Listen(":" + cfg.Port))
}
