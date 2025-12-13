package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"tracktrades/internal/adapters/alphavantage"
	"tracktrades/internal/adapters/storage"
	"tracktrades/internal/app"
	"tracktrades/internal/domain/portfolio"
	"tracktrades/internal/ports"
)

const (
	defaultRepoPath   = "portfolio.json"
	defaultCash       = 0.0
	defaultTimeLayout = "2006-01-02"
)

func main() {
	log.SetFlags(0)

	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	repoSpec := os.Getenv("PORTFOLIO_STORAGE")
	if repoSpec == "" {
		repoPath := envOrDefault("PORTFOLIO_PATH", defaultRepoPath)
		repoSpec = fmt.Sprintf("file:%s", repoPath)
	}

	storeInfo, err := storage.NewPortfolioStore(repoSpec)
	if err != nil {
		log.Fatalf("invalid repository: %v", err)
	}

	apiKey := os.Getenv("ALPHAVANTAGE_API_KEY")

	portfolioName := envOrDefault("PORTFOLIO_NAME", storeInfo.DefaultPortfolio)

	store := storeInfo.Store
	pricer := selectPricer(apiKey)
	svc := app.NewPortfolioService(store, pricer)

	ctx := context.Background()

	cmd := os.Args[1]
	args := os.Args[2:]

	var cmdErr error
	switch cmd {
	case "metrics":
		cmdErr = runMetrics(ctx, svc, portfolioName, args)
	case "positions":
		cmdErr = runPositions(ctx, svc, portfolioName, args)
	case "position":
		cmdErr = runPosition(ctx, svc, portfolioName, args)
	case "add-position":
		cmdErr = runAddPosition(ctx, svc, portfolioName, args)
	case "update-prices":
		cmdErr = runUpdatePrices(ctx, svc, portfolioName, apiKey, args)
	case "recompute-peaks":
		cmdErr = runRecomputePeaks(ctx, svc, portfolioName, apiKey, args)
	case "create-portfolio":
		cmdErr = runCreatePortfolio(ctx, svc, args)
	case "list-portfolios":
		cmdErr = runListPortfolios(ctx, svc)
	case "remove-portfolio":
		cmdErr = runRemovePortfolio(ctx, svc, args)
	default:
		usage()
		os.Exit(1)
	}

	if cmdErr != nil {
		log.Fatalf("%s: %v", cmd, cmdErr)
	}
}

func runMetrics(ctx context.Context, svc *app.PortfolioService, defaultPortfolio string, args []string) error {
	fs := flag.NewFlagSet("metrics", flag.ExitOnError)
	portfolioName := fs.String("portfolio", defaultPortfolio, "Portfolio to target")
	_ = fs.Parse(args)

	metrics, err := svc.GetMetrics(ctx, *portfolioName)
	if err != nil {
		return err
	}
	return printJSON(metrics)
}

func runPositions(ctx context.Context, svc *app.PortfolioService, defaultPortfolio string, args []string) error {
	fs := flag.NewFlagSet("positions", flag.ExitOnError)
	portfolioName := fs.String("portfolio", defaultPortfolio, "Portfolio to target")
	_ = fs.Parse(args)

	positions, err := svc.ListPositions(ctx, *portfolioName)
	if err != nil {
		return err
	}
	return printJSON(positions)
}

func runPosition(ctx context.Context, svc *app.PortfolioService, defaultPortfolio string, args []string) error {
	fs := flag.NewFlagSet("position", flag.ExitOnError)
	ticker := fs.String("ticker", "", "Ticker symbol to query")
	portfolioName := fs.String("portfolio", defaultPortfolio, "Portfolio to target")
	_ = fs.Parse(args)

	if *ticker == "" {
		return errors.New("--ticker is required")
	}

	detail, ok, err := svc.GetPosition(ctx, *portfolioName, *ticker)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("position %s not found", *ticker)
	}
	return printJSON(detail)
}

func runAddPosition(ctx context.Context, svc *app.PortfolioService, defaultPortfolio string, args []string) error {
	fs := flag.NewFlagSet("add-position", flag.ExitOnError)
	ticker := fs.String("ticker", "", "Ticker symbol (required)")
	shares := fs.Float64("shares", 0, "Number of shares")
	costBasis := fs.Float64("cost", 0, "Total cost basis")
	price := fs.Float64("price", 0, "Current price")
	entryDateStr := fs.String("entry", "", "Entry date (YYYY-MM-DD)")
	portfolioName := fs.String("portfolio", defaultPortfolio, "Portfolio to target")
	_ = fs.Parse(args)

	if *ticker == "" {
		return errors.New("--ticker is required")
	}
	if *shares <= 0 {
		return errors.New("--shares must be greater than zero")
	}
	if *price <= 0 {
		return errors.New("--price must be greater than zero")
	}

	pos := &portfolio.Position{
		Ticker:       *ticker,
		Shares:       *shares,
		CostBasis:    *costBasis,
		CurrentPrice: *price,
	}

	if *entryDateStr != "" {
		t, err := time.Parse(defaultTimeLayout, *entryDateStr)
		if err != nil {
			return fmt.Errorf("invalid entry date: %w", err)
		}
		pos.EntryDate = t
	}

	pos.PeakPrice = pos.CurrentPrice
	pos.UpdatePrice(pos.CurrentPrice)

	if err := svc.AddOrUpdatePosition(ctx, *portfolioName, pos); err != nil {
		return err
	}

	fmt.Printf("position %s saved\n", pos.Ticker)
	return nil
}

