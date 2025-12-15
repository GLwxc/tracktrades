package alphavantage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"time"

	"tracktrades/internal/domain/portfolio"
)

type dailyDatum struct {
	High  float64
	Close float64
}

func (c *Client) PriceHistory(ctx context.Context, pos *portfolio.Position) ([]portfolio.PricePoint, error) {
	series, err := c.fetchDailySeries(ctx, pos)
	if err != nil {
		return nil, err
	}

	var dates []time.Time
	for d := range series {
		if pos.EntryDate.IsZero() || !d.Before(pos.EntryDate) {
			dates = append(dates, d)
		}
	}
	sort.Slice(dates, func(i, j int) bool { return dates[i].Before(dates[j]) })

	points := make([]portfolio.PricePoint, 0, len(dates))
	for _, d := range dates {
		points = append(points, portfolio.PricePoint{Date: d, Price: series[d].Close})
	}
	return points, nil
}

func (c *Client) ComputeHistoricalPeak(ctx context.Context, pos *portfolio.Position) error {
	series, err := c.fetchDailySeries(ctx, pos)
	if err != nil {
		return err
	}

	maxPrice := pos.PeakPrice
	if maxPrice < pos.CurrentPrice {
		maxPrice = pos.CurrentPrice
	}

	for d, datum := range series {
		if !pos.EntryDate.IsZero() && d.Before(pos.EntryDate) {
			continue
		}
		if datum.High > maxPrice {
			maxPrice = datum.High
		}
	}

	pos.PeakPrice = maxPrice
	pos.UpdatePrice(pos.CurrentPrice)
	return nil
}

func (c *Client) fetchDailySeries(ctx context.Context, pos *portfolio.Position) (map[time.Time]dailyDatum, error) {
	if pos.EntryDate.IsZero() {
		return nil, fmt.Errorf("entry date missing for %s", pos.Ticker)
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
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, err
	}

	var data map[string]interface{}
	_ = json.Unmarshal(body, &data)

	if note, ok := data["Note"].(string); ok {
		return nil, fmt.Errorf("API limit: %s", note)
	}

	var rawSeries map[string]interface{}
	if pos.IsCrypto() {
		rawSeries, _ = data["Time Series (Digital Currency Daily)"].(map[string]interface{})
	} else {
		rawSeries, _ = data["Time Series (Daily)"].(map[string]interface{})
	}
	if rawSeries == nil {
		return nil, fmt.Errorf("no historical series in response for %s", pos.Ticker)
	}

	series := make(map[time.Time]dailyDatum, len(rawSeries))
	for ds, raw := range rawSeries {
		day, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}

		d, err := time.Parse("2006-01-02", ds)
		if err != nil {
			continue
		}

		var highStr, closeStr string
		if pos.IsCrypto() {
			highStr, _ = day["2b. high (USD)"].(string)
			closeStr, _ = day["4b. close (USD)"].(string)
		} else {
			highStr, _ = day["2. high"].(string)
			closeStr, _ = day["4. close"].(string)
		}

		high, _ := strconv.ParseFloat(highStr, 64)
		closePrice, _ := strconv.ParseFloat(closeStr, 64)

		if high == 0 && closePrice == 0 {
			continue
		}

		series[d] = dailyDatum{High: high, Close: closePrice}
	}

	if len(series) == 0 {
		return nil, fmt.Errorf("no parsable history for %s", pos.Ticker)
	}
	return series, nil
}
