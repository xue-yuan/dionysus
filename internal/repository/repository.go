// Package repository
package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/xue-yuan/dionysus/internal/model"
)

type Repository struct {
	DB *sqlx.DB
}

func New(db *sqlx.DB) *Repository {
	return &Repository{DB: db}
}

const recipeColumns = `r.id, r.title, r.description, r.glassware, r.method, r.steps,
	r.image_url, r.sweetness, r.sourness, r.strength, r.translation_key, r.created_at`

func (r *Repository) GetIngredients(ctx context.Context) ([]model.Ingredient, error) {
	ingredients := []model.Ingredient{}
	err := r.DB.SelectContext(ctx, &ingredients,
		`SELECT id, name, category, translation_key, family FROM ingredients ORDER BY name ASC`)
	return ingredients, err
}

func (r *Repository) GetTags(ctx context.Context) ([]model.Tag, error) {
	tags := []model.Tag{}
	err := r.DB.SelectContext(ctx, &tags,
		`SELECT id, name, type, translation_key FROM tags ORDER BY type ASC, name ASC`)
	return tags, err
}

// escapeLike neutralises the LIKE metacharacters so a search for "50%" does not
// turn into a wildcard that matches every row.
func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

// RecipeQuery describes one page of the archive.
//
// TranslationKeys carries the keys whose localised text matched the search. The
// server holds only the English title and description, so a reader searching in
// Chinese would otherwise match nothing; the client resolves the query against
// its own dictionary and passes the keys it found. Empty unless Search is set.
//
// TagIDs filters as tagFilter describes. Base keeps recipes that call for any
// bottle with that base ("Brandy" takes in Cognac, Pisco and apple brandy), which
// is what "brandy drinks" means on the archive page. It is deliberately not
// family: family is about substitution, and Pisco must not stand in for Cognac.
type RecipeQuery struct {
	Limit           int
	Offset          int
	SortDirection   string
	Search          string
	TranslationKeys []string
	TagIDs          []string
	Base            string
}

// tagFilter keeps recipes that carry, for every type among the selected tags,
// at least one selected tag of that type. Within a type the tags are
// alternatives (Sour or Fruity); across types they narrow (Tiki and Sour). A
// single ANY over everything made picking "Tiki" and "Sour" return every sour
// drink plus every tiki drink.
//
// param is the placeholder holding the uuid array, positional or named.
func tagFilter(param string) string {
	return fmt.Sprintf(`NOT EXISTS (
		SELECT 1 FROM tags st
		WHERE st.id = ANY(CAST(%[1]s AS uuid[]))
		  AND NOT EXISTS (
			SELECT 1 FROM recipe_tags rt
			JOIN tags rtt ON rtt.id = rt.tag_id
			WHERE rt.recipe_id = r.id
			  AND rtt.type = st.type
			  AND rt.tag_id = ANY(CAST(%[1]s AS uuid[]))
		  )
	)`, param)
}

// recipeWhere builds the archive's WHERE clause and its arguments. Placeholders
// start at $next, because the list query keeps $1 and $2 for the page window.
func recipeWhere(q RecipeQuery, next int) (string, []any) {
	var conds []string
	var args []any
	arg := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", next+len(args)-1)
	}

	if q.Search != "" {
		like := arg("%" + escapeLike(q.Search) + "%")
		keys := arg(pq.Array(q.TranslationKeys))
		conds = append(conds, fmt.Sprintf(`(r.title ILIKE %[1]s ESCAPE '\'
		      OR r.description ILIKE %[1]s ESCAPE '\'
		      OR r.translation_key = ANY(CAST(%[2]s AS text[])))`, like, keys))
	}
	if len(q.TagIDs) > 0 {
		conds = append(conds, tagFilter(arg(pq.Array(q.TagIDs))))
	}
	if q.Base != "" {
		conds = append(conds, fmt.Sprintf(`EXISTS (
			SELECT 1 FROM recipe_ingredients ri
			JOIN ingredients i ON i.id = ri.ingredient_id
			WHERE ri.recipe_id = r.id AND i.base = %s)`, arg(q.Base)))
	}

	if len(conds) == 0 {
		return "", nil
	}
	return "WHERE " + strings.Join(conds, " AND "), args
}

