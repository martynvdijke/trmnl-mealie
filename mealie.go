package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// mealieClient talks to a Mealie instance. Authentication is via an API key
// sent as a Bearer token (created in Mealie under User Settings -> API Tokens).
type mealieClient struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

func newMealieClient(baseURL, apiKey string) *mealieClient {
	return &mealieClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *mealieClient) do(ctx context.Context, method, path string, query url.Values) (*http.Response, error) {
	u := c.baseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "trmnl-mealie/"+Version)
	return c.http.Do(req)
}

// recipeSummary mirrors the subset of Mealie's RecipeSummary we consume.
type recipeSummary struct {
	ID                  string   `json:"id"`
	Slug                string   `json:"slug"`
	Name                string   `json:"name"`
	Image               string   `json:"image"`
	Description         string   `json:"description"`
	RecipeServings      string   `json:"recipeServings"`
	RecipeYieldQuantity string   `json:"recipeYieldQuantity"`
	RecipeYield         string   `json:"recipeYield"`
	TotalTime           string   `json:"totalTime"`
	PrepTime            string   `json:"prepTime"`
	CookTime            string   `json:"cookTime"`
	Rating              *float64 `json:"rating"`
	RecipeCategory      []struct {
		Name string `json:"name"`
	} `json:"recipeCategory"`
	Tags []struct {
		Name string `json:"name"`
	} `json:"tags"`
}

// pagination mirrors Mealie's PaginationBase[RecipeSummary].
type pagination struct {
	Page       int             `json:"page"`
	PerPage    int             `json:"per_page"`
	Total      int             `json:"total"`
	TotalPages int             `json:"total_pages"`
	Items      []recipeSummary `json:"items"`
	Next       string          `json:"next"`
	Previous   string          `json:"previous"`
}

// recipeDetail mirrors the Mealie recipe detail output.
type recipeDetail struct {
	recipeSummary
	RecipeIngredient []struct {
		Quantity json.Number `json:"quantity"`
		Unit     struct {
			Name string `json:"name"`
		} `json:"unit"`
		Food struct {
			Name string `json:"name"`
		} `json:"food"`
		Note    string `json:"note"`
		Display string `json:"display"`
	} `json:"recipeIngredient"`
	RecipeInstructions []struct {
		Text string `json:"text"`
	} `json:"recipeInstructions"`
}

func (c *mealieClient) listRecipes(ctx context.Context, page, perPage int) (*pagination, error) {
	q := url.Values{}
	q.Set("page", fmt.Sprintf("%d", page))
	q.Set("perPage", fmt.Sprintf("%d", perPage))
	resp, err := c.do(ctx, http.MethodGet, "/api/recipes", q)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Mealie /api/recipes returned %s", resp.Status)
	}
	var p pagination
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		return nil, err
	}
	return &p, nil
}

func (c *mealieClient) getRecipe(ctx context.Context, slug string) (*recipeDetail, error) {
	resp, err := c.do(ctx, http.MethodGet, "/api/recipes/"+url.PathEscape(slug), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Mealie /api/recipes/%s returned %s", slug, resp.Status)
	}
	var d recipeDetail
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		return nil, err
	}
	return &d, nil
}

// recipeOfTheDay is the JSON shape served to the TRMNL plugin.
type recipeOfTheDay struct {
	Name         string   `json:"name"`
	Slug         string   `json:"slug"`
	Description  string   `json:"description"`
	Image        string   `json:"image"`
	Servings     string   `json:"servings"`
	Yield        string   `json:"yield"`
	PrepTime     string   `json:"prep_time"`
	CookTime     string   `json:"cook_time"`
	TotalTime    string   `json:"total_time"`
	Rating       *float64 `json:"rating"`
	Categories   []string `json:"categories"`
	Tags         []string `json:"tags"`
	Ingredients  []string `json:"ingredients"`
	Instructions []string `json:"instructions"`
	URL          string   `json:"url"`
}

