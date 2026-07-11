package main

import (
	"log"

	"github.com/xue-yuan/dionysus/internal/config"
	"github.com/xue-yuan/dionysus/internal/database"
)

func main() {
	cfg := config.Load()
	if err := database.Connect(cfg); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}

	var count int
	err := database.DB.Get(&count, `
		SELECT COUNT(distinct recipe_id) 
		FROM recipe_tags 
		JOIN tags ON recipe_tags.tag_id = tags.id 
		WHERE tags.name = 'Classic'
	`)
	if err != nil {
		log.Fatalf("Query failed: %v", err)
	}

	log.Printf("Successfully found %d recipes associated with the 'Classic' tag in the database!", count)
}
