package alphavantage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"tracktrades/internal/domain/portfolio"
)

func (c *Client) UpdatePrice(ctx context.Context, pos *portfolio.Position) error {
	baseURL := "https://www.alphavantage.co/query"

	params := url.Values{"apikey": {c.APIKey}}
	if pos.IsCrypto() {
		params.Set("function", "CURRENCY_EXCHANGE_RATE")
		params.Set("from_currency", pos.SymbolBase())
		params.Set("to_currency", "USD")
	} else {
		params.Set("function", "GLOBAL_QUOTE")
		params.Set("symbol", pos.Ticker)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"?"+params.Encode(), nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return err
	}

	var data map[string]interface{}
	_ = json.Unmarshal(body, &data)

	var price float64
	if q, ok := data["Global Quote"].(map[string]interface{}); ok {
		if s, ok := q["05. price"].(string); ok {
			price, _ = strconv.ParseFloat(s, 64)
		}
	}
	if rate, ok := data["Realtime Currency Exchange Rate"].(map[string]interface{}); ok {
		if s, ok := rate["5. Exchange Rate"].(string); ok {
			price, _ = strconv.ParseFloat(s, 64)
		}
	}

	if price <= 0 {
		return fmt.Errorf("no price found in response for %s", pos.Ticker)
	}

	pos.UpdatePrice(price)
	return nil
}
