package repository

import (
	"log"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/xue-yuan/dionysus/internal/model"
)

type Repository struct {
	DB *sqlx.DB
}

func New(db *sqlx.DB) *Repository {
	return &Repository{DB: db}
}

func (r *Repository) GetIngredients() ([]model.Ingredient, error) {
	var ingredients []model.Ingredient
	err := r.DB.Select(&ingredients, "SELECT * FROM ingredients ORDER BY name ASC")
	return ingredients, err
}

func (r *Repository) GetTags() ([]model.Tag, error) {
	var tags []model.Tag
	err := r.DB.Select(&tags, "SELECT * FROM tags ORDER BY type ASC, name ASC")
	return tags, err
}

func (r *Repository) CreateRecipe(recipe *model.Recipe, ingredients []model.RecipeIngredient) error {
	tx, err := r.DB.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	query := `INSERT INTO recipes (title, description, glassware, method, steps, image_url, sweetness, sourness, strength) 
              VALUES (:title, :description, :glassware, :method, :steps, :image_url, :sweetness, :sourness, :strength) 
              RETURNING id, created_at`
	stmt, err := tx.PrepareNamed(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	if err := stmt.QueryRowx(recipe).StructScan(recipe); err != nil {
		return err
	}

	for _, ri := range ingredients {
		ri.RecipeID = recipe.ID
		_, err := tx.NamedExec(`INSERT INTO recipe_ingredients (recipe_id, ingredient_id, amount, unit)
                                VALUES (:recipe_id, :ingredient_id, :amount, :unit)`, ri)
		if err != nil {
			return err
		}
	}

	for _, tag := range recipe.Tags {
		_, err := tx.Exec(`INSERT INTO recipe_tags (recipe_id, tag_id) VALUES ($1, $2)`, recipe.ID, tag.ID)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *Repository) GetRecipes(limit, offset int, sortDirection string) ([]model.Recipe, int64, error) {
	var recipes []model.Recipe
	var total int64
	err := r.DB.Get(&total, "SELECT COUNT(*) FROM recipes")
	if err != nil {
		return nil, 0, err
	}

	order := "DESC"
	if sortDirection == "asc" {
		order = "ASC"
	}

	query := `SELECT * FROM recipes ORDER BY strength ` + order + ` LIMIT $1 OFFSET $2`

	err = r.DB.Select(&recipes, query, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	if recipes == nil {
		recipes = []model.Recipe{}
	}
	if len(recipes) > 0 {
		recipeIDs := make([]string, len(recipes))
		for i, r := range recipes {
			recipeIDs[i] = r.ID.String()
		}

		queryTags := `
			SELECT rt.recipe_id, t.id, t.name, t.type
			FROM recipe_tags rt
			JOIN tags t ON rt.tag_id = t.id
			WHERE rt.recipe_id IN (?)
		`
		queryTags, args, err := sqlx.In(queryTags, recipeIDs)
		if err != nil {
			return nil, 0, err
		}
		queryTags = r.DB.Rebind(queryTags)

		rows, err := r.DB.Query(queryTags, args...)
		if err != nil {
			return nil, 0, err
		}
		defer rows.Close()

		tagMap := make(map[string][]model.Tag)
		for rows.Next() {
			var rID string
			var t model.Tag
			if err := rows.Scan(&rID, &t.ID, &t.Name, &t.Type); err != nil {
				return nil, 0, err
			}
			tagMap[rID] = append(tagMap[rID], t)
		}

		for i := range recipes {
			if tags, ok := tagMap[recipes[i].ID.String()]; ok {
				recipes[i].Tags = tags
			} else {
				recipes[i].Tags = []model.Tag{}
			}
		}
	}

	return recipes, total, nil
}

func (r *Repository) GetRecipeWithIngredients(id uuid.UUID) (*model.Recipe, error) {
	var recipe model.Recipe
	err := r.DB.Get(&recipe, "SELECT * FROM recipes WHERE id = $1", id)
	if err != nil {
		return nil, err
	}

	var details []model.RecipeIngredientDetail
	query := `
		SELECT i.id as ingredient_id, i.name, i.category, ri.amount, ri.unit
		FROM recipe_ingredients ri
		JOIN ingredients i ON ri.ingredient_id = i.id
		WHERE ri.recipe_id = $1
	`
	err = r.DB.Select(&details, query, id)
	if err != nil {
		log.Printf("Error fetching ingredients for recipe %s: %v", id, err)
		return nil, err
	}

	recipe.Ingredients = details

	var tags []model.Tag
	tagQuery := `
		SELECT t.id, t.name, t.type
		FROM recipe_tags rt
		JOIN tags t ON rt.tag_id = t.id
		WHERE rt.recipe_id = $1
	`
	err = r.DB.Select(&tags, tagQuery, id)
	if err != nil {
		log.Printf("Error fetching tags for recipe %s: %v", id, err)
	} else {
		recipe.Tags = tags
	}

	return &recipe, nil
}

func (r *Repository) FindMatchingRecipes(ownedIngredientIDs []string, minStrength int, tagIDs []string) ([]model.RecipeMatch, error) {
	if len(ownedIngredientIDs) == 0 {
		return []model.RecipeMatch{}, nil
	}
	query := `
		WITH recipe_counts AS (
			SELECT recipe_id, COUNT(*) as total_ingredients
			FROM recipe_ingredients
			GROUP BY recipe_id
		),
		user_matches AS (
			SELECT ri.recipe_id, COUNT(*) as owned_count
			FROM recipe_ingredients ri
			WHERE ri.ingredient_id = ANY(:owned_ids)
			GROUP BY ri.recipe_id
		)
		SELECT 
			r.id, r.title, r.description, r.image_url,
			r.glassware, r.method,
            r.sweetness, r.sourness, r.strength,
			rc.total_ingredients,
			COALESCE(um.owned_count, 0) as owned_count,
			(rc.total_ingredients - COALESCE(um.owned_count, 0)) as missing_count
		FROM recipes r
		JOIN recipe_counts rc ON r.id = rc.recipe_id
		LEFT JOIN user_matches um ON r.id = um.recipe_id
		WHERE (rc.total_ingredients - COALESCE(um.owned_count, 0)) <= 1
	`

	args := map[string]interface{}{
		"owned_ids": ownedIngredientIDs,
	}

	if minStrength > 0 {
		query += " AND r.strength >= :min_strength"
		args["min_strength"] = minStrength
	}

	if len(tagIDs) > 0 {
		query += ` AND EXISTS (
            SELECT 1 FROM recipe_tags rt 
            WHERE rt.recipe_id = r.id 
            AND rt.tag_id = ANY(:tag_ids)
        )`
		args["tag_ids"] = tagIDs
	}

	query += " ORDER BY missing_count ASC, r.title ASC"

	matches := []model.RecipeMatch{}

	rows, err := r.DB.NamedQuery(query, args)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var m model.RecipeMatch
		if err := rows.StructScan(&m); err != nil {
			return nil, err
		}
		matches = append(matches, m)
	}

	for i, m := range matches {
		if m.MissingCount > 0 {
			var missingID string
			query := "SELECT ingredient_id FROM recipe_ingredients WHERE recipe_id = ? AND ingredient_id NOT IN (?)"
			query, args, err := sqlx.In(query, m.ID, ownedIngredientIDs)
			if err != nil {
				log.Printf("Error constructing IN query for recipe %s: %v", m.ID, err)
				continue
			}
			query = r.DB.Rebind(query)

			err = r.DB.Get(&missingID, query, args...)
			if err == nil {
				matches[i].MissingIngredients = []string{missingID}
				log.Printf("Found missing ingredient for recipe %s: %s", m.ID, missingID)
			} else {
				log.Printf("Error fetching missing ingredient for recipe %s: %v. Query: %s Args: %v", m.ID, err, query, args)
			}
		}
	}

	if len(matches) > 0 {
		matchIDs := make([]string, len(matches))
		for i, m := range matches {
			matchIDs[i] = m.ID
		}

		queryTags := `
			SELECT rt.recipe_id, t.id, t.name, t.type
			FROM recipe_tags rt
			JOIN tags t ON rt.tag_id = t.id
			WHERE rt.recipe_id IN (?)
		`
		queryTags, args, err := sqlx.In(queryTags, matchIDs)
		if err != nil {
			return nil, err
		}
		queryTags = r.DB.Rebind(queryTags)

		rows, err := r.DB.Query(queryTags, args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		tagMap := make(map[string][]model.Tag)
		for rows.Next() {
			var rID string
			var t model.Tag
			if err := rows.Scan(&rID, &t.ID, &t.Name, &t.Type); err != nil {
				return nil, err
			}
			tagMap[rID] = append(tagMap[rID], t)
		}

		for i := range matches {
			if tags, ok := tagMap[matches[i].ID]; ok {
				matches[i].Tags = tags
			} else {
				matches[i].Tags = []model.Tag{}
			}
		}
	}

	return matches, nil
}
