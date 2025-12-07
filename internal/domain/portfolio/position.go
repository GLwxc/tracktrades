package portfolio

import (
	"strings"
	"time"
)

type Position struct {
	Ticker       string    `json:"ticker"`
	Shares       float64   `json:"shares"`
	CostBasis    float64   `json:"cost_basis"`
	CurrentPrice float64   `json:"current_price"`
	PeakPrice    float64   `json:"peak_price"`
	EntryDate    time.Time `json:"entry_date"`
	LastUpdate   time.Time `json:"last_update"`
}

func (p *Position) UpdatePrice(price float64) {
	p.CurrentPrice = price
	if price > p.PeakPrice {
		p.PeakPrice = price
	}
	p.LastUpdate = time.Now()
}

func (p *Position) CurrentValue() float64 { return p.Shares * p.CurrentPrice }
func (p *Position) PeakValue() float64    { return p.Shares * p.PeakPrice }

func (p *Position) IsCrypto() bool {
	t := strings.ToUpper(p.Ticker)
	return strings.HasSuffix(t, "USD") && len(t) > 3
}

func (p *Position) SymbolBase() string {
	t := strings.ToUpper(p.Ticker)
	if p.IsCrypto() {
		return strings.TrimSuffix(t, "USD")
	}
	return t
}