// recipeRow is a page row plus the window count, so one round trip reports both
// the page and how many rows the filter matched.
type recipeRow struct {
	model.Recipe
	TotalCount int64 `db:"total_count"`
}

// GetRecipes returns one page of recipes ordered by strength. r.id is always the
// final sort key: strength has only a handful of distinct values, and without a
// unique tiebreaker PostgreSQL is free to order ties differently on every query,
// which makes OFFSET pagination skip and repeat rows.
func (r *Repository) GetRecipes(ctx context.Context, q RecipeQuery) ([]model.Recipe, int64, error) {
	order := "DESC"
	if q.SortDirection == "asc" {
		order = "ASC"
	}

	// $1 and $2 are always the page window; the filter args follow.
	where, filterArgs := recipeWhere(q, 3)
	args := append([]any{q.Limit, q.Offset}, filterArgs...)

	// COUNT(*) OVER () is evaluated after WHERE but before LIMIT, so it reports
	// the size of the whole filtered set without a second query.
	rows := []recipeRow{}
	listQuery := fmt.Sprintf(`
		SELECT %s,
		       (SELECT COUNT(*) FROM recipe_ingredients ri WHERE ri.recipe_id = r.id) AS total_ingredients,
		       COUNT(*) OVER () AS total_count
		FROM recipes r
		%s
		ORDER BY r.strength %s, r.id %s
		LIMIT $1 OFFSET $2`, recipeColumns, where, order, order)

	if err := r.DB.SelectContext(ctx, &rows, listQuery, args...); err != nil {
		return nil, 0, err
	}

	recipes := make([]model.Recipe, len(rows))
	for i, row := range rows {
		recipes[i] = row.Recipe
	}

	var total int64
	if len(rows) > 0 {
		total = rows[0].TotalCount
	} else {
		// An empty page carries no window count, and that is either a genuinely
		// empty result or an offset past the end. Only a count separates them.
		var err error
		if total, err = r.countRecipes(ctx, q); err != nil {
			return nil, 0, err
		}
	}

	if err := r.attachTags(ctx, recipes); err != nil {
		return nil, 0, err
	}
	return recipes, total, nil
}

func (r *Repository) countRecipes(ctx context.Context, q RecipeQuery) (int64, error) {
	var total int64
	where, args := recipeWhere(q, 1)
	err := r.DB.GetContext(ctx, &total, `SELECT COUNT(*) FROM recipes r `+where, args...)
	return total, err
}

// GetRecipesByIDs fetches a specific set of recipes, which lets the saved-recipes
// page ask for exactly what it needs instead of downloading the whole archive.
func (r *Repository) GetRecipesByIDs(ctx context.Context, ids []string) ([]model.Recipe, error) {
	recipes := []model.Recipe{}
	if len(ids) == 0 {
		return recipes, nil
	}

	query := fmt.Sprintf(`
		SELECT %s,
		       (SELECT COUNT(*) FROM recipe_ingredients ri WHERE ri.recipe_id = r.id) AS total_ingredients
		FROM recipes r
		WHERE r.id = ANY($1::uuid[])
		ORDER BY r.strength DESC, r.id DESC`, recipeColumns)

	if err := r.DB.SelectContext(ctx, &recipes, query, pq.Array(ids)); err != nil {
		return nil, err
	}
	if err := r.attachTags(ctx, recipes); err != nil {
		return nil, err
	}
	return recipes, nil
}

// tagsByRecipe hydrates tags for a page of recipes in one round trip.
func (r *Repository) tagsByRecipe(ctx context.Context, ids []string) (map[string][]model.Tag, error) {
	byRecipe := make(map[string][]model.Tag, len(ids))
	if len(ids) == 0 {
		return byRecipe, nil
	}

	rows, err := r.DB.QueryxContext(ctx, `
		SELECT rt.recipe_id, t.id, t.name, t.type, t.translation_key
		FROM recipe_tags rt
		JOIN tags t ON rt.tag_id = t.id
		WHERE rt.recipe_id = ANY($1::uuid[])`, pq.Array(ids))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var recipeID string
		var t model.Tag
		if err := rows.Scan(&recipeID, &t.ID, &t.Name, &t.Type, &t.TranslationKey); err != nil {
			return nil, err
		}
		byRecipe[recipeID] = append(byRecipe[recipeID], t)
	}
	return byRecipe, rows.Err()
}