// randomRecipe picks a random recipe from the library and returns its detail
// shaped for the TRMNL plugin. Mealie has no server-side random endpoint, so
// we pick a random page and a random item from it, then fetch the detail.
func (c *mealieClient) randomRecipe(ctx context.Context) (*recipeOfTheDay, error) {
	const perPage = 100

	first, err := c.listRecipes(ctx, 1, perPage)
	if err != nil {
		return nil, err
	}
	if first.Total == 0 || len(first.Items) == 0 {
		return nil, fmt.Errorf("no recipes found in Mealie instance")
	}

	page := first
	if first.TotalPages > 1 {
		pageNum := rand.IntN(first.TotalPages) + 1
		if pageNum != 1 {
			p, err := c.listRecipes(ctx, pageNum, perPage)
			if err != nil {
				return nil, err
			}
			page = p
		}
	}
	if len(page.Items) == 0 {
		return nil, fmt.Errorf("no recipes on page %d", page.Page)
	}
	summary := page.Items[rand.IntN(len(page.Items))]

	detail, err := c.getRecipe(ctx, summary.Slug)
	if err != nil {
		return nil, err
	}

	out := &recipeOfTheDay{
		Name:        detail.Name,
		Slug:        detail.Slug,
		Description: detail.Description,
		Servings:    detail.RecipeServings,
		Yield:       detail.RecipeYield,
		PrepTime:    detail.PrepTime,
		CookTime:    detail.CookTime,
		TotalTime:   detail.TotalTime,
		Rating:      detail.Rating,
		URL:         c.baseURL + "/recipe/" + url.PathEscape(detail.Slug),
	}
	if detail.Image != "" {
		q := url.Values{}
		q.Set("id", detail.ID)
		q.Set("file", detail.Image)
		out.Image = "/api/trmnl/recipe-image?" + q.Encode()
	}
	for _, cat := range detail.RecipeCategory {
		out.Categories = append(out.Categories, cat.Name)
	}
	for _, tag := range detail.Tags {
		out.Tags = append(out.Tags, tag.Name)
	}
	for _, ing := range detail.RecipeIngredient {
		if ing.Display != "" {
			out.Ingredients = append(out.Ingredients, ing.Display)
			continue
		}
		out.Ingredients = append(out.Ingredients, formatIngredient(ing.Quantity, ing.Unit.Name, ing.Food.Name, ing.Note))
	}
	for _, ins := range detail.RecipeInstructions {
		if strings.TrimSpace(ins.Text) != "" {
			out.Instructions = append(out.Instructions, ins.Text)
		}
	}
	return out, nil
}

func formatIngredient(qty json.Number, unit, food, note string) string {
	var b strings.Builder
	if s := qty.String(); s != "" && s != "0" {
		b.WriteString(s)
		if unit != "" {
			b.WriteString(" " + unit)
		}
		if food != "" {
			b.WriteString(" " + food)
		}
	} else if food != "" {
		b.WriteString(food)
	}
	if note != "" {
		if b.Len() > 0 {
			b.WriteString(" — ")
		}
		b.WriteString(note)
	}
	return strings.TrimSpace(b.String())
}

func (c *mealieClient) handleRecipeOfTheDay(w http.ResponseWriter, r *http.Request) {
	recipe, err := c.randomRecipe(r.Context())
	if err != nil {
		log.Printf("recipe-of-the-day: %v", err)
		writeError(w, http.StatusBadGateway, "failed to fetch recipe of the day")
		return
	}
	writeJSON(w, http.StatusOK, recipe)
}

// handleRecipeImage proxies Mealie's recipe image endpoint so the TRMNL device
// does not need credentials. The `image` field in the recipe JSON points here.
func (c *mealieClient) handleRecipeImage(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	file := r.URL.Query().Get("file")
	if id == "" || file == "" {
		writeError(w, http.StatusBadRequest, "id and file query parameters are required")
		return
	}
	path := "/api/media/recipes/" + url.PathEscape(id) + "/images/" + url.PathEscape(file)
	resp, err := c.do(r.Context(), http.MethodGet, path, nil)
	if err != nil {
		log.Printf("recipe-image: %v", err)
		writeError(w, http.StatusBadGateway, "failed to proxy recipe image")
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		writeError(w, http.StatusNotFound, "recipe image not found")
		return
	}
	if resp.StatusCode != http.StatusOK {
		writeError(w, http.StatusBadGateway, "recipe image upstream returned "+resp.Status)
		return
	}
	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.WriteHeader(http.StatusOK)
	if _, err := io.Copy(w, resp.Body); err != nil {
		log.Printf("recipe-image: copy: %v", err)
	}
}
