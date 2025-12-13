package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"time"

	"tracktrades/internal/adapters/alphavantage"
	"tracktrades/internal/adapters/storage"
	"tracktrades/internal/app"
	"tracktrades/internal/domain/portfolio"
)

func main() {
	apiKey := os.Getenv("ALPHAVANTAGE_API_KEY")
	repoSpec := os.Getenv("PORTFOLIO_STORAGE")
	if repoSpec == "" {
		repoSpec = "file:portfolio.json"
	}
	storeInfo, err := storage.NewPortfolioStore(repoSpec)
	if err != nil {
		log.Fatalf("invalid repository: %v", err)
	}
	defaultPortfolio := envOrDefault("PORTFOLIO_NAME", storeInfo.DefaultPortfolio)

	pricer := alphavantage.New(apiKey)
	svc := app.NewPortfolioService(storeInfo.Store, pricer)

	ctx := context.Background()
	cancel := svc.StartPriceUpdater(ctx, defaultPortfolio, 5*time.Minute)
	defer cancel()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/portfolio", makePortfolioHandler(svc, defaultPortfolio))
	mux.HandleFunc("/positions", makePositionsHandler(svc, defaultPortfolio))
	mux.HandleFunc("/position", makePositionHandler(svc, defaultPortfolio))
	mux.HandleFunc("/recompute-peaks", makeRecomputePeaksHandler(svc, defaultPortfolio))
	mux.HandleFunc("/update-prices", makeUpdatePricesHandler(svc, defaultPortfolio))

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	log.Println("API listening on :8080")
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func makePortfolioHandler(svc *app.PortfolioService, defaultPortfolio string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		portfolioName := portfolioFromRequest(r, defaultPortfolio)

		m, err := svc.GetMetrics(r.Context(), portfolioName)
		if err != nil {
			status := statusForError(err)
			writeError(w, status, fmt.Sprintf("failed to get metrics for portfolio %s: %v", portfolioName, err))
			return
		}
		writeJSON(w, m)
	}
}

func makePositionsHandler(svc *app.PortfolioService, defaultPortfolio string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		portfolioName := portfolioFromRequest(r, defaultPortfolio)

		switch r.Method {
		case http.MethodGet:
			list, err := svc.ListPositions(r.Context(), portfolioName)
			if err != nil {
				status := statusForError(err)
				writeError(w, status, fmt.Sprintf("failed to list positions for portfolio %s: %v", portfolioName, err))
				return
			}
			writeJSON(w, list)
		case http.MethodPost:
			var in portfolio.Position
			if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
				http.Error(w, "invalid json", http.StatusBadRequest)
				return
			}
			if in.Ticker == "" {
				http.Error(w, "ticker required", http.StatusBadRequest)
				return
			}
			if err := svc.AddOrUpdatePosition(r.Context(), portfolioName, &in); err != nil {
				status := statusForError(err)
				writeError(w, status, fmt.Sprintf("failed to save position in portfolio %s: %v", portfolioName, err))
				return
			}
			w.WriteHeader(http.StatusCreated)
			writeJSON(w, map[string]string{"status": "ok"})
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func makePositionHandler(svc *app.PortfolioService, defaultPortfolio string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			ticker := r.URL.Query().Get("ticker")
			if ticker == "" {
				http.Error(w, "ticker required", http.StatusBadRequest)
				return
			}
			portfolioName := portfolioFromRequest(r, defaultPortfolio)
			d, ok, err := svc.GetPosition(r.Context(), portfolioName, ticker)
			if err != nil {
				status := statusForError(err)
				writeError(w, status, fmt.Sprintf("failed to load position %s in portfolio %s: %v", ticker, portfolioName, err))
				return
			}
			if !ok {
				writeError(w, http.StatusNotFound, fmt.Sprintf("position %s not found in portfolio %s", ticker, portfolioName))
				return
			}
			writeJSON(w, d)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func makeRecomputePeaksHandler(svc *app.PortfolioService, defaultPortfolio string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		portfolioName := portfolioFromRequest(r, defaultPortfolio)
		if err := svc.RecomputeHistoricalPeaks(r.Context(), portfolioName); err != nil {
			status := statusForError(err)
			writeError(w, status, fmt.Sprintf("failed to recompute peaks for portfolio %s: %v", portfolioName, err))
			return
		}
		writeJSON(w, map[string]string{"status": "ok"})
	}
}

func makeUpdatePricesHandler(svc *app.PortfolioService, defaultPortfolio string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		portfolioName := portfolioFromRequest(r, defaultPortfolio)
		if err := svc.UpdateAllPrices(r.Context(), portfolioName); err != nil {
			status := statusForError(err)
			writeError(w, status, fmt.Sprintf("failed to update prices for portfolio %s: %v", portfolioName, err))
			return
		}
		writeJSON(w, map[string]string{"status": "ok"})
	}
}

func portfolioFromRequest(r *http.Request, defaultName string) string {
	name := r.URL.Query().Get("portfolio")
	if name == "" {
		return defaultName
	}
	return name
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	http.Error(w, message, status)
}

func statusForError(err error) int {
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}