func runUpdatePrices(ctx context.Context, svc *app.PortfolioService, defaultPortfolio, apiKey string, args []string) error {
	if err := requireAPIKey(apiKey); err != nil {
		return err
	}
	fs := flag.NewFlagSet("update-prices", flag.ExitOnError)
	portfolioName := fs.String("portfolio", defaultPortfolio, "Portfolio to target")
	_ = fs.Parse(args)

	return svc.UpdateAllPrices(ctx, *portfolioName)
}

func runRecomputePeaks(ctx context.Context, svc *app.PortfolioService, defaultPortfolio, apiKey string, args []string) error {
	if err := requireAPIKey(apiKey); err != nil {
		return err
	}
	fs := flag.NewFlagSet("recompute-peaks", flag.ExitOnError)
	portfolioName := fs.String("portfolio", defaultPortfolio, "Portfolio to target")
	_ = fs.Parse(args)

	return svc.RecomputeHistoricalPeaks(ctx, *portfolioName)
}

func runCreatePortfolio(ctx context.Context, svc *app.PortfolioService, args []string) error {
	fs := flag.NewFlagSet("create-portfolio", flag.ExitOnError)
	name := fs.String("name", "", "Portfolio name")
	cash := fs.Float64("cash", defaultCash, "Starting cash")
	_ = fs.Parse(args)

	if *name == "" {
		return errors.New("--name is required")
	}

	_, err := svc.CreatePortfolio(ctx, *name, *cash)
	if err != nil {
		return err
	}
	fmt.Printf("portfolio %s created\n", *name)
	return nil
}

func runListPortfolios(ctx context.Context, svc *app.PortfolioService) error {
	names, err := svc.ListPortfolios(ctx)
	if err != nil {
		return err
	}
	return printJSON(map[string][]string{"portfolios": names})
}

func runRemovePortfolio(ctx context.Context, svc *app.PortfolioService, args []string) error {
	fs := flag.NewFlagSet("remove-portfolio", flag.ExitOnError)
	name := fs.String("name", "", "Portfolio name")
	_ = fs.Parse(args)

	if *name == "" {
		return errors.New("--name is required")
	}
	if err := svc.RemovePortfolio(ctx, *name); err != nil {
		return err
	}
	fmt.Printf("portfolio %s removed\n", *name)
	return nil
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

type noopPricer struct{}

func (noopPricer) UpdatePrice(ctx context.Context, pos *portfolio.Position) error { return nil }
func (noopPricer) ComputeHistoricalPeak(ctx context.Context, pos *portfolio.Position) error {
	return nil
}

func selectPricer(apiKey string) ports.PriceProvider {
	if apiKey == "" {
		return noopPricer{}
	}
	return alphavantage.New(apiKey)
}

func requireAPIKey(apiKey string) error {
	if apiKey == "" {
		return errors.New("set ALPHAVANTAGE_API_KEY for this command")
	}
	return nil
}

func usage() {
	fmt.Fprintln(os.Stderr, "Portfolio CLI")
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  metrics [--portfolio NAME]                    Show portfolio metrics")
	fmt.Fprintln(os.Stderr, "  positions [--portfolio NAME]                  List all positions")
	fmt.Fprintln(os.Stderr, "  position --ticker TICKER [--portfolio NAME]   Show a single position")
	fmt.Fprintln(os.Stderr, "  add-position --ticker T --shares N --price P [--cost C] [--entry YYYY-MM-DD] [--portfolio NAME]")
	fmt.Fprintln(os.Stderr, "                                                Add or update a position")
	fmt.Fprintln(os.Stderr, "  update-prices [--portfolio NAME]              Refresh prices via AlphaVantage (requires ALPHAVANTAGE_API_KEY)")
	fmt.Fprintln(os.Stderr, "  recompute-peaks [--portfolio NAME]            Recompute historical peaks (requires ALPHAVANTAGE_API_KEY)")
	fmt.Fprintln(os.Stderr, "  create-portfolio --name NAME [--cash AMOUNT]  Create a new portfolio")
	fmt.Fprintln(os.Stderr, "  list-portfolios                               List existing portfolios")
	fmt.Fprintln(os.Stderr, "  remove-portfolio --name NAME                  Delete a portfolio file")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "Environment variables:\n  PORTFOLIO_PATH (default %s)\n  PORTFOLIO_NAME (default derived from PORTFOLIO_PATH)\n  ALPHAVANTAGE_API_KEY\n", defaultRepoPath)
}
