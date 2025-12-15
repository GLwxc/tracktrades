package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"tracktrades/internal/adapters/storage"
	"tracktrades/internal/app"
	"tracktrades/internal/domain/portfolio"
)

type testPricer struct{}

func (testPricer) UpdatePrice(ctx context.Context, p *portfolio.Position) error           { return nil }
func (testPricer) ComputeHistoricalPeak(ctx context.Context, p *portfolio.Position) error { return nil }
func (testPricer) PriceHistory(ctx context.Context, p *portfolio.Position) ([]portfolio.PricePoint, error) {
	return []portfolio.PricePoint{{Date: time.Now(), Price: p.CurrentPrice}}, nil
}

func newTestService(t *testing.T) (*app.PortfolioService, string) {
	t.Helper()

	storeInfo, err := storage.NewPortfolioStore("memory")
	if err != nil {
		t.Fatalf("NewPortfolioStore memory: %v", err)
	}

	svc := app.NewPortfolioService(storeInfo.Store, testPricer{})
	ctx := context.Background()
	portfolioName := "Test"
	if _, err := svc.CreatePortfolio(ctx, portfolioName, 500); err != nil {
		t.Fatalf("CreatePortfolio: %v", err)
	}

	pos := &portfolio.Position{Ticker: "AAPL", Shares: 2, CostBasis: 200, CurrentPrice: 125}
	pos.UpdatePrice(pos.CurrentPrice)
	if err := svc.AddOrUpdatePosition(ctx, portfolioName, pos); err != nil {
		t.Fatalf("AddOrUpdatePosition: %v", err)
	}

	return svc, portfolioName
}

func TestHealthHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	healthHandler(w, req)

	res := w.Result()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status=%d want %d", res.StatusCode, http.StatusOK)
	}
}

func TestPositionsHandler(t *testing.T) {
	svc, portfolioName := newTestService(t)
	handler := makePositionsHandler(svc, portfolioName)

	req := httptest.NewRequest(http.MethodGet, "/positions", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	res := w.Result()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status=%d want %d", res.StatusCode, http.StatusOK)
	}

	defer res.Body.Close()
	var got []portfolio.Position
	if err := json.NewDecoder(res.Body).Decode(&got); err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if len(got) != 1 || got[0].Ticker != "AAPL" {
		t.Fatalf("unexpected response: %+v", got)
	}
}

func TestPositionsHandlerValidation(t *testing.T) {
	svc, portfolioName := newTestService(t)
	handler := makePositionsHandler(svc, portfolioName)

	req := httptest.NewRequest(http.MethodPost, "/positions", bytes.NewBufferString(`{"shares":1}`))
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d", w.Result().StatusCode)
	}
}

func TestPositionHandler(t *testing.T) {
	svc, portfolioName := newTestService(t)
	handler := makePositionHandler(svc, portfolioName)

	req := httptest.NewRequest(http.MethodGet, "/position?ticker=AAPL", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	res := w.Result()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status=%d want %d", res.StatusCode, http.StatusOK)
	}
}
