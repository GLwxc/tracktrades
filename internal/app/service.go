package app

import (
	"context"
	"fmt"
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
	p, err := s.store.Create(ctx, name, cash)
	if err != nil {
		return nil, fmt.Errorf("create portfolio %q: %w", name, err)
	}
	return p, nil
}

func (s *PortfolioService) ListPortfolios(ctx context.Context) ([]string, error) {
	list, err := s.store.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list portfolios: %w", err)
	}
	return list, nil
}

func (s *PortfolioService) RemovePortfolio(ctx context.Context, name string) error {
	if err := s.store.Remove(ctx, name); err != nil {
		return fmt.Errorf("remove portfolio %q: %w", name, err)
	}
	return nil
}

func (s *PortfolioService) GetMetrics(ctx context.Context, name string) (portfolio.PortfolioMetrics, error) {
	p, err := s.store.Load(ctx, name)
	if err != nil {
		return portfolio.PortfolioMetrics{}, fmt.Errorf("load portfolio %q: %w", name, err)
	}
	return p.Metrics(), nil
}

func (s *PortfolioService) ListPositions(ctx context.Context, name string) ([]portfolio.PositionDetails, error) {
	p, err := s.store.Load(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("load portfolio %q: %w", name, err)
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
		return portfolio.PositionDetails{}, false, fmt.Errorf("load portfolio %q: %w", name, err)
	}
	d, ok := p.PositionDetails(ticker)
	return d, ok, nil
}

func (s *PortfolioService) AddOrUpdatePosition(ctx context.Context, name string, pos *portfolio.Position) error {
	p, err := s.store.Load(ctx, name)
	if err != nil {
		return fmt.Errorf("load portfolio %q: %w", name, err)
	}
	p.AddPosition(pos)
	if err := s.store.Save(ctx, name, p); err != nil {
		return fmt.Errorf("save portfolio %q: %w", name, err)
	}
	return nil
}

func (s *PortfolioService) RecomputeHistoricalPeaks(ctx context.Context, name string) error {
	p, err := s.store.Load(ctx, name)
	if err != nil {
		return fmt.Errorf("load portfolio %q: %w", name, err)
	}
	for _, pos := range p.Positions {
		if err := s.pricer.ComputeHistoricalPeak(ctx, pos); err != nil {
			continue
		}
	}
	if err := s.store.Save(ctx, name, p); err != nil {
		return fmt.Errorf("save portfolio %q: %w", name, err)
	}
	return nil
}

func (s *PortfolioService) UpdateAllPrices(ctx context.Context, name string) error {
	p, err := s.store.Load(ctx, name)
	if err != nil {
		return fmt.Errorf("load portfolio %q: %w", name, err)
	}
	for _, pos := range p.Positions {
		_ = s.pricer.UpdatePrice(ctx, pos)
	}
	if err := s.store.Save(ctx, name, p); err != nil {
		return fmt.Errorf("save portfolio %q: %w", name, err)
	}
	return nil
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
