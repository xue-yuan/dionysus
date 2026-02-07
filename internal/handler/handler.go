package handler

import (
	"log"
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/xue-yuan/dionysus/internal/model"
	"github.com/xue-yuan/dionysus/internal/repository"
)

type Handler struct {
	Repo *repository.Repository
}

func New(repo *repository.Repository) *Handler {
	return &Handler{Repo: repo}
}

func (h *Handler) GetIngredients(c fiber.Ctx) error {
	ingredients, err := h.Repo.GetIngredients()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(ingredients)
}

func (h *Handler) GetTags(c fiber.Ctx) error {
	tags, err := h.Repo.GetTags()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(tags)
}

type CreateRecipeRequest struct {
	Title       string                   `json:"title"`
	Description string                   `json:"description"`
	Glassware   string                   `json:"glassware"`
	Method      string                   `json:"method"`
	Steps       string                   `json:"steps"`
	ImageURL    string                   `json:"image_url"`
	Ingredients []model.RecipeIngredient `json:"ingredients"`
	Sweetness   int                      `json:"sweetness"`
	Sourness    int                      `json:"sourness"`
	Strength    int                      `json:"strength"`
	Tags        []model.Tag              `json:"tags"`
}

func (h *Handler) CreateRecipe(c fiber.Ctx) error {
	var req CreateRecipeRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if req.Title == "" || req.Method == "" || req.Steps == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Missing required fields"})
	}

	recipe := model.Recipe{
		Title:       req.Title,
		Description: req.Description,
		Glassware:   req.Glassware,
		Method:      req.Method,
		Steps:       req.Steps,
		ImageURL:    req.ImageURL,
		Sweetness:   req.Sweetness,
		Sourness:    req.Sourness,
		Strength:    req.Strength,
		Tags:        req.Tags,
	}

	if err := h.Repo.CreateRecipe(&recipe, req.Ingredients); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create recipe: " + err.Error()})
	}

	return c.Status(fiber.StatusCreated).JSON(recipe)
}

func (h *Handler) GetRecipes(c fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "12"))
	sort := c.Query("sort", "desc")

	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 12
	}
	offset := (page - 1) * limit

	recipes, total, err := h.Repo.GetRecipes(limit, offset, sort)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch recipes"})
	}

	return c.JSON(fiber.Map{
		"items": recipes,
		"total": total,
	})
}

func (h *Handler) GetRecipe(c fiber.Ctx) error {
	idParam := c.Params("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid UUID"})
	}

	recipe, err := h.Repo.GetRecipeWithIngredients(id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Recipe not found"})
	}

	return c.JSON(recipe)
}

func (h *Handler) MatchCocktails(c fiber.Ctx) error {
	type MatchRequest struct {
		OwnedIngredientIDs []string `json:"owned_ingredient_ids"`
		MinStrength        int      `json:"min_strength"`
		TagIDs             []string `json:"tag_ids"`
	}

	var req MatchRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	matches, err := h.Repo.FindMatchingRecipes(req.OwnedIngredientIDs, req.MinStrength, req.TagIDs)
	if err != nil {
		log.Printf("MatchCocktails Error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to match recipes",
		})
	}

	return c.JSON(matches)
}

func (h *Handler) Routes(app *fiber.App) {
	api := app.Group("/api")

	api.Get("/health", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	api.Get("/ingredients", h.GetIngredients)
	api.Get("/tags", h.GetTags)
	api.Get("/recipes", h.GetRecipes)
	api.Get("/recipes/:id", h.GetRecipe)
	api.Post("/recipes", h.CreateRecipe)
	api.Post("/match-cocktails", h.MatchCocktails)
}
