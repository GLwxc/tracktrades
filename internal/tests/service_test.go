package tests

import (
	"context"
	"fmt"
	"testing"
	"time"

	"tracktrades/internal/adapters/storage"
	"tracktrades/internal/app"
	"tracktrades/internal/domain/portfolio"
	"tracktrades/internal/ports"
)

type nopPricer struct{}

func (nopPricer) UpdatePrice(ctx context.Context, p *portfolio.Position) error           { return nil }
func (nopPricer) ComputeHistoricalPeak(ctx context.Context, p *portfolio.Position) error { return nil }
func (nopPricer) PriceHistory(ctx context.Context, p *portfolio.Position) ([]portfolio.PricePoint, error) {
	return nil, nil
}

type bumpPricer struct{}

func (bumpPricer) UpdatePrice(ctx context.Context, p *portfolio.Position) error {
	p.CurrentPrice += 1
	return nil
}

func (bumpPricer) ComputeHistoricalPeak(ctx context.Context, p *portfolio.Position) error {
	if p.CurrentPrice > p.PeakPrice {
		p.PeakPrice = p.CurrentPrice
	}
	return nil
}
func (bumpPricer) PriceHistory(ctx context.Context, p *portfolio.Position) ([]portfolio.PricePoint, error) {
	return []portfolio.PricePoint{
		{Date: p.EntryDate.AddDate(0, 0, 0), Price: p.CurrentPrice},
		{Date: p.EntryDate.AddDate(0, 0, 1), Price: p.CurrentPrice + 1},
	}, nil
}

type fixedHistoryPricer struct {
	histories map[string][]portfolio.PricePoint
}

func (fixedHistoryPricer) UpdatePrice(ctx context.Context, p *portfolio.Position) error { return nil }
func (fixedHistoryPricer) ComputeHistoricalPeak(ctx context.Context, p *portfolio.Position) error {
	return nil
}
func (f fixedHistoryPricer) PriceHistory(ctx context.Context, p *portfolio.Position) ([]portfolio.PricePoint, error) {
	pts, ok := f.histories[p.Ticker]
	if !ok {
		return nil, fmt.Errorf("no history for %s", p.Ticker)
	}
	return pts, nil
}

var _ ports.PriceProvider = (*nopPricer)(nil)
var _ ports.PriceProvider = (*bumpPricer)(nil)

func TestServiceAddAndPersistPosition(t *testing.T) {
	storeInfo, err := storage.NewPortfolioStore("memory")
	if err != nil {
		t.Fatalf("NewPortfolioStore memory: %v", err)
	}

	svc := app.NewPortfolioService(storeInfo.Store, nopPricer{})

	ctx := context.Background()
	if _, err := svc.CreatePortfolio(ctx, "Test", 500); err != nil {
		t.Fatalf("CreatePortfolio: %v", err)
	}

	pos := &portfolio.Position{
		Ticker:       "NVDA",
		Shares:       2,
		CostBasis:    200,
		CurrentPrice: 110,
	}
	pos.UpdatePrice(pos.CurrentPrice)

	if err := svc.AddOrUpdatePosition(ctx, "Test", pos); err != nil {
		t.Fatalf("AddOrUpdatePosition: %v", err)
	}

	metrics, err := svc.GetMetrics(ctx, "Test")
	if err != nil {
		t.Fatalf("GetMetrics: %v", err)
	}

	wantTotal := 500.0 + (2 * 110)
	if metrics.TotalValue != wantTotal {
		t.Fatalf("TotalValue=%v want %v", metrics.TotalValue, wantTotal)
	}

	detail, ok, err := svc.GetPosition(ctx, "Test", "NVDA")
	if err != nil {
		t.Fatalf("GetPosition: %v", err)
	}
	if !ok {
		t.Fatalf("position NVDA not found")
	}
	if detail.Ticker != "NVDA" || detail.Shares != 2 {
		t.Fatalf("unexpected detail: %#v", detail)
	}
}

