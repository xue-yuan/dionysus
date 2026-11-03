package main

import (
	"encoding/json"
	"regexp"
	"testing"
)

// These run against the embedded seed file and need no database. They exist
// because the catalogue drifted in ways nothing would have caught: a duplicate
// row for Coca-Cola that split the shelf, a "Classic" tag on all 110 recipes
// that made the filter meaningless, and 30 ingredients no recipe could match.

func load(t *testing.T) SeedData {
	t.Helper()
	var data SeedData
	if err := json.Unmarshal(embeddedSeed, &data); err != nil {
		t.Fatalf("parse seed data: %v", err)
	}
	return data
}

func TestNamesAreUnique(t *testing.T) {
	data := load(t)

	for _, c := range []struct {
		label string
		names []string
	}{
		{"ingredient", ingredientNames(data)},
		{"tag", tagNames(data)},
		{"recipe", titles(data)},
	} {
		seen := make(map[string]bool, len(c.names))
		for _, n := range c.names {
			if seen[n] {
				t.Errorf("duplicate %s %q", c.label, n)
			}
			seen[n] = true
		}
	}
}

func TestRecipesOnlyReferenceKnownRows(t *testing.T) {
	data := load(t)

	ingredients := set(ingredientNames(data))
	tags := set(tagNames(data))

	for _, r := range data.Recipes {
		if len(r.Ingredients) == 0 {
			t.Errorf("%s: no ingredients", r.Title)
		}
		for _, i := range r.Ingredients {
			if !ingredients[i.Name] {
				t.Errorf("%s: unknown ingredient %q", r.Title, i.Name)
			}
		}
		for _, tag := range r.Tags {
			if !tags[tag] {
				t.Errorf("%s: unknown tag %q", r.Title, tag)
			}
		}
	}
}

// Every ingredient must be reachable: either a recipe calls for it, or it has a
// family and can stand in for one that does. Anything else is a row the shelf
// picker offers but that can never affect a single match.
func TestEveryIngredientIsReachable(t *testing.T) {
	data := load(t)

	used := make(map[string]bool)
	for _, r := range data.Recipes {
		for _, i := range r.Ingredients {
			used[i.Name] = true
		}
	}

	families := make(map[string]int)
	for _, i := range data.Ingredients {
		if i.Family != "" {
			families[i.Family]++
		}
	}

	for _, i := range data.Ingredients {
		if used[i.Name] {
			continue
		}
		if i.Family == "" {
			t.Errorf("%s is used by no recipe and has no family, so nothing can ever match it", i.Name)
			continue
		}
		if families[i.Family] < 2 {
			t.Errorf("%s is unused and alone in family %q, so it can substitute for nothing", i.Name, i.Family)
		}
	}
}

// Classic and Modern Classic are the book's era filter, so they have to
// partition it. "Classic" used to be on all 110 recipes, which filtered nothing.
func TestEraTagsPartitionTheBook(t *testing.T) {
	data := load(t)

	for _, r := range data.Recipes {
		era := 0
		for _, tag := range r.Tags {
			if tag == "Classic" || tag == "Modern Classic" {
				era++
			}
		}
		if era != 1 {
			t.Errorf("%s carries %d era tags, want exactly 1", r.Title, era)
		}
	}
}

func TestEveryRowIsTranslatable(t *testing.T) {
	data := load(t)

	for _, i := range data.Ingredients {
		if i.TranslationKey == "" {
			t.Errorf("ingredient %q has no translation_key", i.Name)
		}
	}
	for _, tag := range data.Tags {
		if tag.TranslationKey == "" {
			t.Errorf("tag %q has no translation_key", tag.Name)
		}
	}
	for _, r := range data.Recipes {
		if r.TranslationKey == "" {
			t.Errorf("recipe %q has no translation_key", r.Title)
		}
	}
}

