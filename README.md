# tracktrades

HTTP API, CLI, and service layer for tracking portfolio performance with clean domain/ports/adapters boundaries.

## Features
- Domain-driven layout with separate domain models, ports (interfaces), and adapters.
- File-based repository for persisting portfolios.
- AlphaVantage adapter to refresh live prices and compute historical peaks.
- Service layer exposes portfolio metrics, per-position performance, and recovery percentages after drawdowns.
- HTTP API with endpoints for metrics, positions, peak recomputation, and price updates.
- Local CLI for querying metrics, listing or viewing positions, adding positions, and triggering price/peak refreshes.

## Getting started
1. Ensure Go 1.22+ is installed.
2. Set an AlphaVantage API key in `cmd/api/main.go` (replace `YOUR_ALPHA_VANTAGE_KEY`).
3. Build and run the API server:
   ```bash
   go run ./cmd/api
   ```
4. Use `curl` to interact (set `PORTFOLIO_NAME` to pick a different portfolio file name, default `portfolio`):
   ```bash
   curl "http://localhost:8080/portfolio?portfolio=portfolio"
   curl "http://localhost:8080/positions?portfolio=portfolio"
   curl -X POST "http://localhost:8080/update-prices?portfolio=portfolio"
   ```

The server persists data in `<portfolio name>.json` using the file repository adapter. Files are only created when you call the create-portfolio CLI command (or write your own creation flow); the API no longer creates them implicitly.

### CLI usage
1. Set optional environment variables:
   - `PORTFOLIO_PATH` to point at an alternate portfolio file (default `portfolio.json`).
   - `ALPHAVANTAGE_API_KEY` when using commands that hit AlphaVantage (`update-prices`, `recompute-peaks`).
2. Run commands:
   ```bash
   go run ./cmd/cli metrics
   go run ./cmd/cli positions
   go run ./cmd/cli position --ticker NVDA
   go run ./cmd/cli add-position --ticker NVDA --shares 10 --price 120 --cost 1200 --entry 2024-01-02
   go run ./cmd/cli update-prices
   go run ./cmd/cli recompute-peaks
   go run ./cmd/cli create-portfolio --name swing --cash 2500
   go run ./cmd/cli list-portfolios
   go run ./cmd/cli metrics --portfolio swing
   ```
   Commands render JSON to stdout and exit non-zero on errors.

## Architecture
- **Domain**: `internal/domain/portfolio` holds entities and metric calculations.
- **Ports**: `internal/ports` defines repository and price provider interfaces.
- **Adapters**: `internal/adapters/storage` provides file-backed persistence; `internal/adapters/alphavantage` integrates with AlphaVantage for quotes and historical peaks.
- **Service layer**: `internal/app` orchestrates repositories and price providers.
- **Entrypoint**: `cmd/api` hosts the HTTP server wiring all components together.

## Tests
Run unit tests for recovery math, position metrics, and service behavior:

```bash
go test ./...
```
