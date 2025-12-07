package ports

import (
	"context"

	"tracktrades/internal/domain/portfolio"
)

type PortfolioRepository interface {
	Load(ctx context.Context) (*portfolio.Portfolio, error)
	Save(ctx context.Context, p *portfolio.Portfolio) error
}

type PriceProvider interface {
	UpdatePrice(ctx context.Context, pos *portfolio.Position) error
	ComputeHistoricalPeak(ctx context.Context, pos *portfolio.Position) error
}
