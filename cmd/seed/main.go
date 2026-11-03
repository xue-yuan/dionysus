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
	Base           string `json:"base"`
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

// seed makes the database match the seed file, in one transaction, so a
// reference to a missing ingredient aborts the run instead of leaving the
// database half-updated.
//
// It both upserts and prunes. Upsert alone could not express a removal: correcting
// French 75 from soda water to champagne would have added the champagne row and
// left the soda water one behind, leaving a recipe that demanded both. So
// anything the seed file no longer declares is deleted — per recipe for the join
// tables, and globally for recipes, ingredients and tags.
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
			INSERT INTO ingredients (name, category, translation_key, family, base)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (name) DO UPDATE
			SET category = EXCLUDED.category,
			    translation_key = EXCLUDED.translation_key,
			    family = EXCLUDED.family,
			    base = EXCLUDED.base
			RETURNING id`, ing.Name, ing.Category, ing.TranslationKey, ing.Family, ing.Base)
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

	var staleLinks int64
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

		wantIngredients := make([]string, 0, len(r.Ingredients))
		for _, ri := range r.Ingredients {
			id, ok := ingredientIDs[ri.Name]
			if !ok {
				return fmt.Errorf("recipe %q references unknown ingredient %q", r.Title, ri.Name)
			}
			wantIngredients = append(wantIngredients, id)
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

		wantTags := make([]string, 0, len(r.Tags))
		for _, name := range r.Tags {
			id, ok := tagIDs[name]
			if !ok {
				return fmt.Errorf("recipe %q references unknown tag %q", r.Title, name)
			}
			wantTags = append(wantTags, id)
			_, err = tx.ExecContext(ctx, `
				INSERT INTO recipe_tags (recipe_id, tag_id) VALUES ($1, $2)
				ON CONFLICT (recipe_id, tag_id) DO NOTHING`, recipeID, id)
			if err != nil {
				return fmt.Errorf("recipe %q tag %q: %w", r.Title, name, err)
			}
		}

		// Drop links this recipe no longer declares. Without this an ingredient
		// swap is additive and the recipe ends up asking for both bottles.
		n, err := pruneLinks(ctx, tx, "recipe_ingredients", "ingredient_id", recipeID, wantIngredients)
		if err != nil {
			return fmt.Errorf("recipe %q stale ingredients: %w", r.Title, err)
		}
		staleLinks += n

		n, err = pruneLinks(ctx, tx, "recipe_tags", "tag_id", recipeID, wantTags)
		if err != nil {
			return fmt.Errorf("recipe %q stale tags: %w", r.Title, err)
		}
		staleLinks += n
	}
	log.Printf("recipes upserted: %d", len(data.Recipes))
	if staleLinks > 0 {
		log.Printf("stale recipe links removed: %d", staleLinks)
	}

	// Rows the seed file dropped entirely. Recipes go first so their join rows
	// cascade away before the ingredients and tags they point at.
	for _, p := range []struct {
		label, query string
		keep         []string
	}{
		{"recipe", "DELETE FROM recipes WHERE title NOT IN (?)", titles(data)},
		{"ingredient", "DELETE FROM ingredients WHERE name NOT IN (?)", ingredientNames(data)},
		{"tag", "DELETE FROM tags WHERE name NOT IN (?)", tagNames(data)},
	} {
		n, err := prune(ctx, tx, p.query, p.keep)
		if err != nil {
			return fmt.Errorf("prune %ss: %w", p.label, err)
		}
		if n > 0 {
			log.Printf("%ss removed: %d", p.label, n)
		}
	}

	return tx.Commit()
}

// prune deletes every row whose key is absent from keep. An empty keep list is
// refused rather than interpreted as "delete everything" — that would turn a
// misparsed seed file into a wiped database.
func prune(ctx context.Context, tx *sqlx.Tx, query string, keep []string) (int64, error) {
	if len(keep) == 0 {
		return 0, fmt.Errorf("refusing to prune against an empty seed list")
	}
	q, args, err := sqlx.In(query, keep)
	if err != nil {
		return 0, err
	}
	res, err := tx.ExecContext(ctx, tx.Rebind(q), args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// pruneLinks is prune scoped to one recipe's join table. A recipe that declares
// nothing at all is legal, and then every existing link is stale.
//
// table and column are literals from the two call sites below, never input.
func pruneLinks(ctx context.Context, tx *sqlx.Tx, table, column, recipeID string, keep []string) (int64, error) {
	q := "DELETE FROM " + table + " WHERE recipe_id = ?"
	args := []any{recipeID}

	if len(keep) > 0 {
		var err error
		if q, args, err = sqlx.In(q+" AND "+column+" NOT IN (?)", recipeID, keep); err != nil {
			return 0, err
		}
	}

	res, err := tx.ExecContext(ctx, tx.Rebind(q), args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func titles(d SeedData) []string {
	out := make([]string, 0, len(d.Recipes))
	for _, r := range d.Recipes {
		out = append(out, r.Title)
	}
	return out
}

func ingredientNames(d SeedData) []string {
	out := make([]string, 0, len(d.Ingredients))
	for _, i := range d.Ingredients {
		out = append(out, i.Name)
	}
	return out
}

func tagNames(d SeedData) []string {
	out := make([]string, 0, len(d.Tags))
	for _, t := range d.Tags {
		out = append(out, t.Name)
	}
	return out
}
