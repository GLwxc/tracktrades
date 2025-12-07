package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"tracktrades/internal/adapters/alphavantage"
	"tracktrades/internal/adapters/storage"
	"tracktrades/internal/app"
	"tracktrades/internal/domain/portfolio"
)

func main() {
	apiKey := "YOUR_ALPHA_VANTAGE_KEY"
	repo := storage.NewFilePortfolioRepository("portfolio.json")
	pricer := alphavantage.New(apiKey)
	svc := app.NewPortfolioService(repo, pricer)

	ctx := context.Background()
	_, err := svc.InitPortfolio(ctx, "My Portfolio", 8500)
	if err != nil {
		log.Fatalf("init portfolio: %v", err)
	}

	cancel := svc.StartPriceUpdater(ctx, 5*time.Minute)
	defer cancel()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/portfolio", makePortfolioHandler(svc))
	mux.HandleFunc("/positions", makePositionsHandler(svc))
	mux.HandleFunc("/position", makePositionHandler(svc))
	mux.HandleFunc("/recompute-peaks", makeRecomputePeaksHandler(svc))
	mux.HandleFunc("/update-prices", makeUpdatePricesHandler(svc))

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

func makePortfolioHandler(svc *app.PortfolioService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		m, err := svc.GetMetrics(r.Context())
		if err != nil {
			http.Error(w, "failed to get metrics", http.StatusInternalServerError)
			return
		}
		writeJSON(w, m)
	}
}

func makePositionsHandler(svc *app.PortfolioService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			list, err := svc.ListPositions(r.Context())
			if err != nil {
				http.Error(w, "failed to list positions", http.StatusInternalServerError)
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
			if err := svc.AddOrUpdatePosition(r.Context(), &in); err != nil {
				http.Error(w, "failed to save position", http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusCreated)
			writeJSON(w, map[string]string{"status": "ok"})
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func makePositionHandler(svc *app.PortfolioService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			ticker := r.URL.Query().Get("ticker")
			if ticker == "" {
				http.Error(w, "ticker required", http.StatusBadRequest)
				return
			}
			d, ok, err := svc.GetPosition(r.Context(), ticker)
			if err != nil {
				http.Error(w, "error", http.StatusInternalServerError)
				return
			}
			if !ok {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			writeJSON(w, d)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func makeRecomputePeaksHandler(svc *app.PortfolioService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := svc.RecomputeHistoricalPeaks(r.Context()); err != nil {
			http.Error(w, "failed", http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"status": "ok"})
	}
}

func makeUpdatePricesHandler(svc *app.PortfolioService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := svc.UpdateAllPrices(r.Context()); err != nil {
			http.Error(w, "failed", http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"status": "ok"})
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