// A drink that lists a citrus juice cannot be rated as having no sourness, and
// one with no acid at all cannot be rated sour.
func TestSournessMatchesTheIngredients(t *testing.T) {
	data := load(t)

	acids := set([]string{
		"Fresh Lemon Juice", "Fresh Lime Juice", "Fresh Grapefruit Juice", "Olive Brine",
	})

	for _, r := range data.Recipes {
		acidic := false
		for _, i := range r.Ingredients {
			if acids[i.Name] {
				acidic = true
			}
		}
		if acidic && r.Sourness == 0 {
			t.Errorf("%s lists a citrus juice but has sourness 0", r.Title)
		}
		if !acidic && r.Sourness >= 3 {
			t.Errorf("%s has sourness %d with no acid ingredient", r.Title, r.Sourness)
		}
	}
}

func set(values []string) map[string]bool {
	m := make(map[string]bool, len(values))
	for _, v := range values {
		m[v] = true
	}
	return m
}

// Recipe URLs are the key with "recipe." dropped and underscores written as
// hyphens, and the server reverses that to find the row. A hyphen inside a key,
// or two recipes sharing one, would make a link open the wrong drink or none.
func TestRecipeKeysMakeUsableLinks(t *testing.T) {
	data := load(t)

	pattern := regexp.MustCompile(`^recipe\.[a-z0-9]+(_[a-z0-9]+)*$`)
	seen := make(map[string]string, len(data.Recipes))
	for _, r := range data.Recipes {
		if !pattern.MatchString(r.TranslationKey) {
			t.Errorf("%s: translation_key %q cannot be turned into a link", r.Title, r.TranslationKey)
		}
		if other, dup := seen[r.TranslationKey]; dup {
			t.Errorf("%s and %s share translation_key %q", r.Title, other, r.TranslationKey)
		}
		seen[r.TranslationKey] = r.Title
	}
}

// A tag no recipe carries is a box in the shelf editor that does nothing when
// ticked. Four of the twelve tools used to be exactly that.
func TestEveryTagIsUsed(t *testing.T) {
	data := load(t)

	used := make(map[string]bool)
	for _, r := range data.Recipes {
		for _, tag := range r.Tags {
			used[tag] = true
		}
	}
	for _, tag := range data.Tags {
		if !used[tag.Name] {
			t.Errorf("tag %q is carried by no recipe", tag.Name)
		}
	}
}

// The client builds one shelf-editor section per type and filters each as a
// group, so a misspelt type would silently become a section of its own.
func TestTagTypesAreKnown(t *testing.T) {
	data := load(t)

	known := set([]string{"Palate", "Style", "Equipment"})
	for _, tag := range data.Tags {
		if !known[tag.Type] {
			t.Errorf("tag %q has unknown type %q", tag.Name, tag.Type)
		}
	}
}

// Shared shelf links carry each ingredient's key without "ingredient.", joined
// by hyphens. A hyphen inside a key would split one bottle into two unknowns.
func TestIngredientKeysMakeUsableLinks(t *testing.T) {
	data := load(t)

	pattern := regexp.MustCompile(`^ingredient\.[a-z0-9]+(_[a-z0-9]+)*$`)
	for _, i := range data.Ingredients {
		if !pattern.MatchString(i.TranslationKey) {
			t.Errorf("%s: translation_key %q cannot be put in a shelf link", i.Name, i.TranslationKey)
		}
	}
}

// Base drives the archive's spirit filter, family drives substitution. They
// must agree wherever both are set, or "gin drinks" and "any gin will do" would
// disagree about what a gin is; and base belongs to spirits alone.
func TestBaseAgreesWithFamily(t *testing.T) {
	data := load(t)

	bases := set([]string{"Whiskey", "Gin", "Rum", "Agave", "Brandy", "Vodka"})
	for _, i := range data.Ingredients {
		if i.Base == "" {
			if i.Category == "Spirit" && bases[i.Family] {
				t.Errorf("%s is a %s but has no base", i.Name, i.Family)
			}
			continue
		}
		if !bases[i.Base] {
			t.Errorf("%s has unknown base %q", i.Name, i.Base)
		}
		if i.Category != "Spirit" {
			t.Errorf("%s is a %s, so it cannot be a drink's base", i.Name, i.Category)
		}
		if i.Family != "" && i.Family != i.Base {
			t.Errorf("%s has family %q but base %q", i.Name, i.Family, i.Base)
		}
	}
}
