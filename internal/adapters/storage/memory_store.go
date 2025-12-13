package storage

import (
	"context"
	"errors"
	"io/fs"
	"sync"

	"tracktrades/internal/domain/portfolio"
	"tracktrades/internal/ports"
)

// MemoryPortfolioStore keeps portfolios in-memory. Useful for tests or ephemeral runs.
type MemoryPortfolioStore struct {
	mu    sync.RWMutex
	store map[string]*portfolio.Portfolio
}

var _ ports.PortfolioStore = (*MemoryPortfolioStore)(nil)

func NewMemoryPortfolioStore() *MemoryPortfolioStore {
	return &MemoryPortfolioStore{store: make(map[string]*portfolio.Portfolio)}
}

func (s *MemoryPortfolioStore) Create(ctx context.Context, name string, cash float64) (*portfolio.Portfolio, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if name == "" {
		return nil, errors.New("portfolio name is required")
	}
	if _, exists := s.store[name]; exists {
		return nil, fs.ErrExist
	}
	p := portfolio.New(name, cash)
	s.store[name] = p
	return s.clone(p), nil
}

func (s *MemoryPortfolioStore) List(ctx context.Context) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	names := make([]string, 0, len(s.store))
	for name := range s.store {
		names = append(names, name)
	}
	return names, nil
}

func (s *MemoryPortfolioStore) Load(ctx context.Context, name string) (*portfolio.Portfolio, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if name == "" {
		return nil, errors.New("portfolio name is required")
	}

	p, ok := s.store[name]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return s.clone(p), nil
}

func (s *MemoryPortfolioStore) Save(ctx context.Context, name string, p *portfolio.Portfolio) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if name == "" {
		return errors.New("portfolio name is required")
	}
	s.store[name] = s.clone(p)
	return nil
}

func (s *MemoryPortfolioStore) Remove(ctx context.Context, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if name == "" {
		return errors.New("portfolio name is required")
	}
	if _, ok := s.store[name]; !ok {
		return fs.ErrNotExist
	}
	delete(s.store, name)
	return nil
}

func (s *MemoryPortfolioStore) clone(p *portfolio.Portfolio) *portfolio.Portfolio {
	if p == nil {
		return nil
	}
	cp := *p
	if p.Positions != nil {
		cp.Positions = make(map[string]*portfolio.Position, len(p.Positions))
		for k, v := range p.Positions {
			pos := *v
			cp.Positions[k] = &pos
		}
	}
	return &cp
}
