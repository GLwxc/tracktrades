package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"tracktrades/internal/domain/portfolio"
	"tracktrades/internal/ports"
)

// FilePortfolioStore keeps portfolios as individual JSON files within a base directory.
// Each portfolio is stored as <name>.json.
type FilePortfolioStore struct {
	baseDir string
	mu      sync.RWMutex
}

var _ ports.PortfolioStore = (*FilePortfolioStore)(nil)

func NewFilePortfolioStore(baseDir string) *FilePortfolioStore {
	return &FilePortfolioStore{baseDir: baseDir}
}

func (s *FilePortfolioStore) Create(ctx context.Context, name string, cash float64) (*portfolio.Portfolio, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if name == "" {
		return nil, errors.New("portfolio name is required")
	}

	path := s.pathFor(name)
	if _, err := os.Stat(path); err == nil {
		return nil, fs.ErrExist
	}

	if err := os.MkdirAll(s.baseDir, 0o755); err != nil {
		return nil, err
	}

	p := portfolio.New(name, cash)
	if err := s.saveUnlocked(p, path); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *FilePortfolioStore) List(ctx context.Context) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.baseDir)
	if os.IsNotExist(err) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}

	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".json") {
			names = append(names, strings.TrimSuffix(name, filepath.Ext(name)))
		}
	}
	return names, nil
}

func (s *FilePortfolioStore) Load(ctx context.Context, name string) (*portfolio.Portfolio, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if name == "" {
		return nil, errors.New("portfolio name is required")
	}

	path := s.pathFor(name)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fs.ErrNotExist
		}
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

func (s *FilePortfolioStore) Save(ctx context.Context, name string, p *portfolio.Portfolio) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if name == "" {
		return errors.New("portfolio name is required")
	}

	path := s.pathFor(name)
	if err := os.MkdirAll(s.baseDir, 0o755); err != nil {
		return err
	}
	return s.saveUnlocked(p, path)
}

func (s *FilePortfolioStore) Remove(ctx context.Context, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if name == "" {
		return errors.New("portfolio name is required")
	}
	path := s.pathFor(name)
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return fs.ErrNotExist
		}
		return err
	}
	return nil
}

func (s *FilePortfolioStore) saveUnlocked(p *portfolio.Portfolio, path string) error {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func (s *FilePortfolioStore) pathFor(name string) string {
	filename := fmt.Sprintf("%s.json", name)
	return filepath.Join(s.baseDir, filename)
}
