package tests

import (
	"testing"

	"tracktrades/internal/domain/portfolio"
)

func TestPositionMetrics(t *testing.T) {
	p := &portfolio.Position{
		Ticker:       "TEST",
		Shares:       10,
		CostBasis:    1000,
		CurrentPrice: 80,
		PeakPrice:    120,
	}
	d := p.DetailedMetrics()

	if d.CurrentValue != 800 {
		t.Fatalf("CurrentValue = %v, want 800", d.CurrentValue)
	}
	if d.PeakValue != 1200 {
		t.Fatalf("PeakValue = %v, want 1200", d.PeakValue)
	}
	if d.UnrealizedPnL != -200 {
		t.Fatalf("PnL = %v, want -200", d.UnrealizedPnL)
	}
	if d.DrawdownFromPeakPct <= 30 || d.DrawdownFromPeakPct >= 40 {
		t.Fatalf("DrawdownFromPeakPct = %v, want ~33.33", d.DrawdownFromPeakPct)
	}
}
