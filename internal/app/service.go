package app

import (
	"context"
	"time"

	"tracktrades/internal/domain/portfolio"
	"tracktrades/internal/ports"
)

type PortfolioService struct {
	repo   ports.PortfolioRepository
	pricer ports.PriceProvider
}

func NewPortfolioService(repo ports.PortfolioRepository, pricer ports.PriceProvider) *PortfolioService {
	return &PortfolioService{
		repo:   repo,
		pricer: pricer,
	}
}

func (s *PortfolioService) InitPortfolio(ctx context.Context, name string, cash float64) (*portfolio.Portfolio, error) {
	p, err := s.repo.Load(ctx)
	if err != nil {
		return nil, err
	}
	if p.Name == "" || (len(p.Positions) == 0 && p.Cash == 0) {
		p.Name = name
		p.Cash = cash
		if err := s.repo.Save(ctx, p); err != nil {
			return nil, err
		}
	}
	return p, nil
}

func (s *PortfolioService) GetMetrics(ctx context.Context) (portfolio.PortfolioMetrics, error) {
	p, err := s.repo.Load(ctx)
	if err != nil {
		return portfolio.PortfolioMetrics{}, err
	}
	return p.Metrics(), nil
}

func (s *PortfolioService) ListPositions(ctx context.Context) ([]portfolio.PositionDetails, error) {
	p, err := s.repo.Load(ctx)
	if err != nil {
		return nil, err
	}
	res := make([]portfolio.PositionDetails, 0, len(p.Positions))
	for _, pos := range p.Positions {
		res = append(res, pos.DetailedMetrics())
	}
	return res, nil
}

func (s *PortfolioService) GetPosition(ctx context.Context, ticker string) (portfolio.PositionDetails, bool, error) {
	p, err := s.repo.Load(ctx)
	if err != nil {
		return portfolio.PositionDetails{}, false, err
	}
	d, ok := p.PositionDetails(ticker)
	return d, ok, nil
}

func (s *PortfolioService) AddOrUpdatePosition(ctx context.Context, pos *portfolio.Position) error {
	p, err := s.repo.Load(ctx)
	if err != nil {
		return err
	}
	p.AddPosition(pos)
	return s.repo.Save(ctx, p)
}

func (s *PortfolioService) RecomputeHistoricalPeaks(ctx context.Context) error {
	p, err := s.repo.Load(ctx)
	if err != nil {
		return err
	}
	for _, pos := range p.Positions {
		if err := s.pricer.ComputeHistoricalPeak(ctx, pos); err != nil {
			continue
		}
	}
	return s.repo.Save(ctx, p)
}

func (s *PortfolioService) UpdateAllPrices(ctx context.Context) error {
	p, err := s.repo.Load(ctx)
	if err != nil {
		return err
	}
	for _, pos := range p.Positions {
		_ = s.pricer.UpdatePrice(ctx, pos)
	}
	return s.repo.Save(ctx, p)
}

func (s *PortfolioService) StartPriceUpdater(ctx context.Context, interval time.Duration) (cancel func()) {
	ctx, cancel = context.WithCancel(ctx)

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				_ = s.UpdateAllPrices(ctx)
			case <-ctx.Done():
				return
			}
		}
	}()

	return cancel
}
