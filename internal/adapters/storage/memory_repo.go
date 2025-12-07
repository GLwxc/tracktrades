package storage

import (
	"context"
	"sync"

	"tracktrades/internal/domain/portfolio"
	"tracktrades/internal/ports"
)

// MemoryPortfolioRepository keeps portfolio data in-memory. Useful for tests
// or ephemeral runs where persistence is not required.
type MemoryPortfolioRepository struct {
	mu sync.RWMutex
	p  *portfolio.Portfolio
}

var _ ports.PortfolioRepository = (*MemoryPortfolioRepository)(nil)

func NewMemoryPortfolioRepository() *MemoryPortfolioRepository {
	return &MemoryPortfolioRepository{}
}

func (r *MemoryPortfolioRepository) Load(ctx context.Context) (*portfolio.Portfolio, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.p == nil {
		return portfolio.New("My Portfolio", 0), nil
	}
	return r.clone(), nil
}

func (r *MemoryPortfolioRepository) Save(ctx context.Context, p *portfolio.Portfolio) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.p = r.copyFrom(p)
	return nil
}

func (r *MemoryPortfolioRepository) clone() *portfolio.Portfolio {
	if r.p == nil {
		return nil
	}
	return r.copyFrom(r.p)
}

func (r *MemoryPortfolioRepository) copyFrom(src *portfolio.Portfolio) *portfolio.Portfolio {
	dst := *src
	if src.Positions != nil {
		dst.Positions = make(map[string]*portfolio.Position, len(src.Positions))
		for k, v := range src.Positions {
			pos := *v
			dst.Positions[k] = &pos
		}
	}
	return &dst
}
