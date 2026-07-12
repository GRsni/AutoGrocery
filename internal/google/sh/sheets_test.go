package sh

import (
	"autoGrocery/pkg/constants"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"autoGrocery/utils"

	"google.golang.org/api/googleapi"
	"google.golang.org/api/sheets/v4"
)

// ---------------------------------------------------------------------------
// Fake Sheets API
// ---------------------------------------------------------------------------

// valuesGetCall and valuesUpdateCall mirror the chained call pattern
// (.Get(...).Do(), .Update(...).Do()) without hitting the network.

type fakeValuesGetCall struct {
	resp *sheets.ValueRange
	err  error
}

func (f *fakeValuesGetCall) Do(...googleapi.CallOption) (*sheets.ValueRange, error) {
	return f.resp, f.err
}

type fakeValuesUpdateCall struct {
	resp *sheets.UpdateValuesResponse
	err  error
}

func (f *fakeValuesUpdateCall) ValueInputOption(opt string) *fakeValuesUpdateCall { return f }
func (f *fakeValuesUpdateCall) Do(...googleapi.CallOption) (*sheets.UpdateValuesResponse, error) {
	return f.resp, f.err
}

// SheetsAPI covers the two Values sub-calls we use.
type FakeSheetsAPI interface {
	Get(spreadsheetID, readRange string) ValuesGetCall
	Update(spreadsheetID, writeRange string, vr *sheets.ValueRange) ValuesUpdateCall
	BatchUpdate(spreadsheetID string, req *sheets.BatchUpdateSpreadsheetRequest) (*sheets.BatchUpdateSpreadsheetResponse, error)
}

type FakeValuesGetCall interface {
	Do(...googleapi.CallOption) (*sheets.ValueRange, error)
}

type FakeValuesUpdateCall interface {
	ValueInputOption(string) ValuesUpdateCall
	Do(...googleapi.CallOption) (*sheets.UpdateValuesResponse, error)
}

// fakeValues implements SheetsAPI.
type fakeValues struct {
	getResp         *sheets.ValueRange
	getErr          error
	updateResp      *sheets.UpdateValuesResponse
	updateErr       error
	batchUpdateResp *sheets.BatchUpdateSpreadsheetResponse
	batchUpdateErr  error
}

func (f *fakeValues) Get(_, _ string) (*sheets.ValueRange, error) {
	return f.getResp, f.getErr
}

func (f *fakeValues) Update(_, _ string, _ *sheets.ValueRange, _ string) (*sheets.UpdateValuesResponse, error) {
	return f.updateResp, f.updateErr
}

func (f *fakeValues) BatchUpdate(_ string, _ *sheets.BatchUpdateSpreadsheetRequest) (*sheets.BatchUpdateSpreadsheetResponse, error) {
	return f.batchUpdateResp, f.batchUpdateErr
}

type fakeGetCall struct {
	resp *sheets.ValueRange
	err  error
}

func (c *fakeGetCall) Do(...googleapi.CallOption) (*sheets.ValueRange, error) {
	return c.resp, c.err
}

type fakeUpdateCall struct {
	resp *sheets.UpdateValuesResponse
	err  error
}

func (c *fakeUpdateCall) ValueInputOption(_ string) ValuesUpdateCall { return c }
func (c *fakeUpdateCall) Do(...googleapi.CallOption) (*sheets.UpdateValuesResponse, error) {
	return c.resp, c.err
}

// testManager builds a Manager wired to the fake values API.
func testManager(fv SheetsAPI) Manager {
	return Manager{
		PageName: "Sheet1",
		config: Config{Sheets: struct {
			MainID string `json:"main-id"`
		}{MainID: "spreadsheet-id"}},
		service: fv, // injected mock service
	}
}

// testEntry creates an Entry with specified values for testing
func testEntry(date time.Time, store string, total float64) Entry {
	return Entry{Date: date, Store: store, Total: total}
}

