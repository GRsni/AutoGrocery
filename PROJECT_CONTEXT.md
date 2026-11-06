# AutoGrocery Project Context

## Overview

**autoGrocery** is a Go CLI application that scrapes grocery receipts from Spanish supermarkets (Mercadona, Carrefour, Dia), parses them, and stores the data in a Google Spreadsheet. It uses Gmail to receive Mercadona PDF receipts, browser automation (Rod) for Carrefour and Dia web portals, and the Google Sheets/Gmail APIs for data integration.

**Repository:** `github.com/GRsni/AutoGrocery`
**Module:** `autoGrocery`
**Go Version:** 1.26.0

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
```

### Data Flow

1. **Authenticate** with Google (OAuth2 → token.json)
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
type Items []Item
```

### `internal.Excluded`
```go
type Excluded struct {
    DiaIDs       []string
    CarrefourIds []string
    MercadonaIds []string
}
```
Loaded from `config/excluded.json`. Used to skip known tickets.

### `sh.Entry` (Google Sheets representation)
```go
type Entry struct {
    Date     time.Time
    Id       string
    Store    string
    FirstRow int
    LastRow  int
    Total    float64
}
```

### `sh.Manager` & Interfaces
```go
type SheetsAPI interface {
    Get(spreadsheetID, readRange string) (*sheets.ValueRange, error)
    Update(spreadsheetID, writeRange string, vr *sheets.ValueRange, inputOption string) (*sheets.UpdateValuesResponse, error)
    BatchUpdate(spreadsheetID string, req *sheets.BatchUpdateSpreadsheetRequest) (*sheets.BatchUpdateSpreadsheetResponse, error)
}
```
The `Manager` wraps a `SheetsAPI` interface, making it testable. `realService` implements it with the real Google API.

### `gm.Manager`
```go
type Manager struct {
    Service *gmail.Service
}
```
Wraps Gmail API for fetching messages and attachments.

---

## Store Implementations

### Mercadona (`internal/mercadona/mercadona.go`)
- **Source**: Gmail label `Label_2031551581397134603`
- **Format**: Emails contain PDF attachments named like `20240415_Ticket_XX.XX.pdf`
- **Parsing**: Uses `github.com/ledongthuc/pdf` to extract text from PDF pages
- **Validation**: Verifies item totals match ticket total before accepting
- **No browser needed** — purely API-based

### Dia (`internal/dia/dia.go`)
- **Source**: Browser automation via `github.com/go-rod/rod` + `github.com/go-rod/stealth`
- **Auth**: Loads session cookies from `config/credentials/cookies-www-dia-es.txt` (Netscape format)
- **Cookie format**: Tab-separated: `Domain\tSecure\tPath\tHttpOnly\tExpiry\tName\tValue`
- **Fallback**: Can also do full username/password login if cookies expire
- **Chrome userData**: Uses `CHROME_USER_DATA_DIR` env var (default `/home/user/.config/google-chrome`)

### Carrefour (`internal/carrefour/carrefour.go`)
- **Source**: Browser automation via Rod (no stealth needed)
- **Auth**: Loads username/password from `config/credentials/secrets.json` under `carrefour` key
- **2FA**: Handles Gigya 2FA modal — fetches code from Gmail
- **Gigya**: Login uses Gigya hosted forms inside iframes (`/access`)
- **2FA Label ID**: `Label_9053428003199955812`

---

## Constants (`pkg/constants/`)

```go
const CARREFOUR = "CARREFOUR"
const DIA = "DIA"
const MERCADONA = "MERCADONA"
const TicketDateFormat = "02/01/2006"  // DD/MM/YYYY

// Spanish months (Enero, Febrero, ..., Diciembre)
func FromTimeMonth(m time.Month) SpanishMonth
```

Sheet page naming: `<MonthName><YY>` e.g., `Noviembre26`

---

## Utilities (`utils/utils.go`)

| Function | Purpose |
|---|---|
| `FloatsEqual(a, b)` | Compare floats within epsilon (1e-5) |
| `ToFixed(num, precision)` | Round to N decimal places |
| `StringToFloat(str, precision)` | Parse string → float (handles comma decimals) |
| `ParseQty(qty)` | Strip "ud." / "kg" → float |
| `ParseQtyWithPrecision(qty, prec)` | Same with configurable precision |
| `ParsePrice(price)` | Strip "€" / "/kg" → float |
| `ExtractString(val any)` | Safely extract string from `any` (handles float64 from Sheets) |
| `StringToDate(input)` | Parse date with fallback formats |

---

## Authentication (`internal/token/manager.go`)

**OAuth2 Flow:**
1. Load token from `config/credentials/token.json`
2. If missing/corrupt → open browser → prompt user for auth code
3. If expired → attempt refresh_token → if that fails → force web flow
4. Save token to file (mode 0600)

**Config:** Loaded from `config/credentials/secrets.json` (Google OAuth JSON)
**Scopes:** `sheets.SpreadsheetsScope` + `gmail.MailGoogleComScope`

---

## Configuration Files

| File | Purpose |
|---|---|
| `config/credentials/secrets.json` | Google OAuth2 client ID/secret + Sheets main ID |
| `config/credentials/token.json` | OAuth2 access/refresh tokens (auto-generated) |
| `config/credentials/cookies-www-dia-es.txt` | Netscape cookie format for Dia session |
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

## Testing

```bash
go test ./...          # Run all tests
go test ./internal/    # Run internal package tests
```

**Test coverage areas:**
- `internal/exclusion_test.go` — `LoadExcluded()` with valid/missing/invalid/empty JSON
- `internal/ticket_test.go` — `Ticket.ToValueRange()`, `Items.String()`, `IsExcluded()`, total validation
- `internal/dia/dia_test.go` — `LoadCredentials()`, `LoadSessionFromCookies()`, date/total parsing
- `internal/token/manager_test.go` — `tokenFromFile()`, `saveToken()` with various edge cases
- `utils/utils_test.go` — All float/price/qty parsing functions

**Test Mode:** Set `constants.TestMode = true` to use test sheet pages.

---

## CI/CD (`.github/workflows/go-build-test.yml`)

**Jobs:**
1. **build-and-test** — Checkout, verify `go.sum`, build, test, coverage report (push only)
2. **release** — Semantic versioning based on tag count, builds ARM64 binary for Raspberry Pi, creates GitHub Release

**Triggers:** Push and PR to `develop` branch.

---

## Dependencies

| Package | Purpose |
|---|---|
| `github.com/go-rod/rod` | Browser automation (Carrefour, Dia) |
| `github.com/go-rod/stealth` | Stealth mode for Rod (Dia) |
| `github.com/ledongthuc/pdf` | PDF parsing (Mercadona receipts) |
| `golang.org/x/oauth2` | OAuth2 authentication |
| `google.golang.org/api` | Gmail + Sheets API clients |

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

## Security Notes

- Credentials are stored in `config/credentials/secrets.json` — never commit this file
- Token file has 0600 permissions
- Application uses read-only Sheets scope + full Gmail scope (for 2FA codes)
- Browser automation runs headless with stealth settings
