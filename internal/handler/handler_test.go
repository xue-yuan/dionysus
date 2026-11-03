package handler

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// The search parameter arrives as UTF-8. Truncating it by byte splits a
// multibyte character in half, and PostgreSQL rejects the result outright, so a
// long Chinese query used to come back as a 500.
func TestTruncateRunesKeepsValidUTF8(t *testing.T) {
	cases := []struct {
		name  string
		input string
		max   int
		want  string
	}{
		{"under the limit", "martini", 100, "martini"},
		{"exactly the limit", "abc", 3, "abc"},
		{"ascii truncated", "abcdef", 3, "abc"},
		{"multibyte truncated", strings.Repeat("馬", 40), 10, strings.Repeat("馬", 10)},
		{"multibyte under limit", "馬丁尼", 100, "馬丁尼"},
		{"empty", "", 100, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := truncateRunes(tc.input, tc.max)
			if got != tc.want {
				t.Errorf("truncateRunes(%q, %d) = %q, want %q", tc.input, tc.max, got, tc.want)
			}
			if !utf8.ValidString(got) {
				t.Errorf("truncateRunes(%q, %d) produced invalid UTF-8", tc.input, tc.max)
			}
		})
	}
}

func TestParseTranslationKeys(t *testing.T) {
	t.Run("accepts seed-shaped keys", func(t *testing.T) {
		got, err := parseTranslationKeys("recipe.20th_century, recipe.dry_martini")
		if err != nil {
			t.Fatalf("parseTranslationKeys: %v", err)
		}
		want := []string{"recipe.20th_century", "recipe.dry_martini"}
		if len(got) != len(want) {
			t.Fatalf("got %v, want %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("got %v, want %v", got, want)
			}
		}
	})

	t.Run("empty is no filter", func(t *testing.T) {
		got, err := parseTranslationKeys("")
		if err != nil || got != nil {
			t.Fatalf("got (%v, %v), want (nil, nil)", got, err)
		}
	})

	t.Run("rejects anything that is not a key", func(t *testing.T) {
		for _, raw := range []string{"Recipe.Martini", "recipe martini", "'; DROP TABLE recipes--", strings.Repeat("a", 81)} {
			if _, err := parseTranslationKeys(raw); err == nil {
				t.Errorf("parseTranslationKeys(%q) accepted a non-key", raw)
			}
		}
	})

	t.Run("rejects a scrape", func(t *testing.T) {
		raw := strings.TrimSuffix(strings.Repeat("recipe.a,", maxSearchKeys+1), ",")
		if _, err := parseTranslationKeys(raw); err == nil {
			t.Error("parseTranslationKeys accepted more than maxSearchKeys")
		}
	})
}

func TestRecipeKeyFromSlug(t *testing.T) {
	for slug, want := range map[string]string{
		"negroni":      "recipe.negroni",
		"whiskey-sour": "recipe.whiskey_sour",
		"20th-century": "recipe.20th_century",
	} {
		got, ok := recipeKeyFromSlug(slug)
		if !ok || got != want {
			t.Errorf("recipeKeyFromSlug(%q) = (%q, %v), want (%q, true)", slug, got, ok, want)
		}
	}

	for _, slug := range []string{"", "Negroni", "whiskey_sour", "-negroni", "negroni-", "a--b", "../etc", strings.Repeat("a", 81)} {
		if key, ok := recipeKeyFromSlug(slug); ok {
			t.Errorf("recipeKeyFromSlug(%q) accepted a non-slug as %q", slug, key)
		}
	}
}
