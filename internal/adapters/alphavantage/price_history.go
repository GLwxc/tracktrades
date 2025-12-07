package alphavantage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"tracktrades/internal/domain/portfolio"
)

func (c *Client) ComputeHistoricalPeak(ctx context.Context, pos *portfolio.Position) error {
	if pos.EntryDate.IsZero() {
		return fmt.Errorf("entry date missing for %s", pos.Ticker)
	}

	function := "TIME_SERIES_DAILY"
	if pos.IsCrypto() {
		function = "DIGITAL_CURRENCY_DAILY"
	}

	params := url.Values{
		"function":   {function},
		"symbol":     {pos.SymbolBase()},
		"apikey":     {c.APIKey},
		"outputsize": {"full"},
	}
	if pos.IsCrypto() {
		params.Set("market", "USD")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://www.alphavantage.co/query?"+params.Encode(), nil)
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

	if note, ok := data["Note"].(string); ok {
		return fmt.Errorf("API limit: %s", note)
	}

	var series map[string]interface{}
	if pos.IsCrypto() {
		series, _ = data["Time Series (Digital Currency Daily)"].(map[string]interface{})
	} else {
		series, _ = data["Time Series (Daily)"].(map[string]interface{})
	}
	if series == nil {
		return fmt.Errorf("no historical series in response for %s", pos.Ticker)
	}

	maxPrice := pos.PeakPrice
	if maxPrice < pos.CurrentPrice {
		maxPrice = pos.CurrentPrice
	}

	for ds, raw := range series {
		d, err := time.Parse("2006-01-02", ds)
		if err != nil || d.Before(pos.EntryDate) {
			continue
		}

		day, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}

		var hs string
		if pos.IsCrypto() {
			hs, _ = day["2b. high (USD)"].(string)
		} else {
			hs, _ = day["2. high"].(string)
		}

		h, err := strconv.ParseFloat(hs, 64)
		if err == nil && h > maxPrice {
			maxPrice = h
		}
	}

	pos.PeakPrice = maxPrice
	pos.UpdatePrice(pos.CurrentPrice)
	return nil
}
