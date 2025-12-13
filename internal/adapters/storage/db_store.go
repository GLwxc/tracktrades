package storage

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"tracktrades/internal/domain/portfolio"
	"tracktrades/internal/ports"
)

// DBPortfolioStore persists all portfolios in a single JSON file, acting as a
// lightweight database-style backend.
type DBPortfolioStore struct {
	path string
	mu   sync.RWMutex
	data map[string]*portfolio.Portfolio
}

var _ ports.PortfolioStore = (*DBPortfolioStore)(nil)

func NewDBPortfolioStore(path string) (*DBPortfolioStore, error) {
	store := &DBPortfolioStore{path: path, data: make(map[string]*portfolio.Portfolio)}
	if err := store.loadFromDisk(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *DBPortfolioStore) Create(ctx context.Context, name string, cash float64) (*portfolio.Portfolio, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if name == "" {
		return nil, errors.New("portfolio name is required")
	}
	if _, exists := s.data[name]; exists {
		return nil, fs.ErrExist
	}

	p := portfolio.New(name, cash)
	s.data[name] = clonePortfolio(p)
	if err := s.persist(); err != nil {
		return nil, err
	}
	return clonePortfolio(p), nil
}

func (s *DBPortfolioStore) List(ctx context.Context) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	names := make([]string, 0, len(s.data))
	for name := range s.data {
		names = append(names, name)
	}
	return names, nil
}

func (s *DBPortfolioStore) Load(ctx context.Context, name string) (*portfolio.Portfolio, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if name == "" {
		return nil, errors.New("portfolio name is required")
	}

	p, ok := s.data[name]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return clonePortfolio(p), nil
}

func (s *DBPortfolioStore) Save(ctx context.Context, name string, p *portfolio.Portfolio) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if name == "" {
		return errors.New("portfolio name is required")
	}
	s.data[name] = clonePortfolio(p)
	return s.persist()
}

func (s *DBPortfolioStore) Remove(ctx context.Context, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if name == "" {
		return errors.New("portfolio name is required")
	}
	if _, ok := s.data[name]; !ok {
		return fs.ErrNotExist
	}
	delete(s.data, name)
	return s.persist()
}

func (s *DBPortfolioStore) persist() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, encoded, 0o644)
}

func (s *DBPortfolioStore) loadFromDisk() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var stored map[string]*portfolio.Portfolio
	if err := json.Unmarshal(data, &stored); err != nil {
		return err
	}

	s.data = make(map[string]*portfolio.Portfolio, len(stored))
	for name, p := range stored {
		s.data[name] = clonePortfolio(p)
	}
	return nil
}

func clonePortfolio(p *portfolio.Portfolio) *portfolio.Portfolio {
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
