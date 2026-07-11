package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"

	"github.com/xue-yuan/dionysus/internal/config"
	"github.com/xue-yuan/dionysus/internal/database"
)

type SeedData struct {
	Recipes []struct {
		Title string `json:"title"`
	} `json:"recipes"`
}

func main() {
	cfg := config.Load()
	if err := database.Connect(cfg); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	jsonFile, err := os.Open("cmd/seed/seed_data.json")
	if err != nil {
		log.Fatalf("Failed to open seed file: %v", err)
	}
	defer jsonFile.Close()

	byteValue, _ := ioutil.ReadAll(jsonFile)
	var seed SeedData
	json.Unmarshal(byteValue, &seed)

	seedTitles := make(map[string]bool)
	for _, r := range seed.Recipes {
		seedTitles[r.Title] = true
	}

	rows, err := database.DB.Query("SELECT title FROM recipes")
	if err != nil {
		log.Fatalf("Failed to query database: %v", err)
	}
	defer rows.Close()

	var dbTitles []string
	for rows.Next() {
		var title string
		if err := rows.Scan(&title); err != nil {
			log.Fatalf("Failed to scan row: %v", err)
		}
		dbTitles = append(dbTitles, title)
	}

	log.Printf("DB Total Recipes: %d", len(dbTitles))
	log.Printf("Seed Total Recipes: %d", len(seedTitles))

	extraInDB := 0
	for _, title := range dbTitles {
		if !seedTitles[title] {
			log.Printf("Warning: Database contains recipe '%s' which is not in seed_data.json!", title)
			extraInDB++
		}
	}

	if extraInDB == 0 {
		log.Println("Success: Database is in 100% sync with seed_data.json. No extra records found!")
	}
}
