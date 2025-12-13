package storage

import (
	"fmt"
	"path/filepath"
	"strings"

	"tracktrades/internal/ports"
)

const (
	BackendFile   = "file"
	BackendMemory = "memory"
	BackendJSON   = "json"
	BackendGzip   = "gzip"
	BackendSQLite = "sqlite"
)

// NewPortfolioStore returns a portfolio store for the provided backend spec.
// Examples:
//   - "file:portfolio.json" (default portfolio name "portfolio" in current dir)
//   - "file:/tmp/portfolios" (all portfolios stored in /tmp/portfolios)
//   - "memory" (non-persistent, in-memory portfolios)
func NewPortfolioStore(spec string) (*StoreWithInfo, error) {
	backend, arg := parseSpec(spec)

	switch backend {
	case BackendMemory:
		return &StoreWithInfo{Backend: BackendMemory, Store: NewMemoryPortfolioStore(), DefaultPortfolio: "default"}, nil
	case BackendFile, BackendJSON:
		baseDir, defaultPortfolio := normalizePath(arg)
		return &StoreWithInfo{Backend: BackendFile, Store: NewFilePortfolioStore(baseDir), DefaultPortfolio: defaultPortfolio}, nil
	case BackendGzip:
		baseDir, defaultPortfolio := normalizePath(arg)
		return &StoreWithInfo{Backend: BackendGzip, Store: NewGzipPortfolioStore(baseDir), DefaultPortfolio: defaultPortfolio}, nil
	case BackendSQLite:
		dsn := defaultSQLiteDSN(arg)
		store, err := NewDBPortfolioStore(dsn)
		if err != nil {
			return nil, err
		}
		return &StoreWithInfo{Backend: BackendSQLite, Store: store, DefaultPortfolio: "portfolio"}, nil
	default:
		return nil, fmt.Errorf("unsupported portfolio backend: %s", backend)
	}
}

type StoreWithInfo struct {
	Backend          string
	DefaultPortfolio string
	Store            ports.PortfolioStore
}

func parseSpec(spec string) (backend, arg string) {
	if spec == "" {
		return BackendFile, ""
	}

	if !strings.Contains(spec, ":") {
		backend = strings.ToLower(spec)
		switch backend {
		case BackendMemory, BackendFile, BackendJSON, BackendGzip, BackendSQLite:
			return backend, ""
		default:
			// Treat the entire string as a file path for backward compatibility.
			return BackendFile, spec
		}
	}

	parts := strings.SplitN(spec, ":", 2)
	backend = strings.ToLower(parts[0])
	arg = parts[1]
	return backend, arg
}

func normalizePath(path string) (dir, defaultName string) {
	if path == "" {
		return ".", "portfolio"
	}

	if strings.HasSuffix(path, ".json") {
		dir = filepath.Dir(path)
		base := filepath.Base(path)
		return dir, strings.TrimSuffix(base, filepath.Ext(base))
	}
	if strings.HasSuffix(path, ".json.gz") {
		dir = filepath.Dir(path)
		base := filepath.Base(path)
		trimmed := strings.TrimSuffix(base, ".gz")
		return dir, strings.TrimSuffix(trimmed, filepath.Ext(trimmed))
	}

	return path, "portfolio"
}

func defaultSQLiteDSN(arg string) string {
	if arg != "" {
		return arg
	}
	return "portfolio.db"
}
