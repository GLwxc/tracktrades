package storage

import (
	"fmt"
	"strings"

	"tracktrades/internal/ports"
)

const (
	BackendFile   = "file"
	BackendMemory = "memory"
	BackendJSON   = "json"
)

// NewPortfolioRepository returns a repository for the provided backend spec.
// Examples:
//   - "file:portfolio.json"
//   - "json:/tmp/portfolio.json"
//   - "memory"
//
// If no backend is specified, the argument is treated as a file path.
func NewPortfolioRepository(spec string) (*RepositoryWithInfo, error) {
	backend, arg := parseSpec(spec)

	switch backend {
	case BackendMemory:
		return &RepositoryWithInfo{Backend: BackendMemory, Repository: NewMemoryPortfolioRepository()}, nil
	case BackendFile, BackendJSON:
		path := arg
		if path == "" {
			path = "portfolio.json"
		}
		return &RepositoryWithInfo{Backend: BackendFile, Repository: NewFilePortfolioRepository(path)}, nil
	default:
		return nil, fmt.Errorf("unsupported portfolio backend: %s", backend)
	}
}

type RepositoryWithInfo struct {
	Backend    string
	Repository ports.PortfolioRepository
}

func parseSpec(spec string) (backend, arg string) {
	if spec == "" {
		return BackendFile, "portfolio.json"
	}

	if !strings.Contains(spec, ":") {
		backend = strings.ToLower(spec)
		switch backend {
		case BackendMemory, BackendFile, BackendJSON:
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
