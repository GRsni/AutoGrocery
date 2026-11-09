# autoGrocery

[![Go Coverage](https://github.com/GRsni/AutoGrocery/wiki/coverage.svg)](https://raw.githack.com/wiki/GRsni/AutoGrocery/coverage.html)

A Go CLI application that scrapes grocery receipts from Spanish supermarkets (Mercadona, Carrefour, Dia), parses them, and stores the data in a Google Spreadsheet.

## Overview

**autoGrocery** automates receipt collection from three Spanish supermarket chains:

| Store | Method | Notes |
|---|---|---|
| **Mercadona** | Gmail API → PDF parsing | Reads emails with a configured label, extracts PDF attachments, parses with `ledongthuc/pdf` |
| **Dia** | Browser automation (Rod + stealth) | Uses session cookies from `config/credentials/cookies-www-dia-es.txt`; fallback to username/password login |
| **Carrefour** | Browser automation (Rod) | Loads credentials from `secrets.json`, handles Gigya 2FA via Gmail |

All parsed receipts are uploaded to a Google Spreadsheet row-by-row, with deduplication against existing entries.

---

## Architecture

```
cmd/autogrocery/main.go          ← Entry point. Orchestrates the full pipeline.
internal/                          ← Core domain types
  ticket.go                        ← Ticket, Item structs + ToValueRange, IsExcluded
  exclusion.go                     ← Excluded struct + LoadExcluded()
internal/carrefour/carrefour.go   ← Carrefour browser automation + ticket scraping
internal/dia/dia.go               ← Dia browser automation + ticket scraping
internal/mercadona/mercadona.go   ← Mercadona Gmail PDF parsing (no browser needed)
internal/google/gm/gm.go          ← Gmail API wrapper (fetch messages, attachments)
internal/google/sh/sheets.go      ← Google Sheets API wrapper (read/write entries)
internal/token/manager.go         ← OAuth2 token lifecycle (web flow, refresh, save)
pkg/constants/                     ← Shared constants (store names, month names, date format)
utils/utils.go                     ← Float parsing, price/qty extraction, date helpers
config/                            ← Credentials, cookies, excluded IDs (gitignored)
docker/                            ← Dockerfile, compose files (.dockerignore)
scripts/
  full-flow.py                     ← Playwright cookie refresh + Go service runner
systemd/                           ← autoGrocery.service + .timer for systemd scheduling
deploy/deploy.sh                   ← Cross-compile Docker image, transfer to Pi, load via systemd
```

### Data Flow

1. **Authenticate** with Google (OAuth2 → `config/credentials/token.json`)
2. **Read existing entries** from Google Sheets (range `A2:F300`)
3. **Fetch new tickets** from each store concurrently (3 goroutines):
   - **Mercadona**: Query Gmail label → download PDF attachment → parse with `ledongthuc/pdf`
   - **Dia**: Browser login (Rod + stealth) with session cookies → scrape account page
   - **Carrefour**: Browser login (Rod) with username/password + 2FA handling → scrape account page
4. **Deduplicate** against existing sheet entries (by date + total per store)
5. **Exclude** known ticket IDs from `config/excluded.json`
6. **Upload** new tickets to Google Sheets row-by-row

---

## Key Types

### `internal.Ticket`

```go
type Ticket struct {
    Items Items
    Id    string
    Total float64
    Store string   // constants.MERCADONA / DIA / CARREFOUR
    Date  time.Time
}
type Item struct {
    Name      string
    Amount    float64   // quantity (supports decimals for kg/units)
    Price     float64   // unit price
    Discount  float64
    Cancelled bool
}
```

### `internal.Excluded`

Loaded from `config/excluded.json`. Ticket IDs to skip per store.

### Sheets Integration

The `Manager` wraps a `SheetsAPI` interface, making it testable. `realService` implements it with the real Google API.

---

## Configuration Files

| File | Purpose |
|---|---|
| `config/credentials/secrets.json` | Google OAuth2 client ID/secret + Sheets main ID + store credentials (Dia username/password, Carrefour password) |
| `config/credentials/token.json` | OAuth2 access/refresh tokens (auto-generated, mode 0600) |
| `config/credentials/cookies-www-dia-es.txt` | Netscape cookie format for Dia session (~55 min TTL) |
| `config/excluded.json` | Ticket IDs to skip per store |

---

## CLI Interface

```bash
./autogrocery                          # Run with current month defaults
./autogrocery -start DD-MM-YYYY        # Override start date
./autogrocery -version                 # Show build version
```

**Build flags:**
```bash
go build -ldflags "-X main.Version=v0.0.1 -X main.BuildDate=2025-01-01T00:00:00Z" ./cmd/autogrocery/
```

---

## Cookie Refresh Workflow

### `scripts/full-flow.py`

A Python script using `invisible_playwright` that:

1. Checks if existing Dia cookies are fresh (< 55 min old) — skips login if so
2. Logs in to `dia.es/login` with credentials from `secrets.json`
3. Handles multi-step login (email → password → Continue button clicks)
4. Waits for manual intervention if needed (CAPTCHA, 2FA)
5. Exports cookies in Netscape format to `config/credentials/cookies-www-dia-es.txt`
6. Runs the Go service to export tickets

**Usage:**
```bash
python scripts/full-flow.py
```

---

## Docker Deployment

### Local Development

```bash
# Build and run locally
docker compose run --rm autogrocery

# Or with debug config
docker compose -f docker/docker-compose.debug.yml up
```

The Docker image is a multi-stage build:
1. **Go builder stage** — compiles the Go binary on `golang:1.26-bookworm`
2. **Final stage** — `python:3.12-slim-bookworm` with Chromium, Playwright browsers, and the Go binary

### Production (Raspberry Pi)

The app runs on a Raspberry Pi via systemd timer (daily at 3:00 AM):

```bash
# Install systemd unit files manually
sudo cp systemd/autogrocery.service /etc/systemd/system/
sudo cp systemd/autogrocery.timer /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now autogrocery.timer
```

The Docker container mounts credentials as read-only volumes from a persistent config directory on the host.

---

## Deploy Script

Builds a cross-compiled ARM64 Docker image, transfers it to the Raspberry Pi, loads it, and sets up systemd.

```bash
# Usage: ./deploy/deploy.sh <user@host> [tag] [remote-dest] [--force]
./deploy/deploy.sh user@192.168.1.x                          # Normal deploy (defaults to ~/autogrocery/ on remote)
./deploy/deploy.sh user@192.168.1.x autogrocery:v1.2         # Custom tag
./deploy/deploy.sh user@192.168.1.x --force                  # Force rebuild even if source unchanged
./deploy/deploy.sh user@192.168.1.x autogrocery:latest /mnt/data  # Custom remote path
```

To check logs of previous runs, SSH into PI, then run:

```bash
journalctl --unit=autogrocery.service -n 200
```

### Features

- **Source caching**: Hashes all tracked files (`git ls-files`) and compares to `.deploy-cache`. Skips the build if nothing changed.
- **Cache written before building**: Survives partial failures — if SCP or transfer fails, retries skip the expensive build step.
- **Remote disk-space check**: Verifies available space at destination path before transferring (~920 MB tar).
- **SCP retry**: 3 attempts with 5s between to handle mid-transfer failures.
- **Systemd auto-setup**: Generates and uploads `.service`/`.timer` files dynamically (injects `$REMOTE_USER` paths).

---

## Authentication Flow

1. Load token from `config/credentials/token.json`
2. If missing/corrupt → open browser → prompt user for auth code
3. If expired → attempt `refresh_token` → if that fails → force web flow
4. Save token to file (mode 0600)

**Scopes:** `sheets.SpreadsheetsScope` + `gmail.MailGoogleComScope`

---

## Development Workflow

### Build & Run Locally

```bash
# Build
go build -o autogrocery ./cmd/autogrocery

# Run
./autogrocery
```

### Run Cookie Refresh + Tickets

```bash
python scripts/full-flow.py
```

This handles cookie refresh (if stale) then runs the Go binary.

### Tests

```bash
go test ./...          # All tests
go test ./internal/    # Internal packages only
```

**Test coverage areas:**
- `internal/ticket_test.go` — `ToValueRange()`, `IsExcluded()`, total validation, entry conversion
- `internal/dia/dia_test.go` — `LoadCredentials()`, cookie parsing, date/total extraction
- `internal/token/manager_test.go` — token load/save with edge cases
- `internal/google/sh/sheets_test.go` — Sheets API wrapper
- `utils/utils_test.go` — Float/price/qty/date parsing functions

**Test Mode:** Set `constants.TestMode = true` to use test sheet pages.

---

## CI/CD (`.github/workflows/go-build-test.yml`)

**Jobs:**
1. **build-and-test** — Checkout, verify `go.sum`, build, test, coverage report (push only)
2. **release** — Semantic versioning based on tag count, builds ARM64 binary for Raspberry Pi, creates GitHub Release

**Triggers:** Push and PR to `develop` branch.

---

## Important Patterns & Conventions

1. **Cleanup pattern**: Browser login functions return `(*rod.Page, func(), error)` — the cleanup func must always be called.
2. **Concurrency**: Store fetching runs in parallel via `sync.WaitGroup`.
3. **Deduplication**: Compares new tickets against existing sheet entries by (store, date, total).
4. **Row-by-row upload**: Tickets are uploaded one item at a time, tracking `lastWrittenRow`.
5. **Plus items**: Item names starting with `+` are escaped with `'` in Sheets to prevent formula injection.
6. **Human delays**: `humanDelay()` adds 600-1200ms random sleep between browser actions to avoid detection.
7. **Logging**: Uses `log/slog` with Debug level. Store context added via `"STORE", constants.XXX`.
8. **Headless mode**: Controlled by `constants.HeadlessMode` boolean.

---

## Dependencies

| Package | Purpose |
|---|---|
| `github.com/go-rod/rod` | Browser automation (Carrefour, Dia) |
| `github.com/go-rod/stealth` | Stealth mode for Rod (Dia) |
| `github.com/ledongthuc/pdf` | PDF parsing (Mercadona receipts) |
| `golang.org/x/oauth2` | OAuth2 authentication |
| `google.golang.org/api` | Gmail + Sheets API clients |
| `playwright` / `invisible-playwright` | Cookie refresh for Dia (Python) |

---

## Security Notes

- Credentials stored in `config/credentials/secrets.json` — never commit this file
- Token file has 0600 permissions
- Application uses read-only Sheets scope + full Gmail scope (for 2FA codes)
- Browser automation runs headless with stealth settings
- Cookie files contain session data — handle with care
