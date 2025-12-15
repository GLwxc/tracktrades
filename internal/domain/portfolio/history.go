package portfolio

import "time"

// PricePoint represents the closing price for a position at a date.
// It is used by price providers that can return historical data.
type PricePoint struct {
	Date  time.Time `json:"date"`
	Price float64   `json:"price"`
}

// ValuePoint captures the total value of one or more positions (plus cash)
// at a specific date.
type ValuePoint struct {
	Date  time.Time `json:"date"`
	Value float64   `json:"value"`
}
