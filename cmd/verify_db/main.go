// Command verify_db reports rows that exist in the database but not in the seed
// file, which is how leftovers from manual experiments get spotted.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"
	"time"

	"github.com/xue-yuan/dionysus/internal/config"
	"github.com/xue-yuan/dionysus/internal/database"
)

type SeedData struct {
	Ingredients []struct {
		Name string `json:"name"`
	} `json:"ingredients"`
	Recipes []struct {
		Title string `json:"title"`
	} `json:"recipes"`
}

func main() {
	file := flag.String("file", "cmd/seed/seed_data.json", "path to seed_data.json")
	flag.Parse()

	raw, err := os.ReadFile(*file)
	if err != nil {
		log.Fatalf("read %s: %v", *file, err)
	}
	var seed SeedData
	if err := json.Unmarshal(raw, &seed); err != nil {
		log.Fatalf("parse %s: %v", *file, err)
	}

	cfg := config.Load()
	db, err := database.Connect(cfg)
	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	extra := 0
	extra += report(ctx, db, "recipe", "SELECT title FROM recipes", titles(seed))
	extra += report(ctx, db, "ingredient", "SELECT name FROM ingredients", names(seed))

	if extra == 0 {
		log.Println("database matches the seed file")
	} else {
		log.Printf("%d row(s) in the database are not in the seed file", extra)
		os.Exit(1)
	}
}

func titles(s SeedData) map[string]bool {
	m := make(map[string]bool, len(s.Recipes))
	for _, r := range s.Recipes {
		m[r.Title] = true
	}
	return m
}

func names(s SeedData) map[string]bool {
	m := make(map[string]bool, len(s.Ingredients))
	for _, i := range s.Ingredients {
		m[i.Name] = true
	}
	return m
}

func report(ctx context.Context, db interface {
	SelectContext(context.Context, any, string, ...any) error
}, label, query string, expected map[string]bool) int {
	var found []string
	if err := db.SelectContext(ctx, &found, query); err != nil {
		log.Fatalf("query %ss: %v", label, err)
	}

	extra := 0
	for _, name := range found {
		if !expected[name] {
			log.Printf("extra %s in database: %q", label, name)
			extra++
		}
	}
	log.Printf("%ss — database: %d, seed: %d", label, len(found), len(expected))
	return extra
}
