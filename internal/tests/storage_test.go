package tests

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"tracktrades/internal/adapters/storage"
	"tracktrades/internal/domain/portfolio"
)

func TestMemoryRepositoryIsIsolated(t *testing.T) {
	repoInfo, err := storage.NewPortfolioRepository("memory")
	if err != nil {
		t.Fatalf("NewPortfolioRepository memory: %v", err)
	}

	repo := repoInfo.Repository
	ctx := context.Background()

	p, err := repo.Load(ctx)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	p.Name = "SessionOne"
	p.Cash = 500
	if err := repo.Save(ctx, p); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := repo.Load(ctx)
	if err != nil {
		t.Fatalf("Load after save: %v", err)
	}
	if loaded.Name != "SessionOne" || loaded.Cash != 500 {
		t.Fatalf("unexpected portfolio: %#v", loaded)
	}

	// Mutating the loaded portfolio should not affect persisted state until Save is called.
	loaded.Cash = 0
	again, err := repo.Load(ctx)
	if err != nil {
		t.Fatalf("Load after mutation: %v", err)
	}
	if again.Cash != 500 {
		t.Fatalf("expected persisted cash to remain 500, got %v", again.Cash)
	}
}

func TestFileRepositorySpecCreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "portfolio.json")

	repoInfo, err := storage.NewPortfolioRepository("file:" + path)
	if err != nil {
		t.Fatalf("NewPortfolioRepository file: %v", err)
	}

	repo := repoInfo.Repository
	ctx := context.Background()

	p, err := repo.Load(ctx)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	p.Name = "Persisted"
	p.Positions["TEST"] = &portfolio.Position{Ticker: "TEST", Shares: 1, CurrentPrice: 10}

	if err := repo.Save(ctx, p); err != nil {
		t.Fatalf("Save: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if len(data) == 0 {
		t.Fatalf("expected data to be written to %s", path)
	}

	loaded, err := repo.Load(ctx)
	if err != nil {
		t.Fatalf("Load after save: %v", err)
	}
	if loaded.Name != "Persisted" {
		t.Fatalf("Name=%s want Persisted", loaded.Name)
	}
	if len(loaded.Positions) != 1 {
		t.Fatalf("Positions=%d want 1", len(loaded.Positions))
	}
}

func TestUnsupportedRepositorySpec(t *testing.T) {
	if _, err := storage.NewPortfolioRepository("db:postgres"); err == nil {
		t.Fatalf("expected error for unsupported backend")
	}
}
