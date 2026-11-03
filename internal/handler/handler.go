// Package handler
package handler

import (
	"database/sql"
	"errors"
	"log"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/xue-yuan/dionysus/internal/model"
	"github.com/xue-yuan/dionysus/internal/repository"
)

const (
	defaultPageSize = 12
	maxPageSize     = 100
	// A shelf larger than this is not a real inventory.
	maxOwnedIngredients = 500
	maxSearchLength     = 100
	// The client sends the translation keys whose localised text matched the
	// search. More than this many is not a search, it is a scrape.
	maxSearchKeys = 200
	// The whole catalogue has 40 tags; a filter naming more is not a filter.
	maxFilterTags = 50
)

// translationKeyPattern is what the seed file generates: a dotted lowercase
// slug. Anything else is not a key this archive knows.
var translationKeyPattern = regexp.MustCompile(`^[a-z0-9_.]{1,80}$`)

// basePattern is a base spirit as the seed file spells it ("Whiskey", "Agave").
var basePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z ]{0,39}$`)

// recipeSlugPattern is what the client puts in a recipe URL: the recipe's
// translation key without its "recipe." prefix, with underscores written as
// hyphens. Recipe keys never contain a hyphen (the seed tests hold that), so
// the mapping reverses exactly.
var recipeSlugPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// recipeKeyFromSlug turns "whiskey-sour" back into "recipe.whiskey_sour".
func recipeKeyFromSlug(slug string) (string, bool) {
	if len(slug) > 80 || !recipeSlugPattern.MatchString(slug) {
		return "", false
	}
	return "recipe." + strings.ReplaceAll(slug, "-", "_"), true
}

type Handler struct {
	Repo *repository.Repository
}

func New(repo *repository.Repository) *Handler {
	return &Handler{Repo: repo}
}

// fail logs the real cause and returns a generic message. Driver errors name
// tables, columns and constraints, so they must not reach the client.
func fail(c fiber.Ctx, status int, publicMsg string, err error) error {
	if err != nil {
		log.Printf("%s %s: %v", c.Method(), c.Path(), err)
	}
	return c.Status(status).JSON(fiber.Map{"error": publicMsg})
}

func badRequest(c fiber.Ctx, msg string) error {
	return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": msg})
}

// parseUUIDs rejects the whole request if any id is malformed, so bad input is a
// 400 rather than a driver cast failure surfacing as a 500.
func parseUUIDs(ids []string) ([]string, error) {
	out := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, raw := range ids {
		id, err := uuid.Parse(strings.TrimSpace(raw))
		if err != nil {
			return nil, err
		}
		s := id.String()
		if _, dup := seen[s]; dup {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out, nil
}

func (h *Handler) Health(c fiber.Ctx) error {
	if err := h.Repo.DB.PingContext(c.Context()); err != nil {
		return fail(c, fiber.StatusServiceUnavailable, "database unavailable", err)
	}
	return c.JSON(fiber.Map{"status": "ok"})
}

// cacheable marks a response the browser may reuse. The archive has no write
// API, so the catalogue only changes when the seeder runs — an hour of staleness
// costs nothing and saves refetching 25 KB on every page load.
func cacheable(c fiber.Ctx) {
	c.Set(fiber.HeaderCacheControl, "public, max-age=3600")
}

func (h *Handler) GetIngredients(c fiber.Ctx) error {
	ingredients, err := h.Repo.GetIngredients(c.Context())
	if err != nil {
		return fail(c, fiber.StatusInternalServerError, "failed to fetch ingredients", err)
	}
	cacheable(c)
	return c.JSON(ingredients)
}

func (h *Handler) GetTags(c fiber.Ctx) error {
	tags, err := h.Repo.GetTags(c.Context())
	if err != nil {
		return fail(c, fiber.StatusInternalServerError, "failed to fetch tags", err)
	}
	cacheable(c)
	return c.JSON(tags)
}

func (h *Handler) GetRecipes(c fiber.Ctx) error {
	page, err := strconv.Atoi(c.Query("page", "1"))
	if err != nil || page < 1 {
		page = 1
	}
	limit, err := strconv.Atoi(c.Query("limit", strconv.Itoa(defaultPageSize)))
	if err != nil || limit < 1 {
		limit = defaultPageSize
	}
	if limit > maxPageSize {
		limit = maxPageSize
	}

	// Truncated by rune, not by byte: "search" arrives as UTF-8, and cutting a
	// multibyte character in half produces a string PostgreSQL rejects outright
	// with "invalid byte sequence for encoding UTF8" — a 500 for what is really
	// just a long query.
	search := truncateRunes(c.Query("search", ""), maxSearchLength)

	// "sort" only picks the direction; the ordering key is always strength.
	sort := c.Query("sort", "desc")
	if sort != "asc" && sort != "desc" {
		sort = "desc"
	}

	keys, err := parseTranslationKeys(c.Query("keys", ""))
	if err != nil {
		return badRequest(c, err.Error())
	}

	var tagIDs []string
	if raw := c.Query("tags", ""); raw != "" {
		parts := strings.Split(raw, ",")
		if len(parts) > maxFilterTags {
			return badRequest(c, "too many tags requested")
		}
		if tagIDs, err = parseUUIDs(parts); err != nil {
			return badRequest(c, "tags must be uuids")
		}
	}

	base := c.Query("base", "")
	if base != "" && !basePattern.MatchString(base) {
		return badRequest(c, "invalid base")
	}

	recipes, total, err := h.Repo.GetRecipes(c.Context(), repository.RecipeQuery{
		Limit:           limit,
		Offset:          (page - 1) * limit,
		SortDirection:   sort,
		Search:          search,
		TranslationKeys: keys,
		TagIDs:          tagIDs,
		Base:            base,
	})
	if err != nil {
		return fail(c, fiber.StatusInternalServerError, "failed to fetch recipes", err)
	}
	return c.JSON(fiber.Map{"items": recipes, "total": total})
}

func truncateRunes(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	return string([]rune(s)[:max])
}

// parseTranslationKeys reads the "keys" parameter, which carries the
// translation keys whose localised text matched the user's search. The server
// stores only English text, so this is what lets a zh-TW reader find a recipe
// by its Chinese name.
func parseTranslationKeys(raw string) ([]string, error) {
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	if len(parts) > maxSearchKeys {
		return nil, errors.New("too many keys requested")
	}
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		key := strings.TrimSpace(part)
		if key == "" {
			continue
		}
		if !translationKeyPattern.MatchString(key) {
			return nil, errors.New("keys must be translation keys")
		}
		out = append(out, key)
	}
	return out, nil
}

// GetRecipesByIDs backs the saved-recipes page, which previously downloaded the
// entire archive just to keep the handful of rows the user had favourited.
func (h *Handler) GetRecipesByIDs(c fiber.Ctx) error {
	raw := c.Query("ids", "")
	if raw == "" {
		return c.JSON(fiber.Map{"items": []any{}, "total": 0})
	}

	parts := strings.Split(raw, ",")
	if len(parts) > maxPageSize {
		return badRequest(c, "too many ids requested")
	}
	ids, err := parseUUIDs(parts)
	if err != nil {
		return badRequest(c, "ids must be uuids")
	}

	recipes, err := h.Repo.GetRecipesByIDs(c.Context(), ids)
	if err != nil {
		return fail(c, fiber.StatusInternalServerError, "failed to fetch recipes", err)
	}
	return c.JSON(fiber.Map{"items": recipes, "total": len(recipes)})
}

// GetRecipe accepts either a UUID or a slug. The client links to recipes by
// slug, because those stay the same in every database the seed file fills.
func (h *Handler) GetRecipe(c fiber.Ctx) error {
	ref := c.Params("id")

	var recipe *model.Recipe
	var err error
	if id, parseErr := uuid.Parse(ref); parseErr == nil {
		recipe, err = h.Repo.GetRecipeWithIngredients(c.Context(), id)
	} else if key, ok := recipeKeyFromSlug(ref); ok {
		recipe, err = h.Repo.GetRecipeByKey(c.Context(), key)
	} else {
		return badRequest(c, "invalid recipe id")
	}
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "recipe not found"})
		}
		return fail(c, fiber.StatusInternalServerError, "failed to fetch recipe", err)
	}
	return c.JSON(recipe)
}

type matchRequest struct {
	OwnedIngredientIDs []string `json:"owned_ingredient_ids"`
	MinStrength        int      `json:"min_strength"`
	MaxStrength        int      `json:"max_strength"`
	TagIDs             []string `json:"tag_ids"`
	// Tools the user does not have. Recipes tagged with any of them are dropped.
	ExcludeTagIDs    []string `json:"exclude_tag_ids"`
	AllowSubstitutes bool     `json:"allow_substitutes"`
}

func (h *Handler) MatchCocktails(c fiber.Ctx) error {
	var req matchRequest
	if err := c.Bind().Body(&req); err != nil {
		return badRequest(c, "invalid request body")
	}

	if len(req.OwnedIngredientIDs) > maxOwnedIngredients ||
		len(req.TagIDs) > maxOwnedIngredients ||
		len(req.ExcludeTagIDs) > maxOwnedIngredients {
		return badRequest(c, "too many ingredients or tags")
	}
	owned, err := parseUUIDs(req.OwnedIngredientIDs)
	if err != nil {
		return badRequest(c, "owned_ingredient_ids must be uuids")
	}
	tagIDs, err := parseUUIDs(req.TagIDs)
	if err != nil {
		return badRequest(c, "tag_ids must be uuids")
	}
	excludeTagIDs, err := parseUUIDs(req.ExcludeTagIDs)
	if err != nil {
		return badRequest(c, "exclude_tag_ids must be uuids")
	}
	if req.MinStrength < 0 || req.MinStrength > 5 {
		return badRequest(c, "min_strength must be between 0 and 5")
	}
	if req.MaxStrength < 0 || req.MaxStrength > 5 {
		return badRequest(c, "max_strength must be between 0 and 5")
	}

	matches, err := h.Repo.FindMatchingRecipes(c.Context(), repository.MatchQuery{
		OwnedIngredientIDs: owned,
		MinStrength:        req.MinStrength,
		MaxStrength:        req.MaxStrength,
		TagIDs:             tagIDs,
		ExcludeTagIDs:      excludeTagIDs,
		AllowSubstitutes:   req.AllowSubstitutes,
	})
	if err != nil {
		return fail(c, fiber.StatusInternalServerError, "failed to match recipes", err)
	}
	return c.JSON(matches)
}

func (h *Handler) Routes(app *fiber.App) {
	api := app.Group("/api")

	api.Get("/health", h.Health)
	api.Get("/ingredients", h.GetIngredients)
	api.Get("/tags", h.GetTags)
	api.Get("/recipes", h.GetRecipes)
	api.Get("/recipes/by-ids", h.GetRecipesByIDs)
	api.Get("/recipes/:id", h.GetRecipe)
	api.Post("/match-cocktails", h.MatchCocktails)
}
