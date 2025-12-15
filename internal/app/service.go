package app

import (
	"context"
	"fmt"
	"sort"
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

// PortfolioHistory fetches historical prices for the requested tickers (or all positions
// when tickers is empty) and aggregates their total value by day. Cash is added to each
// day's total so the series reflects full portfolio value.
func (s *PortfolioService) PortfolioHistory(ctx context.Context, name string, tickers []string) ([]portfolio.ValuePoint, error) {
	p, err := s.store.Load(ctx, name)
	if err != nil {
		return nil, err
	}

	selected := p.Positions
	if len(tickers) > 0 {
		selected = make(map[string]*portfolio.Position, len(tickers))
		for _, t := range tickers {
			pos, ok := p.Positions[t]
			if !ok {
				return nil, fmt.Errorf("position %s not found", t)
			}
			selected[t] = pos
		}
	}

	totals := make(map[time.Time]float64)
	for _, pos := range selected {
		history, err := s.pricer.PriceHistory(ctx, pos)
		if err != nil {
			return nil, err
		}
		for _, pt := range history {
			totals[pt.Date] += pt.Price * pos.Shares
		}
	}

	if len(totals) == 0 {
		return nil, fmt.Errorf("no historical data available")
	}

	// add cash to every aggregated date
	for d := range totals {
		totals[d] += p.Cash
	}

	dates := make([]time.Time, 0, len(totals))
	for d := range totals {
		dates = append(dates, d)
	}
	sort.Slice(dates, func(i, j int) bool { return dates[i].Before(dates[j]) })

	points := make([]portfolio.ValuePoint, 0, len(dates))
	for _, d := range dates {
		points = append(points, portfolio.ValuePoint{Date: d, Value: totals[d]})
	}
	return points, nil
}
