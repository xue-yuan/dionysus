package handler

import (
	"database/sql"
	"errors"
	"log"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/xue-yuan/dionysus/internal/repository"
)

const (
	defaultPageSize = 12
	maxPageSize     = 100
	// A shelf larger than this is not a real inventory.
	maxOwnedIngredients = 500
	maxSearchLength     = 100
)

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

func (h *Handler) GetIngredients(c fiber.Ctx) error {
	ingredients, err := h.Repo.GetIngredients(c.Context())
	if err != nil {
		return fail(c, fiber.StatusInternalServerError, "failed to fetch ingredients", err)
	}
	return c.JSON(ingredients)
}

func (h *Handler) GetTags(c fiber.Ctx) error {
	tags, err := h.Repo.GetTags(c.Context())
	if err != nil {
		return fail(c, fiber.StatusInternalServerError, "failed to fetch tags", err)
	}
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

	search := c.Query("search", "")
	if len(search) > maxSearchLength {
		search = search[:maxSearchLength]
	}

	// "sort" only picks the direction; the ordering key is always strength.
	sort := c.Query("sort", "desc")
	if sort != "asc" && sort != "desc" {
		sort = "desc"
	}

	recipes, total, err := h.Repo.GetRecipes(c.Context(), limit, (page-1)*limit, sort, search)
	if err != nil {
		return fail(c, fiber.StatusInternalServerError, "failed to fetch recipes", err)
	}
	return c.JSON(fiber.Map{"items": recipes, "total": total})
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

func (h *Handler) GetRecipe(c fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return badRequest(c, "invalid recipe id")
	}

	recipe, err := h.Repo.GetRecipeWithIngredients(c.Context(), id)
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
	TagIDs             []string `json:"tag_ids"`
	AllowSubstitutes   bool     `json:"allow_substitutes"`
}

func (h *Handler) MatchCocktails(c fiber.Ctx) error {
	var req matchRequest
	if err := c.Bind().Body(&req); err != nil {
		return badRequest(c, "invalid request body")
	}

	if len(req.OwnedIngredientIDs) > maxOwnedIngredients || len(req.TagIDs) > maxOwnedIngredients {
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
	if req.MinStrength < 0 || req.MinStrength > 5 {
		return badRequest(c, "min_strength must be between 0 and 5")
	}

	matches, err := h.Repo.FindMatchingRecipes(c.Context(), repository.MatchQuery{
		OwnedIngredientIDs: owned,
		MinStrength:        req.MinStrength,
		TagIDs:             tagIDs,
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
