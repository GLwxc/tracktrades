package app

import (
	"context"
	"time"

	"tracktrades/internal/domain/portfolio"
	"tracktrades/internal/ports"
)

type PortfolioService struct {
	store  ports.PortfolioStore
	pricer ports.PriceProvider
}

func NewPortfolioService(store ports.PortfolioStore, pricer ports.PriceProvider) *PortfolioService {
	return &PortfolioService{
		store:  store,
		pricer: pricer,
	}
}

func (s *PortfolioService) CreatePortfolio(ctx context.Context, name string, cash float64) (*portfolio.Portfolio, error) {
	return s.store.Create(ctx, name, cash)
}

func (s *PortfolioService) ListPortfolios(ctx context.Context) ([]string, error) {
	return s.store.List(ctx)
}

func (s *PortfolioService) RemovePortfolio(ctx context.Context, name string) error {
	return s.store.Remove(ctx, name)
}

func (s *PortfolioService) GetMetrics(ctx context.Context, name string) (portfolio.PortfolioMetrics, error) {
	p, err := s.store.Load(ctx, name)
	if err != nil {
		return portfolio.PortfolioMetrics{}, err
	}
	return p.Metrics(), nil
}

func (s *PortfolioService) ListPositions(ctx context.Context, name string) ([]portfolio.PositionDetails, error) {
	p, err := s.store.Load(ctx, name)
	if err != nil {
		return nil, err
	}
	res := make([]portfolio.PositionDetails, 0, len(p.Positions))
	for _, pos := range p.Positions {
		res = append(res, pos.DetailedMetrics())
	}
	return res, nil
}

func (s *PortfolioService) GetPosition(ctx context.Context, name, ticker string) (portfolio.PositionDetails, bool, error) {
	p, err := s.store.Load(ctx, name)
	if err != nil {
		return portfolio.PositionDetails{}, false, err
	}
	d, ok := p.PositionDetails(ticker)
	return d, ok, nil
}

func (s *PortfolioService) AddOrUpdatePosition(ctx context.Context, name string, pos *portfolio.Position) error {
	p, err := s.store.Load(ctx, name)
	if err != nil {
		return err
	}
	p.AddPosition(pos)
	return s.store.Save(ctx, name, p)
}

func (s *PortfolioService) RecomputeHistoricalPeaks(ctx context.Context, name string) error {
	p, err := s.store.Load(ctx, name)
	if err != nil {
		return err
	}
	for _, pos := range p.Positions {
		if err := s.pricer.ComputeHistoricalPeak(ctx, pos); err != nil {
			continue
		}
	}
	return s.store.Save(ctx, name, p)
}

func (s *PortfolioService) UpdateAllPrices(ctx context.Context, name string) error {
	p, err := s.store.Load(ctx, name)
	if err != nil {
		return err
	}
	for _, pos := range p.Positions {
		_ = s.pricer.UpdatePrice(ctx, pos)
	}
	return s.store.Save(ctx, name, p)
}

func (s *PortfolioService) StartPriceUpdater(ctx context.Context, name string, interval time.Duration) (cancel func()) {
	ctx, cancel = context.WithCancel(ctx)

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				_ = s.UpdateAllPrices(ctx, name)
			case <-ctx.Done():
				return
			}
		}
	}()

	return cancel
}