func (r *Repository) attachTags(ctx context.Context, recipes []model.Recipe) error {
	ids := make([]string, len(recipes))
	for i := range recipes {
		recipes[i].Tags = []model.Tag{}
		ids[i] = recipes[i].ID.String()
	}

	byRecipe, err := r.tagsByRecipe(ctx, ids)
	if err != nil {
		return err
	}
	for i := range recipes {
		if tags, ok := byRecipe[recipes[i].ID.String()]; ok {
			recipes[i].Tags = tags
		}
	}
	return nil
}

func (r *Repository) GetRecipeWithIngredients(ctx context.Context, id uuid.UUID) (*model.Recipe, error) {
	var recipe model.Recipe
	query := fmt.Sprintf(`
		SELECT %s,
		       (SELECT COUNT(*) FROM recipe_ingredients ri WHERE ri.recipe_id = r.id) AS total_ingredients
		FROM recipes r WHERE r.id = $1`, recipeColumns)
	if err := r.DB.GetContext(ctx, &recipe, query, id); err != nil {
		return nil, err
	}

	recipe.Ingredients = []model.RecipeIngredientDetail{}
	err := r.DB.SelectContext(ctx, &recipe.Ingredients, `
		SELECT i.id AS ingredient_id, i.name, i.category, i.translation_key, i.family,
		       ri.amount, ri.unit, ri.strict
		FROM recipe_ingredients ri
		JOIN ingredients i ON ri.ingredient_id = i.id
		WHERE ri.recipe_id = $1
		ORDER BY i.name`, id)
	if err != nil {
		return nil, fmt.Errorf("fetch ingredients for recipe %s: %w", id, err)
	}

	recipe.Tags = []model.Tag{}
	err = r.DB.SelectContext(ctx, &recipe.Tags, `
		SELECT t.id, t.name, t.type, t.translation_key
		FROM recipe_tags rt
		JOIN tags t ON rt.tag_id = t.id
		WHERE rt.recipe_id = $1
		ORDER BY t.name`, id)
	if err != nil {
		return nil, fmt.Errorf("fetch tags for recipe %s: %w", id, err)
	}

	return &recipe, nil
}

// GetRecipeByKey looks a recipe up by its translation key, which is what the
// client's recipe URLs carry. Ids differ between databases seeded separately,
// so a link built from one would not open on another.
func (r *Repository) GetRecipeByKey(ctx context.Context, key string) (*model.Recipe, error) {
	var id uuid.UUID
	if err := r.DB.GetContext(ctx, &id, `SELECT id FROM recipes WHERE translation_key = $1`, key); err != nil {
		return nil, err
	}
	return r.GetRecipeWithIngredients(ctx, id)
}

// MatchQuery describes what the user has and what they will accept.
//
// TagIDs narrows the result as tagFilter describes (palate and style).
// ExcludeTagIDs drops recipes carrying any of them, which is how equipment
// works: the user unticks a tool they do not own, and every recipe that needs
// it disappears. Equipment used to go through TagIDs, so ticking "Shaker" hid
// every stirred drink — the opposite of "I have a shaker".
//
// MinStrength and MaxStrength bound the 1–5 strength scale; zero means no bound.
type MatchQuery struct {
	OwnedIngredientIDs []string
	MinStrength        int
	MaxStrength        int
	TagIDs             []string
	ExcludeTagIDs      []string
	AllowSubstitutes   bool
}

