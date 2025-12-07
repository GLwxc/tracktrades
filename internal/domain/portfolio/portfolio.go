package portfolio

type Portfolio struct {
	Name      string               `json:"name"`
	Cash      float64              `json:"cash"`
	Positions map[string]*Position `json:"positions"`
	PeakValue float64              `json:"peak_value"`
}

func New(name string, cash float64) *Portfolio {
	return &Portfolio{
		Name:      name,
		Cash:      cash,
		Positions: make(map[string]*Position),
	}
}

func (p *Portfolio) AddPosition(pos *Position) {
	if p.Positions == nil {
		p.Positions = make(map[string]*Position)
	}
	p.Positions[pos.Ticker] = pos
}

func (p *Portfolio) TotalValue() float64 {
	v := p.Cash
	for _, pos := range p.Positions {
		v += pos.CurrentValue()
	}
	return v
}

func (p *Portfolio) PositionDetails(ticker string) (PositionDetails, bool) {
	pos, ok := p.Positions[ticker]
	if !ok {
		return PositionDetails{}, false
	}
	return pos.DetailedMetrics(), true
}
