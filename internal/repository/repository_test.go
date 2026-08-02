package repository_test

import (
	"context"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/xue-yuan/dionysus/internal/config"
	"github.com/xue-yuan/dionysus/internal/database"
	"github.com/xue-yuan/dionysus/internal/model"
	"github.com/xue-yuan/dionysus/internal/repository"
)

// The matcher is expressed almost entirely in SQL, so these run against a real
// seeded database. They skip rather than fail when one is not reachable.
func newRepo(t *testing.T) (*repository.Repository, *sqlx.DB) {
	t.Helper()

	db, err := database.Connect(config.Load())
	if err != nil {
		t.Skipf("no database available: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	var seeded int
	if err := db.Get(&seeded, `SELECT COUNT(*) FROM recipes`); err != nil || seeded == 0 {
		t.Skip("database is not seeded; run: go run ./cmd/seed")
	}
	return repository.New(db), db
}

// ids resolves ingredient names, failing loudly if the seed data has drifted.
func ids(t *testing.T, db *sqlx.DB, names ...string) []string {
	t.Helper()

	out := make([]string, 0, len(names))
	for _, name := range names {
		var id string
		if err := db.Get(&id, `SELECT id FROM ingredients WHERE name = $1`, name); err != nil {
			t.Fatalf("ingredient %q not found: %v", name, err)
		}
		out = append(out, id)
	}
	return out
}

// recipeIngredients returns every ingredient a recipe calls for, optionally
// leaving some out to simulate an incomplete shelf.
func recipeIngredients(t *testing.T, db *sqlx.DB, title string, except ...string) []string {
	t.Helper()

	var all []struct {
		ID   string `db:"id"`
		Name string `db:"name"`
	}
	err := db.Select(&all, `
		SELECT i.id, i.name FROM recipe_ingredients ri
		JOIN ingredients i ON i.id = ri.ingredient_id
		JOIN recipes r ON r.id = ri.recipe_id
		WHERE r.title = $1`, title)
	if err != nil || len(all) == 0 {
		t.Fatalf("recipe %q not found: %v", title, err)
	}

	skip := make(map[string]bool, len(except))
	for _, n := range except {
		skip[n] = true
	}

	out := make([]string, 0, len(all))
	for _, ing := range all {
		if !skip[ing.Name] {
			out = append(out, ing.ID)
		}
	}
	return out
}

func find(matches []model.RecipeMatch, title string) *model.RecipeMatch {
	for i := range matches {
		if matches[i].Title == title {
			return &matches[i]
		}
	}
	return nil
}

func match(t *testing.T, repo *repository.Repository, owned []string, allowSubs bool) []model.RecipeMatch {
	t.Helper()

	got, err := repo.FindMatchingRecipes(context.Background(), repository.MatchQuery{
		OwnedIngredientIDs: owned,
		AllowSubstitutes:   allowSubs,
	})
	if err != nil {
		t.Fatalf("FindMatchingRecipes: %v", err)
	}
	return got
}

// The behaviour the whole feature exists for: a recipe that just says "Bourbon"
// should be makeable with any other whiskey.
func TestFamilySubstitutionAcceptsAnyWhiskey(t *testing.T) {
	repo, db := newRepo(t)

	owned := recipeIngredients(t, db, "Old Fashioned", "Bourbon")
	owned = append(owned, ids(t, db, "Japanese Whisky")...)

	m := find(match(t, repo, owned, true), "Old Fashioned")
	if m == nil {
		t.Fatal("Old Fashioned should be makeable with Japanese Whisky standing in for Bourbon")
	}
	if m.MissingCount != 0 {
		t.Errorf("missing_count = %d, want 0", m.MissingCount)
	}

	if len(m.Substitutions) != 1 {
		t.Fatalf("substitutions = %d, want 1", len(m.Substitutions))
	}
	sub := m.Substitutions[0]
	if sub.Family != "Whiskey" {
		t.Errorf("substitution family = %q, want Whiskey", sub.Family)
	}
	if want := ids(t, db, "Bourbon")[0]; sub.RequiredID != want {
		t.Errorf("substituted for %s, want the Bourbon requirement %s", sub.RequiredID, want)
	}
	if want := ids(t, db, "Japanese Whisky")[0]; sub.SubstituteID != want {
		t.Errorf("substitute = %s, want Japanese Whisky %s", sub.SubstituteID, want)
	}
}

// With substitution off, only the exact bottle counts.
func TestSubstitutionDisabledRequiresExactBottle(t *testing.T) {
	repo, db := newRepo(t)

	owned := recipeIngredients(t, db, "Old Fashioned", "Bourbon")
	owned = append(owned, ids(t, db, "Japanese Whisky")...)

	m := find(match(t, repo, owned, false), "Old Fashioned")
	if m == nil {
		t.Fatal("Old Fashioned should still appear as one ingredient short")
	}
	if m.MissingCount != 1 {
		t.Fatalf("missing_count = %d, want 1", m.MissingCount)
	}
	if want := ids(t, db, "Bourbon")[0]; len(m.MissingIngredients) != 1 || m.MissingIngredients[0] != want {
		t.Errorf("missing = %v, want [Bourbon %s]", m.MissingIngredients, want)
	}
	if len(m.Substitutions) != 0 {
		t.Errorf("substitutions = %v, want none", m.Substitutions)
	}
}

// The bug this redesign exists to kill: a Zombie calls for three different rums,
// and owning one rum used to satisfy all three.
func TestOneRumDoesNotSatisfyThreeRumRequirements(t *testing.T) {
	repo, db := newRepo(t)

	// Everything a Zombie needs except two of its three rums.
	owned := recipeIngredients(t, db, "Zombie", "Spiced Rum", "Overproof Rum")

	if m := find(match(t, repo, owned, true), "Zombie"); m != nil {
		t.Fatalf("Zombie reported as makeable (missing_count=%d) while holding only one rum", m.MissingCount)
	}

	// Adding one more distinct rum leaves it exactly one short.
	owned = append(owned, ids(t, db, "Aged Rum")...)
	m := find(match(t, repo, owned, true), "Zombie")
	if m == nil {
		t.Fatal("Zombie should appear once two of the three rums are covered")
	}
	if m.MissingCount != 1 {
		t.Errorf("missing_count = %d, want 1", m.MissingCount)
	}
}

// Three distinct rums do cover three rum requirements.
func TestDistinctFamilyMembersCoverRepeatedRequirements(t *testing.T) {
	repo, db := newRepo(t)

	owned := recipeIngredients(t, db, "Zombie", "Spiced Rum", "Overproof Rum")
	owned = append(owned, ids(t, db, "Aged Rum", "Light Rum")...)

	m := find(match(t, repo, owned, true), "Zombie")
	if m == nil {
		t.Fatal("Zombie not matched")
	}
	// Overproof Rum is marked strict, so it can never be stood in for.
	if m.MissingCount != 1 {
		t.Fatalf("missing_count = %d, want 1 (the strict Overproof Rum)", m.MissingCount)
	}
	if want := ids(t, db, "Overproof Rum")[0]; m.MissingIngredients[0] != want {
		t.Errorf("missing = %v, want the strict Overproof Rum %s", m.MissingIngredients, want)
	}
}

// A strict requirement names the one bottle that makes the drink, so no amount
// of family stock replaces it.
func TestStrictRequirementRejectsSubstitutes(t *testing.T) {
	repo, db := newRepo(t)

	// Penicillin without its Islay float, but with two other whiskeys on hand.
	owned := recipeIngredients(t, db, "Penicillin", "Islay Scotch Whisky")
	owned = append(owned, ids(t, db, "Bourbon", "Japanese Whisky")...)

	m := find(match(t, repo, owned, true), "Penicillin")
	if m == nil {
		t.Fatal("Penicillin should appear as one ingredient short")
	}
	if m.MissingCount != 1 {
		t.Fatalf("missing_count = %d, want 1", m.MissingCount)
	}
	if want := ids(t, db, "Islay Scotch Whisky")[0]; m.MissingIngredients[0] != want {
		t.Errorf("missing = %v, want the strict Islay %s", m.MissingIngredients, want)
	}
}

// Owning the exact bottle a strict requirement names uses it up, so it cannot
// also cover a flexible requirement in the same family.
func TestStrictMatchDoesNotDoubleCount(t *testing.T) {
	repo, db := newRepo(t)

	// Penicillin needs Islay (strict) plus Blended Scotch (flexible). Holding
	// only the Islay leaves the flexible whiskey uncovered.
	owned := recipeIngredients(t, db, "Penicillin", "Blended Scotch Whisky")

	m := find(match(t, repo, owned, true), "Penicillin")
	if m == nil {
		t.Fatal("Penicillin should appear as one ingredient short")
	}
	if m.MissingCount != 1 {
		t.Errorf("missing_count = %d, want 1 — the Islay must not cover both requirements", m.MissingCount)
	}
}

func TestEmptyInventoryReturnsNoMatches(t *testing.T) {
	repo, _ := newRepo(t)

	if got := match(t, repo, nil, true); len(got) != 0 {
		t.Errorf("got %d matches for an empty shelf, want 0", len(got))
	}
}

// Every returned row is complete or one ingredient away, and owned_count always
// agrees with total minus missing.
func TestMatchInvariants(t *testing.T) {
	repo, db := newRepo(t)

	owned := ids(t, db, "London Dry Gin", "Sweet Vermouth", "Campari", "Simple Syrup",
		"Fresh Lemon Juice", "Fresh Lime Juice", "Angostura Bitters", "Bourbon")

	for _, allowSubs := range []bool{false, true} {
		for _, m := range match(t, repo, owned, allowSubs) {
			if m.MissingCount > 1 {
				t.Errorf("%s: missing_count = %d, want <= 1", m.Title, m.MissingCount)
			}
			if m.OwnedCount+m.MissingCount != m.TotalIngredients {
				t.Errorf("%s: owned %d + missing %d != total %d",
					m.Title, m.OwnedCount, m.MissingCount, m.TotalIngredients)
			}
			if len(m.MissingIngredients) != m.MissingCount {
				t.Errorf("%s: %d missing ids for missing_count %d",
					m.Title, len(m.MissingIngredients), m.MissingCount)
			}
			if !allowSubs && len(m.Substitutions) > 0 {
				t.Errorf("%s: substitutions reported while disabled", m.Title)
			}
		}
	}
}

// Pagination must be stable: strength has few distinct values, so without a
// unique tiebreaker paging drops and repeats rows.
func TestPaginationVisitsEveryRecipeExactlyOnce(t *testing.T) {
	repo, db := newRepo(t)

	var total int
	if err := db.Get(&total, `SELECT COUNT(*) FROM recipes`); err != nil {
		t.Fatalf("count recipes: %v", err)
	}

	const limit = 12
	seen := make(map[string]int)
	for offset := 0; offset < total; offset += limit {
		page, _, err := repo.GetRecipes(context.Background(), limit, offset, "desc", "")
		if err != nil {
			t.Fatalf("GetRecipes(offset=%d): %v", offset, err)
		}
		for _, r := range page {
			seen[r.ID.String()]++
		}
	}

	for id, n := range seen {
		if n > 1 {
			t.Errorf("recipe %s returned %d times across pages", id, n)
		}
	}
	if len(seen) != total {
		t.Errorf("paging returned %d distinct recipes, want all %d", len(seen), total)
	}
}

// A search for "%" is a literal percent sign, not a wildcard.
func TestSearchEscapesLikeWildcards(t *testing.T) {
	repo, _ := newRepo(t)

	_, total, err := repo.GetRecipes(context.Background(), 1, 0, "desc", "%")
	if err != nil {
		t.Fatalf("GetRecipes: %v", err)
	}
	if total != 0 {
		t.Errorf("search for %%%% matched %d recipes, want 0", total)
	}
}