// matchSQL resolves, for every recipe, how many of its ingredients the user is
// short of, and which of their bottles stand in for the ones they lack.
//
// Substitution is a property of the requirement, not of the user's shelf. A
// requirement is either strict (only that exact bottle will do) or open to any
// bottle in the same family. Family requirements are then satisfied by counting:
// a recipe calling for three different rums needs three distinct rums on the
// shelf, not one rum three times. Strict requirements are assigned first because
// they can only be met one way; whatever is left over covers the flexible ones.
// That greedy order is optimal here, which is what keeps the counting honest.
//
// Casts are spelled CAST(x AS t) rather than x::t because sqlx reads "::" as the
// start of a named parameter and refuses to compile the query.
const matchSQL = `
WITH owned AS (
	SELECT i.id, i.family FROM ingredients i WHERE i.id = ANY(CAST(:owned_ids AS uuid[]))
),
req AS (
	SELECT ri.recipe_id, ri.ingredient_id, ri.strict, i.family,
	       EXISTS (SELECT 1 FROM owned o WHERE o.id = ri.ingredient_id) AS owned
	FROM recipe_ingredients ri
	JOIN ingredients i ON i.id = ri.ingredient_id
),
exact_req AS (
	SELECT * FROM req WHERE strict OR family = '' OR NOT :allow_subs
),
fam_req AS (
	SELECT * FROM req WHERE NOT strict AND family <> '' AND :allow_subs
),
owned_by_family AS (
	SELECT family, COUNT(*) AS have FROM owned WHERE family <> '' GROUP BY family
),
-- An exact requirement the user satisfies uses up one bottle of that family, so
-- it can no longer stand in for a flexible requirement.
consumed AS (
	SELECT recipe_id, family, COUNT(*) AS used
	FROM exact_req WHERE owned AND family <> '' GROUP BY recipe_id, family
),
fam_need AS (
	SELECT recipe_id, family, COUNT(*) AS need FROM fam_req GROUP BY recipe_id, family
),
fam_short AS (
	SELECT n.recipe_id, n.family,
	       GREATEST(0, n.need - GREATEST(0, COALESCE(o.have, 0) - COALESCE(c.used, 0))) AS short
	FROM fam_need n
	LEFT JOIN owned_by_family o ON o.family = n.family
	LEFT JOIN consumed c ON c.recipe_id = n.recipe_id AND c.family = n.family
),
fam_unowned AS (
	SELECT f.recipe_id, f.family, f.ingredient_id,
	       ROW_NUMBER() OVER (PARTITION BY f.recipe_id, f.family ORDER BY f.ingredient_id) AS rn
	FROM fam_req f WHERE NOT f.owned
),
missing AS (
	SELECT recipe_id, ingredient_id FROM exact_req WHERE NOT owned
	UNION ALL
	SELECT u.recipe_id, u.ingredient_id
	FROM fam_unowned u
	JOIN fam_short s ON s.recipe_id = u.recipe_id AND s.family = u.family
	WHERE u.rn <= s.short
),
missing_agg AS (
	SELECT recipe_id, COUNT(*) AS missing_count,
	       ARRAY_AGG(CAST(ingredient_id AS text)) AS missing_ids
	FROM missing GROUP BY recipe_id
),
-- Flexible requirements the user lacks but the family can cover. These are the
-- ones an actual substitution is made for. Note that comments in this query must
-- not contain a colon, because sqlx scans the raw string for named parameters
-- and does not skip over SQL comments.
substituted AS (
	SELECT u.recipe_id, u.family, u.ingredient_id AS required_id,
	       u.rn - COALESCE(s.short, 0) AS pick
	FROM fam_unowned u
	LEFT JOIN fam_short s ON s.recipe_id = u.recipe_id AND s.family = u.family
	WHERE u.rn > COALESCE(s.short, 0)
),
-- Bottles of the right family that this recipe does not already call for.
spare AS (
	SELECT d.recipe_id, o.family, o.id AS substitute_id,
	       ROW_NUMBER() OVER (PARTITION BY d.recipe_id, o.family ORDER BY o.id) AS rn
	FROM (SELECT DISTINCT recipe_id, family FROM substituted) d
	JOIN owned o ON o.family = d.family
	WHERE NOT EXISTS (
		SELECT 1 FROM req q WHERE q.recipe_id = d.recipe_id AND q.ingredient_id = o.id
	)
),
subs_agg AS (
	SELECT sb.recipe_id,
	       ARRAY_AGG(CAST(sb.required_id AS text))   AS required_ids,
	       ARRAY_AGG(CAST(sp.substitute_id AS text)) AS substitute_ids,
	       ARRAY_AGG(sb.family)                      AS sub_families
	FROM substituted sb
	JOIN spare sp ON sp.recipe_id = sb.recipe_id AND sp.family = sb.family AND sp.rn = sb.pick
	GROUP BY sb.recipe_id
),
totals AS (
	SELECT recipe_id, COUNT(*) AS total_ingredients FROM recipe_ingredients GROUP BY recipe_id
)
SELECT
	r.id, r.title, r.description, r.image_url, r.glassware, r.method, r.translation_key,
	r.sweetness, r.sourness, r.strength,
	t.total_ingredients,
	t.total_ingredients - COALESCE(m.missing_count, 0) AS owned_count,
	COALESCE(m.missing_count, 0)                       AS missing_count,
	COALESCE(m.missing_ids, '{}')                      AS missing_ids,
	COALESCE(s.required_ids, '{}')                     AS required_ids,
	COALESCE(s.substitute_ids, '{}')                   AS substitute_ids,
	COALESCE(s.sub_families, '{}')                     AS sub_families
FROM recipes r
JOIN totals t ON t.recipe_id = r.id
LEFT JOIN missing_agg m ON m.recipe_id = r.id
LEFT JOIN subs_agg s ON s.recipe_id = r.id
WHERE COALESCE(m.missing_count, 0) <= 1`

