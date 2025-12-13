package tests

import (
	"context"
	"testing"

	"tracktrades/internal/adapters/storage"
	"tracktrades/internal/app"
	"tracktrades/internal/domain/portfolio"
	"tracktrades/internal/ports"
)

type nopPricer struct{}

func (nopPricer) UpdatePrice(ctx context.Context, p *portfolio.Position) error           { return nil }
func (nopPricer) ComputeHistoricalPeak(ctx context.Context, p *portfolio.Position) error { return nil }

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