func TestServiceUpdatesPricesAndPeaks(t *testing.T) {
	storeInfo, err := storage.NewPortfolioStore("memory")
	if err != nil {
		t.Fatalf("NewPortfolioStore memory: %v", err)
	}

	svc := app.NewPortfolioService(storeInfo.Store, bumpPricer{})

	ctx := context.Background()
	if _, err := svc.CreatePortfolio(ctx, "Test", 0); err != nil {
		t.Fatalf("CreatePortfolio: %v", err)
	}
	pos := &portfolio.Position{Ticker: "AAPL", Shares: 1, CostBasis: 100, CurrentPrice: 100}
	pos.UpdatePrice(pos.CurrentPrice)
	if err := svc.AddOrUpdatePosition(ctx, "Test", pos); err != nil {
		t.Fatalf("AddOrUpdatePosition: %v", err)
	}

	if err := svc.UpdateAllPrices(ctx, "Test"); err != nil {
		t.Fatalf("UpdateAllPrices: %v", err)
	}

	detail, ok, err := svc.GetPosition(ctx, "Test", "AAPL")
	if err != nil {
		t.Fatalf("GetPosition: %v", err)
	}
	if !ok {
		t.Fatalf("position AAPL not found")
	}

	if detail.CurrentPrice != 101 {
		t.Fatalf("CurrentPrice=%v want 101", detail.CurrentPrice)
	}

	if err := svc.RecomputeHistoricalPeaks(ctx, "Test"); err != nil {
		t.Fatalf("RecomputeHistoricalPeaks: %v", err)
	}

	detail, ok, err = svc.GetPosition(ctx, "Test", "AAPL")
	if err != nil || !ok {
		t.Fatalf("GetPosition after recompute: %v ok=%v", err, ok)
	}
	if detail.PeakValue < detail.CurrentValue {
		t.Fatalf("PeakValue should be at least current value: peak=%v current=%v", detail.PeakValue, detail.CurrentValue)
	}
}

func TestPortfolioHistoryAggregatesPositions(t *testing.T) {
	storeInfo, err := storage.NewPortfolioStore("memory")
	if err != nil {
		t.Fatalf("NewPortfolioStore memory: %v", err)
	}

	hist := fixedHistoryPricer{histories: map[string][]portfolio.PricePoint{
		"AAA": {
			{Date: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), Price: 10},
			{Date: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), Price: 12},
		},
		"BBB": {
			{Date: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), Price: 5},
			{Date: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), Price: 7},
		},
	}}

	svc := app.NewPortfolioService(storeInfo.Store, hist)
	ctx := context.Background()
	if _, err := svc.CreatePortfolio(ctx, "Test", 100); err != nil {
		t.Fatalf("CreatePortfolio: %v", err)
	}

	posA := &portfolio.Position{Ticker: "AAA", Shares: 2, CostBasis: 0, CurrentPrice: 12, EntryDate: time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC)}
	posA.UpdatePrice(posA.CurrentPrice)
	posB := &portfolio.Position{Ticker: "BBB", Shares: 1, CostBasis: 0, CurrentPrice: 7, EntryDate: time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC)}
	posB.UpdatePrice(posB.CurrentPrice)

	if err := svc.AddOrUpdatePosition(ctx, "Test", posA); err != nil {
		t.Fatalf("AddOrUpdatePosition AAA: %v", err)
	}
	if err := svc.AddOrUpdatePosition(ctx, "Test", posB); err != nil {
		t.Fatalf("AddOrUpdatePosition BBB: %v", err)
	}

	history, err := svc.PortfolioHistory(ctx, "Test", nil)
	if err != nil {
		t.Fatalf("PortfolioHistory: %v", err)
	}

	if len(history) != 2 {
		t.Fatalf("expected 2 history points, got %d", len(history))
	}
	// Jan 1: (2*10) + (1*5) + 100 cash
	if history[0].Value != 125 {
		t.Fatalf("unexpected first value: %v", history[0].Value)
	}
	// Jan 2: (2*12) + (1*7) + 100 cash
	if history[1].Value != 131 {
		t.Fatalf("unexpected second value: %v", history[1].Value)
	}

	filtered, err := svc.PortfolioHistory(ctx, "Test", []string{"AAA"})
	if err != nil {
		t.Fatalf("PortfolioHistory filtered: %v", err)
	}
	if len(filtered) != 2 {
		t.Fatalf("expected 2 filtered points, got %d", len(filtered))
	}
	if filtered[1].Value != 124 { // (2*12) + cash
		t.Fatalf("unexpected filtered value: %v", filtered[1].Value)
	}
}
