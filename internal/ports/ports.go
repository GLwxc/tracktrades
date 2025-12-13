package ports

import (
	"context"

	"tracktrades/internal/domain/portfolio"
)

type PortfolioStore interface {
	Create(ctx context.Context, name string, cash float64) (*portfolio.Portfolio, error)
	List(ctx context.Context) ([]string, error)
	Load(ctx context.Context, name string) (*portfolio.Portfolio, error)
	Save(ctx context.Context, name string, p *portfolio.Portfolio) error
	Remove(ctx context.Context, name string) error
}

type PriceProvider interface {
	UpdatePrice(ctx context.Context, pos *portfolio.Position) error
	ComputeHistoricalPeak(ctx context.Context, pos *portfolio.Position) error
}
