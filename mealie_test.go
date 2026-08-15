package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeMealie spins up an httptest server that emulates the subset of the
// Mealie API the backend consumes.
func fakeMealie(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/recipes", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(pagination{
			Page:       1,
			PerPage:    100,
			Total:      2,
			TotalPages: 1,
			Items: []recipeSummary{
				{ID: "r1", Slug: "carbonara", Name: "Carbonara", Image: "min-500x500.webp",
					RecipeServings: "4", TotalTime: "PT30M", RecipeCategory: []struct {
						Name string `json:"name"`
					}{{Name: "Pasta"}}},
				{ID: "r2", Slug: "pancakes", Name: "Pancakes", Image: "min-500x500.webp",
					RecipeServings: "2", TotalTime: "PT15M"},
			},
		})
	})
	mux.HandleFunc("GET /api/recipes/{slug}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(recipeDetail{
			recipeSummary: recipeSummary{
				ID: "r1", Slug: "carbonara", Name: "Carbonara", Image: "min-500x500.webp",
				Description:    "A classic.",
				RecipeServings: "4",
				TotalTime:      "PT30M",
				RecipeCategory: []struct {
					Name string `json:"name"`
				}{{Name: "Pasta"}},
			},
			RecipeIngredient: []struct {
				Quantity json.Number `json:"quantity"`
				Unit     struct {
					Name string `json:"name"`
				} `json:"unit"`
				Food struct {
					Name string `json:"name"`
				} `json:"food"`
				Note    string `json:"note"`
				Display string `json:"display"`
			}{
				{Quantity: "100", Unit: struct {
					Name string `json:"name"`
				}{Name: "g"}, Food: struct {
					Name string `json:"name"`
				}{Name: "Pancetta"}, Display: "100 g Pancetta"},
			},
			RecipeInstructions: []struct {
				Text string `json:"text"`
			}{{Text: "Fry the pancetta."}},
		})
	})
	mux.HandleFunc("GET /api/media/recipes/{id}/images/{file}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/webp")
		w.Write([]byte("fake-image-bytes"))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestRandomRecipe(t *testing.T) {
	srv := fakeMealie(t)
	c := newMealieClient(srv.URL, "test-key")

	recipe, err := c.randomRecipe(context.Background())
	if err != nil {
		t.Fatalf("randomRecipe: %v", err)
	}
	if recipe.Name == "" {
		t.Error("expected a recipe name")
	}
	if !strings.Contains(recipe.Image, "/api/trmnl/recipe-image?") ||
		!strings.Contains(recipe.Image, "id=r1") ||
		!strings.Contains(recipe.Image, "file=min-500x500.webp") {
		t.Errorf("expected proxied image path, got %q", recipe.Image)
	}
	if len(recipe.Ingredients) != 1 || recipe.Ingredients[0] != "100 g Pancetta" {
		t.Errorf("unexpected ingredients: %v", recipe.Ingredients)
	}
	if len(recipe.Instructions) != 1 {
		t.Errorf("expected 1 instruction, got %d", len(recipe.Instructions))
	}
	if recipe.URL == "" {
		t.Error("expected a recipe URL")
	}
}

func TestRecipeOfTheDayHandler(t *testing.T) {
	srv := fakeMealie(t)
	c := newMealieClient(srv.URL, "test-key")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/trmnl/recipe-of-the-day", nil)

	c.handleRecipeOfTheDay(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var out recipeOfTheDay
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if out.Slug == "" {
		t.Error("expected a slug in the response")
	}
}

func TestRecipeImageHandler(t *testing.T) {
	srv := fakeMealie(t)
	c := newMealieClient(srv.URL, "test-key")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/trmnl/recipe-image?id=r1&file=min-500x500.webp", nil)
	c.handleRecipeImage(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if got := rec.Body.String(); got != "fake-image-bytes" {
		t.Errorf("unexpected body %q", got)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "image/webp" {
		t.Errorf("unexpected content type %q", ct)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/trmnl/recipe-image", nil)
	c.handleRecipeImage(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing params, got %d", rec.Code)
	}
}

func TestFormatIngredient(t *testing.T) {
	cases := []struct {
		name string
		qty  json.Number
		unit string
		food string
		note string
		want string
	}{
		{"full", "100", "g", "Pancetta", "", "100 g Pancetta"},
		{"food only", "", "", "Salt", "", "Salt"},
		{"with note", "2", "cloves", "Garlic", "minced", "2 cloves Garlic — minced"},
		{"empty", "", "", "", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := formatIngredient(tc.qty, tc.unit, tc.food, tc.note); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
