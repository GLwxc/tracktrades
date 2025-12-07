package portfolio

import "tracktrades/internal/util"

type PositionDetails struct {
	Ticker              string  `json:"ticker"`
	Shares              float64 `json:"shares"`
	CostBasis           float64 `json:"cost_basis"`
	CurrentPrice        float64 `json:"current_price"`
	CurrentValue        float64 `json:"current_value"`
	PeakValue           float64 `json:"peak_value"`
	UnrealizedPnL       float64 `json:"unrealized_pnl"`
	UnrealizedPnLPct    float64 `json:"unrealized_pnl_pct"`
	DrawdownFromPeakPct float64 `json:"drawdown_from_peak_pct"`
	RecoveryNeededPct   float64 `json:"recovery_needed_pct"`
}

func (p *Position) DetailedMetrics() PositionDetails {
	curr := p.CurrentValue()
	peak := p.PeakValue()

	pnl := curr - p.CostBasis
	pnlPct := 0.0
	if p.CostBasis > 0 {
		pnlPct = (pnl / p.CostBasis) * 100
	}

	drawdownPct := 0.0
	if peak > 0 {
		drawdownPct = ((peak - curr) / peak) * 100
	}

	return PositionDetails{
		Ticker:              p.Ticker,
		Shares:              p.Shares,
		CostBasis:           p.CostBasis,
		CurrentPrice:        p.CurrentPrice,
		CurrentValue:        curr,
		PeakValue:           peak,
		UnrealizedPnL:       pnl,
		UnrealizedPnLPct:    pnlPct,
		DrawdownFromPeakPct: drawdownPct,
		RecoveryNeededPct:   util.RequiredRecoveryPct(drawdownPct),
	}
}

type PortfolioMetrics struct {
	TotalValue          float64 `json:"total_value"`
	UnrealizedPnL       float64 `json:"unrealized_pnl"`
	UnrealizedPnLPct    float64 `json:"unrealized_pnl_pct"`
	DrawdownFromPeakPct float64 `json:"drawdown_from_peak_pct"`
	RecoveryNeededPct   float64 `json:"recovery_needed_pct"`
}

func (p *Portfolio) Metrics() PortfolioMetrics {
	total := p.TotalValue()

	cost := 0.0
	for _, pos := range p.Positions {
		cost += pos.CostBasis
	}

	if total > p.PeakValue {
		p.PeakValue = total
	}

	pnl := total - cost
	pnlPct := 0.0
	if cost > 0 {
		pnlPct = pnl / cost * 100
	}

	dd := 0.0
	if p.PeakValue > 0 {
		dd = ((p.PeakValue - total) / p.PeakValue) * 100
	}

	return PortfolioMetrics{
		TotalValue:          total,
		UnrealizedPnL:       pnl,
		UnrealizedPnLPct:    pnlPct,
		DrawdownFromPeakPct: dd,
		RecoveryNeededPct:   util.RequiredRecoveryPct(dd),
	}
}
