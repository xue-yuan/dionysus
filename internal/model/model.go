package model

import (
	"time"

	"github.com/google/uuid"
)

type Ingredient struct {
	ID             uuid.UUID `db:"id" json:"id"`
	Name           string    `db:"name" json:"name"`
	Category       string    `db:"category" json:"category"`
	TranslationKey string    `db:"translation_key" json:"translation_key"`
}

type Recipe struct {
	ID             uuid.UUID `db:"id" json:"id"`
	Title          string    `db:"title" json:"title"`
	Description    string    `db:"description" json:"description"`
	Glassware      string    `db:"glassware" json:"glassware"`
	Method         string    `db:"method" json:"method"`
	Steps          string    `db:"steps" json:"steps"`
	ImageURL       string    `db:"image_url" json:"image_url"`
	Sweetness      int       `db:"sweetness" json:"sweetness"`
	Sourness       int       `db:"sourness" json:"sourness"`
	Strength       int       `db:"strength" json:"strength"`
	TranslationKey string    `db:"translation_key" json:"translation_key"`
	CreatedAt      time.Time `db:"created_at" json:"created_at"`

	Ingredients      []RecipeIngredientDetail `json:"ingredients,omitempty"`
	Tags             []Tag                    `json:"tags,omitempty"`
	TotalIngredients int                      `db:"total_ingredients" json:"total_ingredients"`
}

type Tag struct {
	ID   uuid.UUID `db:"id" json:"id"`
	Name string    `db:"name" json:"name"`
	Type string    `db:"type" json:"type"`
}

type RecipeIngredient struct {
	RecipeID     uuid.UUID `db:"recipe_id" json:"recipe_id"`
	IngredientID uuid.UUID `db:"ingredient_id" json:"ingredient_id"`
	Amount       string    `db:"amount" json:"amount"`
	Unit         string    `db:"unit" json:"unit"`
}

type RecipeIngredientDetail struct {
	IngredientID   uuid.UUID `db:"ingredient_id" json:"ingredient_id"`
	Name           string    `db:"name" json:"name"`
	Category       string    `db:"category" json:"category"`
	TranslationKey string    `db:"translation_key" json:"translation_key"`
	Amount         string    `db:"amount" json:"amount"`
	Unit           string    `db:"unit" json:"unit"`
}

type RecipeMatch struct {
	ID                 string   `json:"id" db:"id"`
	Title              string   `json:"title" db:"title"`
	Description        string   `json:"description" db:"description"`
	ImageUrl           string   `json:"image_url" db:"image_url"`
	TotalIngredients   int      `json:"total_ingredients" db:"total_ingredients"`
	OwnedCount         int      `json:"owned_count" db:"owned_count"`
	MissingCount       int      `json:"missing_count" db:"missing_count"`
	MissingIngredients []string `json:"missing_ingredients,omitempty"`
	Sweetness          int      `json:"sweetness" db:"sweetness"`
	Sourness           int      `json:"sourness" db:"sourness"`
	Strength           int      `json:"strength" db:"strength"`
	Tags               []Tag    `json:"tags,omitempty"`
	Glassware          string   `json:"glassware" db:"glassware"`
	Method             string   `json:"method" db:"method"`
	TranslationKey     string   `json:"translation_key" db:"translation_key"`
}
