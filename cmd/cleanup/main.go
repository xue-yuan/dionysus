package main

import (
	"log"
	"github.com/xue-yuan/dionysus/internal/config"
	"github.com/xue-yuan/dionysus/internal/database"
)

func main() {
	cfg := config.Load()
	if err := database.Connect(cfg); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	res, err := database.DB.Exec("DELETE FROM recipes WHERE title = 'Antigravity Elixir'")
	if err != nil {
		log.Fatalf("Failed to delete recipe: %v", err)
	}
	rows, _ := res.RowsAffected()
	log.Printf("Successfully deleted %d recipe(s) matching 'Antigravity Elixir'", rows)
}
