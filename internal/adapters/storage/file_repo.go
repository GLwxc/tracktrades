package storage

import (
	"context"
	"encoding/json"
	"os"
	"sync"

	"tracktrades/internal/domain/portfolio"
	"tracktrades/internal/ports"
)

type FilePortfolioRepository struct {
	path string
	mu   sync.RWMutex
}

var _ ports.PortfolioRepository = (*FilePortfolioRepository)(nil)

func NewFilePortfolioRepository(path string) *FilePortfolioRepository {
	return &FilePortfolioRepository{path: path}
}

func (r *FilePortfolioRepository) Load(ctx context.Context) (*portfolio.Portfolio, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, err := os.Stat(r.path)
	if os.IsNotExist(err) {
		return portfolio.New("My Portfolio", 0), nil
	}

	data, err := os.ReadFile(r.path)
	if err != nil {
		return nil, err
	}

	var p portfolio.Portfolio
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	if p.Positions == nil {
		p.Positions = make(map[string]*portfolio.Position)
	}
	return &p, nil
}

func (r *FilePortfolioRepository) Save(ctx context.Context, p *portfolio.Portfolio) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(r.path, data, 0o644)
}
