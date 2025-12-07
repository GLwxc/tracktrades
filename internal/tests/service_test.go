package tests

import (
	"context"
	"testing"

	"tracktrades/internal/app"
	"tracktrades/internal/domain/portfolio"
	"tracktrades/internal/ports"
)

type memRepo struct {
	p *portfolio.Portfolio
}

func (m *memRepo) Load(ctx context.Context) (*portfolio.Portfolio, error) {
	if m.p == nil {
		m.p = portfolio.New("Test", 1000)
	}
	return m.p, nil
}

func (m *memRepo) Save(ctx context.Context, p *portfolio.Portfolio) error {
	m.p = p
	return nil
}

type nopPricer struct{}

func (nopPricer) UpdatePrice(ctx context.Context, p *portfolio.Position) error           { return nil }
func (nopPricer) ComputeHistoricalPeak(ctx context.Context, p *portfolio.Position) error { return nil }

var _ ports.PortfolioRepository = (*memRepo)(nil)
var _ ports.PriceProvider = (*nopPricer)(nil)

func TestServiceAddPosition(t *testing.T) {
	repo := &memRepo{}
	svc := app.NewPortfolioService(repo, nopPricer{})

	ctx := context.Background()
	pos := &portfolio.Position{
		Ticker:       "NVDA",
		Shares:       10,
		CostBasis:    1000,
		CurrentPrice: 120,
	}
	if err := svc.AddOrUpdatePosition(ctx, pos); err != nil {
		t.Fatalf("AddOrUpdatePosition error: %v", err)
	}

	m, err := svc.GetMetrics(ctx)
	if err != nil {
		t.Fatalf("GetMetrics error: %v", err)
	}
	if m.TotalValue <= 0 {
		t.Fatalf("TotalValue = %v, want >0", m.TotalValue)
	}
}
