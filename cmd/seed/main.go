package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"

	"github.com/jmoiron/sqlx"
	"github.com/xue-yuan/dionysus/internal/config"
	"github.com/xue-yuan/dionysus/internal/database"
)

type SeedData struct {
	Ingredients []IngredientSeed `json:"ingredients"`
	Tags        []TagSeed        `json:"tags"`
	Recipes     []RecipeSeed     `json:"recipes"`
}

type IngredientSeed struct {
	Name     string `json:"name"`
	Category string `json:"category"`
}

type TagSeed struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type RecipeIngredientSeed struct {
	Name   string `json:"name"`
	Amount string `json:"amount"`
	Unit   string `json:"unit"`
}

type RecipeSeed struct {
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Glassware   string                 `json:"glassware"`
	Method      string                 `json:"method"`
	Steps       string                 `json:"steps"`
	Sweetness   int                    `json:"sweetness"`
	Sourness    int                    `json:"sourness"`
	Strength    int                    `json:"strength"`
	Ingredients []RecipeIngredientSeed `json:"ingredients"`
	Tags        []string               `json:"tags"`
}

func main() {
	cfg := config.Load()
	if err := database.Connect(cfg); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	seedData(database.DB)
}

func seedData(db *sqlx.DB) {
	log.Println("Starting Smart Database Seeding...")

	jsonFile, err := os.Open("cmd/seed/seed_data.json")
	if err != nil {
		log.Fatalf("Failed to open seed_data.json: %v", err)
	}
	defer jsonFile.Close()

	byteValue, _ := ioutil.ReadAll(jsonFile)

	var data SeedData
	if err := json.Unmarshal(byteValue, &data); err != nil {
		log.Fatalf("Failed to parse JSON: %v", err)
	}

	ingMap := make(map[string]string)
	for _, ing := range data.Ingredients {
		var id string
		err := db.Get(&id, "SELECT id FROM ingredients WHERE name = $1", ing.Name)
		if err == nil {
			ingMap[ing.Name] = id
		} else {
			err := db.QueryRow(`INSERT INTO ingredients (name, category) VALUES ($1, $2) RETURNING id`, ing.Name, ing.Category).Scan(&id)
			if err != nil {
				log.Printf("Error inserting ingredient %s: %v", ing.Name, err)
				continue
			}
			log.Printf("Inserted new ingredient: %s", ing.Name)
			ingMap[ing.Name] = id
		}
	}
	log.Printf("Processed %d ingredients.", len(data.Ingredients))

	tagMap := make(map[string]string)
	for _, t := range data.Tags {
		var id string
		err := db.Get(&id, "SELECT id FROM tags WHERE name = $1 AND type = $2", t.Name, t.Type)
		if err == nil {
			tagMap[t.Name] = id
		} else {
			err := db.QueryRow(`INSERT INTO tags (name, type) VALUES ($1, $2) RETURNING id`, t.Name, t.Type).Scan(&id)
			if err != nil {
				log.Printf("Error inserting tag %s: %v", t.Name, err)
				continue
			}
			log.Printf("Inserted new tag: %s", t.Name)
			tagMap[t.Name] = id
		}
	}
	log.Printf("Processed %d tags.", len(data.Tags))

	for _, r := range data.Recipes {
		var recipeID string
		err := db.Get(&recipeID, "SELECT id FROM recipes WHERE title = $1", r.Title)

		if err == nil {
			log.Printf("Recipe '%s' already exists. Checking associations...", r.Title)
		} else {
			err = db.QueryRow(`
				INSERT INTO recipes (title, description, glassware, method, steps, sweetness, sourness, strength) 
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8) 
				RETURNING id`,
				r.Title, r.Description, r.Glassware, r.Method, r.Steps,
				r.Sweetness, r.Sourness, r.Strength).Scan(&recipeID)

			if err != nil {
				log.Printf("Error seeding recipe %s: %v", r.Title, err)
				continue
			}
			log.Printf("Inserted new recipe: %s", r.Title)
		}

		for _, ri := range r.Ingredients {
			ingID, ok := ingMap[ri.Name]
			if !ok {
				err := db.Get(&ingID, "SELECT id FROM ingredients WHERE name=$1", ri.Name)
				if err != nil {
					log.Printf("WARNING: Missing ingredient '%s' for recipe '%s'. Skipping.", ri.Name, r.Title)
					continue
				}
			}
			_, err = db.Exec(`
				INSERT INTO recipe_ingredients (recipe_id, ingredient_id, amount, unit) 
				VALUES ($1, $2, $3, $4)
				ON CONFLICT (recipe_id, ingredient_id) DO UPDATE 
				SET amount = EXCLUDED.amount, unit = EXCLUDED.unit`,
				recipeID, ingID, ri.Amount, ri.Unit)

			if err != nil {
				log.Printf("Error linking ingredient %s to %s: %v", ri.Name, r.Title, err)
			}
		}

		for _, t := range r.Tags {
			tagID, ok := tagMap[t]
			if !ok {
				err := db.Get(&tagID, "SELECT id FROM tags WHERE name=$1", t)
				if err != nil {
					log.Printf("WARNING: Missing tag '%s' for recipe '%s'. Skipping.", t, r.Title)
					continue
				}
			}
			_, err = db.Exec(`
				INSERT INTO recipe_tags (recipe_id, tag_id) 
				VALUES ($1, $2)
				ON CONFLICT (recipe_id, tag_id) DO NOTHING`,
				recipeID, tagID)

			if err != nil {
				log.Printf("Error linking tag %s to %s: %v", t, r.Title, err)
			}
		}
	}

	log.Printf("Seeding check complete.")
}
