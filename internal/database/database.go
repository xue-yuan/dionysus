package database

import (
	"fmt"
	"log"
	"os"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/xue-yuan/dionysus/internal/config"
)

var DB *sqlx.DB

func Connect(cfg *config.Config) error {
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, cfg.DBName)

	log.Printf("Connecting to DB DSN: postgres://%s:***@%s:%s/%s?sslmode=disable", cfg.DBUser, cfg.DBHost, cfg.DBPort, cfg.DBName)

	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		return err
	}

	if err := db.Ping(); err != nil {
		return err
	}

	DB = db
	log.Println("Connected to PostgreSQL successfully")
	return nil
}

func Migrate(schemaPath string) error {
	content, err := os.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("failed to read schema file: %w", err)
	}

	_, err = DB.Exec(string(content))
	if err != nil {
		return fmt.Errorf("failed to execute migration: %w", err)
	}

	log.Println("Database migration completed successfully")
	return nil
}
