package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
)

// Version is injected at build time via -ldflags "-X main.Version=...".
var Version = "dev"

type config struct {
	mealieURL    string
	mealieAPIKey string
	port         string
}

func main() {
	cfg := config{
		mealieURL:    os.Getenv("MEALIE_URL"),
		mealieAPIKey: os.Getenv("MEALIE_API_KEY"),
		port:         os.Getenv("PORT"),
	}
	if cfg.mealieURL == "" || cfg.mealieAPIKey == "" {
		log.Fatal("MEALIE_URL and MEALIE_API_KEY environment variables are required")
	}
	if cfg.port == "" {
		cfg.port = "8080"
	}

	client := newMealieClient(cfg.mealieURL, cfg.mealieAPIKey)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealthz)
	mux.HandleFunc("GET /api/trmnl/recipe-of-the-day", client.handleRecipeOfTheDay)
	mux.HandleFunc("GET /api/trmnl/recipe-image", client.handleRecipeImage)

	addr := ":" + cfg.port
	log.Printf("trmnl-mealie %s listening on %s (Mealie: %s)", Version, addr, cfg.mealieURL)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "ok")
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("writeJSON: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
