package alphavantage

import "tracktrades/internal/ports"

type Client struct {
	APIKey string
}

func New(apiKey string) *Client {
	return &Client{APIKey: apiKey}
}

var _ ports.PriceProvider = (*Client)(nil)
