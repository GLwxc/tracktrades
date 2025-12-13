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
	storeInfo, err := storage.NewPortfolioStore("memory")
	if err != nil {
		t.Fatalf("NewPortfolioStore memory: %v", err)
	}

	store := storeInfo.Store
	ctx := context.Background()
	if _, err := store.Create(ctx, "SessionOne", 500); err != nil {
		t.Fatalf("Create: %v", err)
	}

	loaded, err := store.Load(ctx, "SessionOne")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Name != "SessionOne" || loaded.Cash != 500 {
		t.Fatalf("unexpected portfolio: %#v", loaded)
	}

	// Mutating the loaded portfolio should not affect persisted state until Save is called.
	loaded.Cash = 0
	again, err := store.Load(ctx, "SessionOne")
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

	storeInfo, err := storage.NewPortfolioStore("file:" + path)
	if err != nil {
		t.Fatalf("NewPortfolioStore file: %v", err)
	}
	store := storeInfo.Store
	ctx := context.Background()

	if _, err := store.Create(ctx, "persisted", 0); err != nil {
		t.Fatalf("Create: %v", err)
	}

	p, err := store.Load(ctx, "persisted")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	p.Positions["TEST"] = &portfolio.Position{Ticker: "TEST", Shares: 1, CurrentPrice: 10}

	if err := store.Save(ctx, "persisted", p); err != nil {
		t.Fatalf("Save: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "persisted.json"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if len(data) == 0 {
		t.Fatalf("expected data to be written to %s", path)
	}

	loaded, err := store.Load(ctx, "persisted")
	if err != nil {
		t.Fatalf("Load after save: %v", err)
	}
	if loaded.Name != "persisted" {
		t.Fatalf("Name=%s want persisted", loaded.Name)
	}
	if len(loaded.Positions) != 1 {
		t.Fatalf("Positions=%d want 1", len(loaded.Positions))
	}
}

func TestUnsupportedRepositorySpec(t *testing.T) {
	if _, err := storage.NewPortfolioStore("db:postgres"); err == nil {
		t.Fatalf("expected error for unsupported backend")
	}
}
