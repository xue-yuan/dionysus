package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/xue-yuan/dionysus/internal/config"
	"github.com/xue-yuan/dionysus/internal/database"
)

// Embedded so the seeder no longer depends on being run from the server root.
//
//go:embed seed_data.json
var embeddedSeed []byte

type SeedData struct {
	Ingredients []IngredientSeed `json:"ingredients"`
	Tags        []TagSeed        `json:"tags"`
	Recipes     []RecipeSeed     `json:"recipes"`
}

type IngredientSeed struct {
	Name           string `json:"name"`
	Category       string `json:"category"`
	TranslationKey string `json:"translation_key"`
	Family         string `json:"family"`
}

type TagSeed struct {
	Name           string `json:"name"`
	Type           string `json:"type"`
	TranslationKey string `json:"translation_key"`
}

type RecipeIngredientSeed struct {
	Name   string `json:"name"`
	Amount string `json:"amount"`
	Unit   string `json:"unit"`
	Strict bool   `json:"strict"`
}

type RecipeSeed struct {
	Title          string                 `json:"title"`
	Description    string                 `json:"description"`
	Glassware      string                 `json:"glassware"`
	Method         string                 `json:"method"`
	Steps          string                 `json:"steps"`
	Sweetness      int                    `json:"sweetness"`
	Sourness       int                    `json:"sourness"`
	Strength       int                    `json:"strength"`
	TranslationKey string                 `json:"translation_key"`
	Ingredients    []RecipeIngredientSeed `json:"ingredients"`
	Tags           []string               `json:"tags"`
}

func main() {
	file := flag.String("file", "", "read seed data from this path instead of the embedded copy")
	flag.Parse()

	raw := embeddedSeed
	if *file != "" {
		var err error
		if raw, err = os.ReadFile(*file); err != nil {
			log.Fatalf("read %s: %v", *file, err)
		}
	}

	var data SeedData
	if err := json.Unmarshal(raw, &data); err != nil {
		log.Fatalf("parse seed data: %v", err)
	}

	cfg := config.Load()
	db, err := database.Connect(cfg)
	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if err := database.Migrate(ctx, db); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	if err := seed(ctx, db, data); err != nil {
		log.Fatalf("seed: %v", err)
	}
	log.Println("seeding complete")
}

// seed upserts the whole dataset in one transaction, so a reference to a missing
// ingredient aborts the run instead of leaving the database half-updated.
func seed(ctx context.Context, db *sqlx.DB, data SeedData) error {
	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	ingredientIDs := make(map[string]string, len(data.Ingredients))
	for _, ing := range data.Ingredients {
		var id string
		err := tx.GetContext(ctx, &id, `
			INSERT INTO ingredients (name, category, translation_key, family)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (name) DO UPDATE
			SET category = EXCLUDED.category,
			    translation_key = EXCLUDED.translation_key,
			    family = EXCLUDED.family
			RETURNING id`, ing.Name, ing.Category, ing.TranslationKey, ing.Family)
		if err != nil {
			return fmt.Errorf("ingredient %q: %w", ing.Name, err)
		}
		ingredientIDs[ing.Name] = id
	}
	log.Printf("ingredients upserted: %d", len(ingredientIDs))

	tagIDs := make(map[string]string, len(data.Tags))
	for _, t := range data.Tags {
		var id string
		err := tx.GetContext(ctx, &id, `
			INSERT INTO tags (name, type, translation_key)
			VALUES ($1, $2, $3)
			ON CONFLICT (name) DO UPDATE
			SET type = EXCLUDED.type, translation_key = EXCLUDED.translation_key
			RETURNING id`, t.Name, t.Type, t.TranslationKey)
		if err != nil {
			return fmt.Errorf("tag %q: %w", t.Name, err)
		}
		tagIDs[t.Name] = id
	}
	log.Printf("tags upserted: %d", len(tagIDs))

	for _, r := range data.Recipes {
		var recipeID string
		err := tx.GetContext(ctx, &recipeID, `
			INSERT INTO recipes (title, description, glassware, method, steps,
			                     sweetness, sourness, strength, translation_key)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			ON CONFLICT (title) DO UPDATE
			SET description = EXCLUDED.description,
			    glassware = EXCLUDED.glassware,
			    method = EXCLUDED.method,
			    steps = EXCLUDED.steps,
			    sweetness = EXCLUDED.sweetness,
			    sourness = EXCLUDED.sourness,
			    strength = EXCLUDED.strength,
			    translation_key = EXCLUDED.translation_key
			RETURNING id`,
			r.Title, r.Description, r.Glassware, r.Method, r.Steps,
			r.Sweetness, r.Sourness, r.Strength, r.TranslationKey)
		if err != nil {
			return fmt.Errorf("recipe %q: %w", r.Title, err)
		}

		for _, ri := range r.Ingredients {
			id, ok := ingredientIDs[ri.Name]
			if !ok {
				return fmt.Errorf("recipe %q references unknown ingredient %q", r.Title, ri.Name)
			}
			_, err = tx.ExecContext(ctx, `
				INSERT INTO recipe_ingredients (recipe_id, ingredient_id, amount, unit, strict)
				VALUES ($1, $2, $3, $4, $5)
				ON CONFLICT (recipe_id, ingredient_id) DO UPDATE
				SET amount = EXCLUDED.amount, unit = EXCLUDED.unit, strict = EXCLUDED.strict`,
				recipeID, id, ri.Amount, ri.Unit, ri.Strict)
			if err != nil {
				return fmt.Errorf("recipe %q ingredient %q: %w", r.Title, ri.Name, err)
			}
		}

		for _, name := range r.Tags {
			id, ok := tagIDs[name]
			if !ok {
				return fmt.Errorf("recipe %q references unknown tag %q", r.Title, name)
			}
			_, err = tx.ExecContext(ctx, `
				INSERT INTO recipe_tags (recipe_id, tag_id) VALUES ($1, $2)
				ON CONFLICT (recipe_id, tag_id) DO NOTHING`, recipeID, id)
			if err != nil {
				return fmt.Errorf("recipe %q tag %q: %w", r.Title, name, err)
			}
		}
	}
	log.Printf("recipes upserted: %d", len(data.Recipes))

	return tx.Commit()
}
