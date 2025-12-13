package storage

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"tracktrades/internal/domain/portfolio"
	"tracktrades/internal/ports"
)

// GzipPortfolioStore keeps portfolios as gzipped JSON files within a base directory.
// Each portfolio is stored as <name>.json.gz.
type GzipPortfolioStore struct {
	baseDir string
	mu      sync.RWMutex
}

var _ ports.PortfolioStore = (*GzipPortfolioStore)(nil)

func NewGzipPortfolioStore(baseDir string) *GzipPortfolioStore {
	return &GzipPortfolioStore{baseDir: baseDir}
}

func (s *GzipPortfolioStore) Create(ctx context.Context, name string, cash float64) (*portfolio.Portfolio, error) {
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

func (s *GzipPortfolioStore) List(ctx context.Context) ([]string, error) {
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
		if strings.HasSuffix(name, ".json.gz") {
			names = append(names, strings.TrimSuffix(name, ".json.gz"))
		}
	}
	return names, nil
}

func (s *GzipPortfolioStore) Load(ctx context.Context, name string) (*portfolio.Portfolio, error) {
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

	uncompressed, err := s.inflate(data)
	if err != nil {
		return nil, err
	}

	var p portfolio.Portfolio
	if err := json.Unmarshal(uncompressed, &p); err != nil {
		return nil, err
	}
	if p.Positions == nil {
		p.Positions = make(map[string]*portfolio.Position)
	}
	return &p, nil
}

func (s *GzipPortfolioStore) Save(ctx context.Context, name string, p *portfolio.Portfolio) error {
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

func (s *GzipPortfolioStore) Remove(ctx context.Context, name string) error {
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

func (s *GzipPortfolioStore) saveUnlocked(p *portfolio.Portfolio, path string) error {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	compressed, err := s.deflate(data)
	if err != nil {
		return err
	}
	return os.WriteFile(path, compressed, 0o644)
}

func (s *GzipPortfolioStore) pathFor(name string) string {
	filename := fmt.Sprintf("%s.json.gz", name)
	return filepath.Join(s.baseDir, filename)
}

func (s *GzipPortfolioStore) deflate(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(data); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (s *GzipPortfolioStore) inflate(data []byte) ([]byte, error) {
	zr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer zr.Close()

	return io.ReadAll(zr)
}