// ---------------------------------------------------------------------------
// EntryToStr
// ---------------------------------------------------------------------------

func TestEntryToStr(t *testing.T) {
	d := func(day, month, year int) time.Time {
		return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	}

	tests := []struct {
		name     string
		entry    Entry
		expected string
	}{
		{
			name:     "normal entry",
			entry:    Entry{Store: "MERCADONA", Date: d(15, 4, 2024)},
			expected: fmt.Sprintf("Grocery Ticket: Shop=MERCADONA, Date=%s\n", d(15, 4, 2024).Format(constants.TicketDateFormat)),
		},
		{
			name:     "different store",
			entry:    Entry{Store: "DIA", Date: d(1, 1, 2023)},
			expected: fmt.Sprintf("Grocery Ticket: Shop=DIA, Date=%s\n", d(1, 1, 2023).Format(constants.TicketDateFormat)),
		},
		{
			name:     "zero date",
			entry:    Entry{Store: "CARREFOUR", Date: time.Time{}},
			expected: fmt.Sprintf("Grocery Ticket: Shop=CARREFOUR, Date=%s\n", time.Time{}.Format(constants.TicketDateFormat)),
		},
		{
			name:     "empty store",
			entry:    Entry{Store: "", Date: d(10, 6, 2024)},
			expected: fmt.Sprintf("Grocery Ticket: Shop=, Date=%s\n", d(10, 6, 2024).Format(constants.TicketDateFormat)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EntryToStr(tt.entry)
			if got != tt.expected {
				t.Errorf("got %q, want %q", got, tt.expected)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// GetLastEntryForStore
// ---------------------------------------------------------------------------

func TestGetLastEntryForStore(t *testing.T) {
	d := func(day int) time.Time { return time.Date(2024, 1, day, 0, 0, 0, 0, time.UTC) }

	tickets := []Entry{
		{Store: "MERCADONA", Date: d(1), FirstRow: 1},
		{Store: "DIA", Date: d(2), FirstRow: 5},
		{Store: "MERCADONA", Date: d(3), FirstRow: 9},
		{Store: "DIA", Date: d(4), FirstRow: 13},
	}

	tests := []struct {
		name      string
		tickets   []Entry
		shop      string
		wantStore string
		wantRow   int
	}{
		{"returns last MERCADONA", tickets, "MERCADONA", "MERCADONA", 9},
		{"returns last DIA", tickets, "DIA", "DIA", 13},
		{"store not present", tickets, "CARREFOUR", "", -1},
		{"empty slice", []Entry{}, "MERCADONA", "", 0},
		{"single matching entry", tickets[:1], "MERCADONA", "MERCADONA", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetLastEntryForStore(tt.tickets, tt.shop)
			if tt.wantRow == -1 && got != nil {
				t.Errorf("Ticket = %v, want nil", got)
			}
			if got != nil {
				if got.Store != tt.wantStore {
					t.Errorf("Store = %q, want %q", got.Store, tt.wantStore)
				}
				if got.FirstRow != tt.wantRow {
					if got != nil {
						t.Errorf("FirstRow = %d, want %d", got.FirstRow, tt.wantRow)
					}
				}
			}
		})
	}

}

// ---------------------------------------------------------------------------
// GetSheetTicketList (requires fake API)
// ---------------------------------------------------------------------------

func TestGetSheetTicketList(t *testing.T) {
	tests := []struct {
		name       string
		rows       [][]any
		wantLen    int
		wantStores []string
		wantTotals []float64
	}{
		{
			name: "two valid tickets with separate TOTAL rows",
			rows: [][]any{
				{"15/04/2024", "MERCADONA", "", "", "", ""}, // Row 0: Date + Store header
				{"item1", "2.99", "", "", "", ""},           // Row 1: Item
				{"", "", "", "", "TOTAL", "45.50 €"},        // Row 2: Total
				{"01/05/2024", "DIA", "", "", "", ""},       // Row 3: Date + Store header
				{"item1", "1.50", "", "", "", ""},           // Row 4: Item
				{"", "", "", "", "TOTAL", "23.75 €"},        // Row 5: Total
			},
			wantLen:    2,
			wantStores: []string{"MERCADONA", "DIA"},
			wantTotals: []float64{45.50, 23.75},
		},
		{
			name:       "empty sheet",
			rows:       [][]any{},
			wantLen:    0,
			wantStores: []string{},
		},
		{
			name: "sheet with empty row marker (0,00 €) at end",
			rows: [][]any{
				{"15/04/2024", "MERCADONA", "", "", "", ""},
				{"item1", "2.99", "", "", "", ""},
				{"", "", "", "", "TOTAL", "45.50 €"},
				{"", "", "", "", "", "0,00 €"},
			},
			wantLen:    1,
			wantStores: []string{"MERCADONA"},
			wantTotals: []float64{45.50},
		},
		{
			name: "multiple items before TOTAL",
			rows: [][]any{
				{"20/06/2024", "CARREFOUR", "", "", "", ""},
				{"item1", "5.00", "", "", "", ""},
				{"item2", "10.00", "", "", "", ""},
				{"", "", "", "", "TOTAL", "150.00 €"},
			},
			wantLen:    1,
			wantStores: []string{"CARREFOUR"},
			wantTotals: []float64{150.00},
		},
		{
			name: "float64 date value (Sheets number format)",
			rows: [][]any{
				{float64(20240415), "MERCADONA", "", "", "", ""},
				{"item1", "2.99", "", "", "", ""},
				{"", "", "", "", "TOTAL", "45.50 €"},
			},
			wantLen:    1,
			wantStores: []string{"MERCADONA"},
			wantTotals: []float64{45.50},
		},
		{
			name: "ticket with empty date (should be skipped)",
			rows: [][]any{
				{"", "", "", "", "", "0,00 €"},
			},
			wantLen:    0,
			wantStores: []string{},
		},
		{
			name: "ticket with missing TOTAL row (should still be created with 0 total)",
			rows: [][]any{
				{"15/04/2024", "MERCADONA", "", "", "", ""},
				{"item1", "2.99", "", "", "", ""},
				{"item2", "10.00", "", "", "", ""},
			},
			wantLen:    0,
			wantStores: []string{},
			wantTotals: []float64{0},
		},
		{
			name: "multiple tickets with varying TOTAL positions",
			rows: [][]any{
				{"15/04/2024", "MERCADONA", "", "", "", ""},
				{"item1", "2.99", "", "", "", ""},
				{"", "", "", "", "TOTAL", "45.50 €"},
				{"01/05/2024", "DIA", "", "", "", ""},
				{"item1", "1.50", "", "", "", ""},
				{"item2", "3.00", "", "", "", ""},
				{"", "", "", "", "TOTAL", "23.75 €"},
				{"10/06/2024", "ALCAMPO", "", "", "", ""},
				{"item1", "5.00", "", "", "", ""},
				{"", "", "", "", "TOTAL", "89.99 €"},
			},
			wantLen:    3,
			wantStores: []string{"MERCADONA", "DIA", "ALCAMPO"},
			wantTotals: []float64{45.50, 23.75, 89.99},
		},
		{
			name: "sheet with empty row in middle (should stop processing)",
			rows: [][]any{
				{"15/04/2024", "MERCADONA", "", "", "", ""},
				{"item1", "2.99", "", "", "", ""},
				{"", "", "", "", "TOTAL", "45.50 €"},
				{"", "", "", "", "", "0,00 €"},
				{"01/05/2024", "DIA", "", "", "", ""},
				{"item1", "1.50", "", "", "", ""},
				{"", "", "", "", "TOTAL", "23.75 €"},
				{"10/06/2024", "ALCAMPO", "", "", "", ""},
				{"item1", "5.00", "", "", "", ""},
				{"", "", "", "", "TOTAL", "89.99 €"},
			},
			wantLen:    1,
			wantStores: []string{"MERCADONA"},
			wantTotals: []float64{45.50},
		},
		{
			name: "ticket with price containing /kg",
			rows: [][]any{
				{"15/04/2024", "MERCADONA", "", "", "", ""},
				{"item1", "2.99/kg", "", "", "", ""},
				{"", "", "", "", "TOTAL", "45.50 €"},
			},
			wantLen:    1,
			wantStores: []string{"MERCADONA"},
			wantTotals: []float64{45.50},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := testManager(&fakeValues{
				getResp: &sheets.ValueRange{Values: tt.rows},
			})

			got, err := GetSheetTicketList(m, "A1:F300")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != tt.wantLen {
				t.Fatalf("got %d entries, want %d", len(got), tt.wantLen)
			}
			for i := range got {
				if got[i].Store != tt.wantStores[i] {
					t.Errorf("entry[%d].Store = %q, want %q", i, got[i].Store, tt.wantStores[i])
				}
				if !utils.FloatsEqual(got[i].Total, tt.wantTotals[i]) {
					t.Errorf("entry[%d].Total = %f, want %f", i, got[i].Total, tt.wantTotals[i])
				}
			}
		})
	}
}

func TestGetSheetTicketList_APIError(t *testing.T) {
	m := testManager(&fakeValues{
		getErr: fmt.Errorf("network error"),
	})

	_, err := GetSheetTicketList(m, "A1:F300")
	if err == nil {
		t.Error("expected error from API failure, got nil")
	}
}

// ---------------------------------------------------------------------------
// GetLastWrittenRowIndex (requires fake API)
// ---------------------------------------------------------------------------

func TestGetLastWrittenRowIndex(t *testing.T) {
	tests := []struct {
		name    string
		entries []Entry
		wantRow int
	}{
		{
			name:    "single Entry returns default",
			entries: []Entry{{FirstRow: 0, LastRow: 3}},
			wantRow: 3,
		},
		{
			name: "multiple Entries returns lastRow",
			entries: []Entry{
				{FirstRow: 0, LastRow: 3},
				{LastRow: 50}},
			wantRow: 50,
		},
		{
			name: "single Entry with no lastRow returns default",
			entries: []Entry{
				{}},
			wantRow: 1,
		},
		{
			name: "multiple Entries with no LastRow, returns default",
			entries: []Entry{
				{FirstRow: 0},
				{FirstRow: 50}},
			wantRow: 1,
		},
		{
			name:    "no Entries returns default",
			entries: []Entry{},
			wantRow: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetLastWrittenRowIndex(tt.entries)
			if got != tt.wantRow {
				t.Errorf("got %d, want %d", got, tt.wantRow)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// getSheetsConfig (existing, kept for completeness — already in your suite)
// ---------------------------------------------------------------------------

func TestGetSheetsConfig(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		writeFile bool
		wantErr   bool
		wantID    string
	}{
		{"valid", `{"sheets":{"main-id":"1234"}}`, true, false, "1234"},
		{"missing file", "", false, true, ""},
		{"invalid JSON", `{ invalid }`, true, true, ""},
		{"missing sheets field", `{"other":"field"}`, true, true, ""},
		{"empty main-id", `{"sheets":{"main-id":""}}`, true, true, ""},
		{"extra fields ignored", `{"sheets":{"main-id":"42"},"x":1}`, true, false, "42"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "creds.json")
			if tt.writeFile {
				if err := os.WriteFile(path, []byte(tt.content), 0644); err != nil {
					t.Fatalf("setup: %v", err)
				}
			}

			cfg, err := getSheetsConfig(path)
			if (err != nil) != tt.wantErr {
				t.Fatalf("wantErr=%v, got err=%v", tt.wantErr, err)
			}
			if !tt.wantErr && cfg.Sheets.MainID != tt.wantID {
				t.Errorf("MainID = %q, want %q", cfg.Sheets.MainID, tt.wantID)
			}
		})
	}
}