// FindMatchingRecipes returns recipes the user can make now, or is one
// ingredient away from making.
func (r *Repository) FindMatchingRecipes(ctx context.Context, q MatchQuery) ([]model.RecipeMatch, error) {
	matches := []model.RecipeMatch{}
	if len(q.OwnedIngredientIDs) == 0 {
		return matches, nil
	}

	query := matchSQL
	args := map[string]any{
		"owned_ids":  pq.Array(q.OwnedIngredientIDs),
		"allow_subs": q.AllowSubstitutes,
	}

	if q.MinStrength > 0 {
		query += " AND r.strength >= :min_strength"
		args["min_strength"] = q.MinStrength
	}
	if q.MaxStrength > 0 {
		query += " AND r.strength <= :max_strength"
		args["max_strength"] = q.MaxStrength
	}
	if len(q.TagIDs) > 0 {
		query += " AND " + tagFilter(":tag_ids")
		args["tag_ids"] = pq.Array(q.TagIDs)
	}
	if len(q.ExcludeTagIDs) > 0 {
		query += ` AND NOT EXISTS (
			SELECT 1 FROM recipe_tags rt
			WHERE rt.recipe_id = r.id AND rt.tag_id = ANY(CAST(:exclude_tag_ids AS uuid[]))
		)`
		args["exclude_tag_ids"] = pq.Array(q.ExcludeTagIDs)
	}
	query += " ORDER BY missing_count ASC, r.title ASC"

	rows, err := r.DB.NamedQueryContext(ctx, query, args)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var m model.RecipeMatch
		var missingIDs, requiredIDs, substituteIDs, subFamilies pq.StringArray

		if err := rows.Scan(
			&m.ID, &m.Title, &m.Description, &m.ImageURL, &m.Glassware, &m.Method,
			&m.TranslationKey, &m.Sweetness, &m.Sourness, &m.Strength,
			&m.TotalIngredients, &m.OwnedCount, &m.MissingCount,
			&missingIDs, &requiredIDs, &substituteIDs, &subFamilies,
		); err != nil {
			return nil, err
		}

		m.MissingIngredients = missingIDs
		for i := range requiredIDs {
			m.Substitutions = append(m.Substitutions, model.Substitution{
				RequiredID:   requiredIDs[i],
				SubstituteID: substituteIDs[i],
				Family:       subFamilies[i],
			})
		}
		matches = append(matches, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if err := r.attachMatchTags(ctx, matches); err != nil {
		return nil, err
	}
	return matches, nil
}

func (r *Repository) attachMatchTags(ctx context.Context, matches []model.RecipeMatch) error {
	ids := make([]string, len(matches))
	for i := range matches {
		matches[i].Tags = []model.Tag{}
		ids[i] = matches[i].ID
	}

	byRecipe, err := r.tagsByRecipe(ctx, ids)
	if err != nil {
		return err
	}
	for i := range matches {
		if tags, ok := byRecipe[matches[i].ID]; ok {
			matches[i].Tags = tags
		}
	}
	return nil
}
