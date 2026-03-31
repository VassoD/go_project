# VTC Driver Revenue Microservice

A Go microservice that ingests ride data from VTC platforms (Uber, Bolt, Heetch) and calculates net driver payouts.

## Quick start

With Go installed: `go run .`
> Requires Go 1.22+.

With Docker: `make docker-build && make docker-run`

The service starts on `http://localhost:8080`.

## API

### POST /ingest

Upload one or more JSON or CSV files as multipart form data.

Accepted trip schema:
- `driver_id`: driver identifier
- `timestamp`: trip date/time
- `amount`: trip amount in euros

The original exercise statement mixes French business wording (`montant`) with the API description. This implementation uses English field names consistently across JSON, CSV, and API responses, so `montant` is represented as `amount` in input files.

Ingestion is best-effort:
- valid rows are stored even if some rows in the same file are invalid
- invalid rows are skipped and returned in the `warnings` field
- the endpoint returns `400` only when no valid trip was ingested at all

```bash
curl -X POST http://localhost:8080/ingest \
  -F "files=@data/trips_uber.json" \
  -F "files=@data/trips_bolt.csv" \
  -F "files=@data/trips_heetch.csv"
```

> **Note on re-ingestion**: there is no deduplication — calling `/ingest` twice with the same file stores the trips twice.

The files `data/trips_uber.json`, `data/trips_bolt.csv`, and `data/trips_heetch.csv` are valid happy-path examples.
To demonstrate partial ingestion with warnings, use `data/trips_corrupted.csv`, which intentionally mixes valid and invalid rows:

```bash
curl -X POST http://localhost:8080/ingest \
  -F "files=@data/trips_corrupted.csv"
```

### GET /balances

Returns the net payout for a driver over a period.

```bash
curl "http://localhost:8080/balances?driver_id=driver-1&period=monthly"
```

Parameters:
- `driver_id` (required): the driver's ID
- `period` (required): `daily`, `weekly`, or `monthly`

If the driver has no trips in the requested period — or the `driver_id` is unknown — the endpoint returns `200` with all amounts set to `0`. Both cases are indistinguishable by design: a zero payout means nothing to pay out, regardless of whether the driver exists.

### GET /health

```bash
curl http://localhost:8080/health
# {"status":"ok"}
```

## Running tests

```bash
make test
# or verbose:
make test-verbose
```

## Architecture

### Structure

```
vtc-service/
├── main.go                   # Entry point, HTTP routing
├── internal/
│   ├── model/                # Data types (Trip, Balance)
│   ├── store/                # In-memory storage layer
│   ├── calculator/           # Financial calculation logic
│   └── handler/              # HTTP handlers (ingest, balance)
├── data/                     # Sample data files
├── Dockerfile
└── Makefile
```

### Storage choice: in-memory

Trips are stored in a thread-safe in-memory slice. Rationale:
- **Simplicity**: no external database to set up, instant startup.
- **Test isolation**: each test creates a fresh store.
- **Trade-off**: data is lost on restart. For production, swap the store for PostgreSQL or SQLite

### Period definitions

The spec defines `daily`, `weekly`, and `monthly` but does not clarify whether these mean "current calendar period" or "last N days". We chose **current calendar period**:

- `daily` -> from midnight today (Paris time) until now
- `weekly` -> from Monday midnight of the current ISO week until now
- `monthly` -> from the 1st of the current month until now

Why calendar periods over rolling windows (last 24h, last 7 days, last 30 days):
- Drivers and accountants think in calendar terms — "how much did I earn this month" means March, not the last 30 days
- Payroll is processed monthly on calendar months, not rolling windows
- It also avoids the ambiguity of month length (30 vs 31 vs 28 days) in a rolling window

Future-dated trips (timestamps after now) are excluded even if they fall within the period window, to avoid counting trips that haven't happened yet.

### Calculation logic

The test specifies three deductions (15% commission, 20% VAT, 20% Urssaf) but does not define the exact base for each. The following is a simplified model assumed for this exercise:

- **Commission** = `gross x 15%` — deducted first, it's the platform's cut.
- **Net after commission** = `gross - commission` — the base for the remaining deductions.
- **VAT** = `net x 20%` — applied to the net after commission.
- **Urssaf** = `net x 20%` — applied to the same base, independently.
- **Net payout** = `net - VAT - Urssaf`

Each intermediate value is rounded to 2 decimal places before being used in the next step, so all fields in the response are self-consistent (`net_payout` always equals `net_after_commission - vat - urssaf` as displayed).

### Test strategy

- **Unit tests** (`internal/calculator/`): pure function tests with table-driven cases covering normal values, edge cases (zero), and boundary values.
- **Integration tests** (`internal/handler/`): test full HTTP handler behavior using `httptest` - no real server needed, no mocks of the store. These verify parsing, routing, validation, and response format together.
- **Period windowing** (`internal/handler/period_test.go`): the `currentTime` function is overridable, allowing deterministic tests for daily, weekly, and monthly boundaries, including edge cases like leap years and short months — without a clock interface.
