package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"tracktrades/internal/adapters/storage"
	"tracktrades/internal/app"
	"tracktrades/internal/domain/portfolio"
)

func newCLITestService(t *testing.T) (*app.PortfolioService, string) {
	t.Helper()

	storeInfo, err := storage.NewPortfolioStore("memory")
	if err != nil {
		t.Fatalf("NewPortfolioStore memory: %v", err)
	}

	svc := app.NewPortfolioService(storeInfo.Store, noopPricer{})
	ctx := context.Background()
	portfolioName := "CLI"
	if _, err := svc.CreatePortfolio(ctx, portfolioName, 1000); err != nil {
		t.Fatalf("CreatePortfolio: %v", err)
	}

	pos := &portfolio.Position{Ticker: "MSFT", Shares: 5, CostBasis: 500, CurrentPrice: 250}
	pos.UpdatePrice(pos.CurrentPrice)
	if err := svc.AddOrUpdatePosition(ctx, portfolioName, pos); err != nil {
		t.Fatalf("AddOrUpdatePosition: %v", err)
	}

	return svc, portfolioName
}

func captureOutput(t *testing.T, fn func()) string {
	t.Helper()

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Pipe: %v", err)
	}

	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("Copy: %v", err)
	}

	return buf.String()
}

func TestRunMetrics(t *testing.T) {
	svc, portfolioName := newCLITestService(t)

	out := captureOutput(t, func() {
		if err := runMetrics(context.Background(), svc, portfolioName, nil); err != nil {
			t.Fatalf("runMetrics: %v", err)
		}
	})

	if !strings.Contains(out, "total_value") {
		t.Fatalf("expected metrics output, got %q", out)
	}
}

func TestRunPositionValidation(t *testing.T) {
	svc, portfolioName := newCLITestService(t)

	err := runPosition(context.Background(), svc, portfolioName, nil)
	if err == nil || !strings.Contains(err.Error(), "--ticker is required") {
		t.Fatalf("expected ticker validation error, got %v", err)
	}
}

func TestRunUpdatePricesRequiresAPIKey(t *testing.T) {
        svc, portfolioName := newCLITestService(t)
        t.Setenv("ALPHAVANTAGE_API_KEY", "")

        err := runUpdatePrices(context.Background(), svc, portfolioName, os.Getenv("ALPHAVANTAGE_API_KEY"), nil)
        if err == nil || !strings.Contains(err.Error(), "ALPHAVANTAGE_API_KEY") {
                t.Fatalf("expected API key error, got %v", err)
        }
}
