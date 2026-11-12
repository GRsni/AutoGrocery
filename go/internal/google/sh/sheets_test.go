package sh

import (
	"autoGrocery/pkg/constants"
	"fmt"
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
			expected: fmt.Sprintf("Grocery Ticket: Shop: MERCADONA, Date: %s, Item count: 0,  Total: 0.000000", d(15, 4, 2024).Format(constants.TicketDateFormat)),
		},
		{
			name:     "different store",
			entry:    Entry{Store: "DIA", Date: d(1, 1, 2023)},
			expected: fmt.Sprintf("Grocery Ticket: Shop: DIA, Date: %s, Item count: 0,  Total: 0.000000", d(1, 1, 2023).Format(constants.TicketDateFormat)),
		},
		{
			name:     "zero date",
			entry:    Entry{Store: "CARREFOUR", Date: time.Time{}},
			expected: fmt.Sprintf("Grocery Ticket: Shop: CARREFOUR, Date: %s, Item count: 0,  Total: 0.000000", time.Time{}.Format(constants.TicketDateFormat)),
		},
		{
			name:     "empty store",
			entry:    Entry{Store: "", Date: d(10, 6, 2024)},
			expected: fmt.Sprintf("Grocery Ticket: Shop: , Date: %s, Item count: 0,  Total: 0.000000", d(10, 6, 2024).Format(constants.TicketDateFormat)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.entry.EntryToStr()
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
					t.Errorf("FirstRow = %d, want %d", got.FirstRow, tt.wantRow)
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
// ReadFromSheet
// ---------------------------------------------------------------------------

func TestReadFromSheet(t *testing.T) {
	tests := []struct {
		name    string
		getResp *sheets.ValueRange
		getErr  error
		wantErr bool
		wantLen int
	}{
		{
			name:    "successful read",
			getResp: &sheets.ValueRange{Values: [][]any{{"a", "b"}, {"c", "d"}}},
			getErr:  nil,
			wantErr: false,
			wantLen: 2,
		},
		{
			name:    "empty response",
			getResp: &sheets.ValueRange{Values: [][]any{}},
			getErr:  nil,
			wantErr: false,
			wantLen: 0,
		},
		{
			name:    "nil response",
			getResp: nil,
			getErr:  nil,
			wantErr: false,
			wantLen: 0,
		},
		{
			name:    "API error",
			getResp: nil,
			getErr:  fmt.Errorf("connection failed"),
			wantErr: true,
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := testManager(&fakeValues{getResp: tt.getResp, getErr: tt.getErr})

			got, err := ReadFromSheet(m, "A1:B10")
			if (err != nil) != tt.wantErr {
				t.Fatalf("wantErr=%v, got err=%v", tt.wantErr, err)
			}
			if tt.wantErr {
				return
			}
			if got == nil {
				if tt.wantLen != 0 {
					t.Fatalf("got nil response, want %d rows", tt.wantLen)
				}
				return
			}
			if len(got.Values) != tt.wantLen {
				t.Errorf("got %d rows, want %d", len(got.Values), tt.wantLen)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// getEntryFromSheet
// ---------------------------------------------------------------------------

func TestGetEntryFromSheet(t *testing.T) {
	tests := []struct {
		name         string
		rows         [][]any
		startingRow  int
		wantStore    string
		wantTotal    float64
		wantFirstRow int
		wantLastRow  int
		wantErr      bool
	}{
		{
			name:         "valid ticket with single item",
			rows:         [][]any{{"15/04/2024", "MERCADONA", "", "", "", ""}, {"item1", "2.99", "", "", "", ""}, {"", "", "", "", "TOTAL", "45.50 €"}},
			startingRow:  0,
			wantStore:    "MERCADONA",
			wantTotal:    45.50,
			wantFirstRow: 2,
			wantLastRow:  4,
			wantErr:      false,
		},
		{
			name:         "valid ticket with multiple items",
			rows:         [][]any{{"01/01/2024", "DIA", "", "", "", ""}, {"item1", "5.00", "", "", "", ""}, {"item2", "10.00", "", "", "", ""}, {"", "", "", "", "TOTAL", "150.00 €"}},
			startingRow:  0,
			wantStore:    "DIA",
			wantTotal:    150.00,
			wantFirstRow: 2,
			wantLastRow:  5,
			wantErr:      false,
		},
		{
			name:         "missing TOTAL row",
			rows:         [][]any{{"15/04/2024", "MERCADONA", "", "", "", ""}, {"item1", "2.99", "", "", "", ""}},
			startingRow:  0,
			wantStore:    "",
			wantTotal:    0,
			wantFirstRow: 0,
			wantLastRow:  0,
			wantErr:      true,
		},
		{
			name:         "empty row",
			rows:         [][]any{{"", "", "", "", "", ""}},
			startingRow:  0,
			wantStore:    "",
			wantTotal:    0,
			wantFirstRow: 2,
			wantLastRow:  2,
			wantErr:      false,
		},
		{
			name:         "ticket with store containing newline",
			rows:         [][]any{{"20/06/2024", "MERCADONA\nM001", "", "", "", ""}, {"item1", "3.50", "", "", "", ""}, {"", "", "", "", "TOTAL", "89.99 €"}},
			startingRow:  0,
			wantStore:    "MERCADONA",
			wantTotal:    89.99,
			wantFirstRow: 2,
			wantLastRow:  4,
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &sheets.ValueRange{Values: tt.rows}

			got, finalRow, err := getEntryFromSheet(resp, tt.startingRow)
			if (err != nil) != tt.wantErr {
				t.Fatalf("wantErr=%v, got err=%v", tt.wantErr, err)
			}
			if tt.wantErr {
				return
			}
			if got == nil {
				t.Fatal("got nil entry, want non-nil")
			}
			if got.Store != tt.wantStore {
				t.Errorf("Store = %q, want %q", got.Store, tt.wantStore)
			}
			if !utils.FloatsEqual(got.Total, tt.wantTotal) {
				t.Errorf("Total = %f, want %f", got.Total, tt.wantTotal)
			}
			if got.FirstRow != tt.wantFirstRow {
				t.Errorf("FirstRow = %d, want %d", got.FirstRow, tt.wantFirstRow)
			}
			if got.LastRow != tt.wantLastRow {
				t.Errorf("LastRow = %d, want %d", got.LastRow, tt.wantLastRow)
			}
			// finalRow is the 0-based sheet row index; wantLastRow is 1-based adjusted (finalRow + 2)
			if finalRow != tt.wantLastRow-2 {
				t.Errorf("finalRow = %d, want %d", finalRow, tt.wantLastRow-2)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// isRowEmpty
// ---------------------------------------------------------------------------

func TestIsRowEmpty(t *testing.T) {
	tests := []struct {
		name string
		row  []any
		want bool
	}{
		{
			name: "empty row with 0,00 € marker",
			row:  []any{"", "", "", "", "", "0,00 €"},
			want: true,
		},
		{
			name: "empty row with empty last cell",
			row:  []any{"", "", "", "", "", ""},
			want: false,
		},
		{
			name: "row with content in first column",
			row:  []any{"15/04/2024", "MERCADONA", "", "", "", ""},
			want: false,
		},
		{
			name: "row with content in middle column",
			row:  []any{"", "item1", "2.99", "", "", ""},
			want: false,
		},
		{
			name: "row with only 0,00 € in last column",
			row:  []any{"", "", "", "", "", "0,00 €"},
			want: true,
		},
		{
			name: "single element row with 0,00 €",
			row:  []any{"0,00 €"},
			want: true,
		},
		{
			name: "row with non-empty second-to-last element",
			row:  []any{"", "something", "", "", "", "0,00 €"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isRowEmpty(tt.row)
			if got != tt.want {
				t.Errorf("isRowEmpty(%v) = %v, want %v", tt.row, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// WriteToSheet
// ---------------------------------------------------------------------------

func TestWriteToSheet(t *testing.T) {
	tests := []struct {
		name       string
		values     *sheets.ValueRange
		firstRow   int
		updateResp *sheets.UpdateValuesResponse
		updateErr  error
		wantRows   int
		wantErr    bool
	}{
		{
			name:     "successful write",
			values:   &sheets.ValueRange{Values: [][]any{{"item1", "2.99"}, {"item2", "5.00"}}},
			firstRow: 10,
			updateResp: &sheets.UpdateValuesResponse{
				UpdatedRows:    3,
				UpdatedColumns: 6,
				UpdatedCells:   18,
			},
			updateErr: nil,
			wantRows:  3,
			wantErr:   false,
		},
		{
			name:     "empty values",
			values:   &sheets.ValueRange{Values: [][]any{}},
			firstRow: 5,
			updateResp: &sheets.UpdateValuesResponse{
				UpdatedCells: 6,
			},
			updateErr: nil,
			wantRows:  1,
			wantErr:   false,
		},
		{
			name:       "write error",
			values:     &sheets.ValueRange{Values: [][]any{{"item1", "2.99"}}},
			firstRow:   10,
			updateResp: nil,
			updateErr:  fmt.Errorf("network error"),
			wantRows:   10,    // should return firstRow on error
			wantErr:    false, // WriteToSheet doesn't return error, just logs
		},
		{
			name:     "single item ticket",
			values:   &sheets.ValueRange{Values: [][]any{{"item1", "10.00"}}},
			firstRow: 20,
			updateResp: &sheets.UpdateValuesResponse{
				UpdatedCells: 9,
			},
			updateErr: nil,
			wantRows:  2,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := testManager(&fakeValues{
				updateResp: tt.updateResp,
				updateErr:  tt.updateErr,
			})

			got := WriteSingleTicketToSheet(m, tt.values, tt.firstRow)
			if got != tt.wantRows {
				t.Errorf("got %d, want %d", got, tt.wantRows)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// FormatTicketBlock
// ---------------------------------------------------------------------------

func TestFormatTicketBlock(t *testing.T) {
	tests := []struct {
		name        string
		firstRow    int
		ticketItems int
		batchResp   *sheets.BatchUpdateSpreadsheetResponse
		batchErr    error
		wantErr     bool
	}{
		{
			name:        "successful format",
			firstRow:    10,
			ticketItems: 3,
			batchResp:   &sheets.BatchUpdateSpreadsheetResponse{},
			batchErr:    nil,
			wantErr:     false,
		},
		{
			name:        "batch update error",
			firstRow:    10,
			ticketItems: 3,
			batchResp:   nil,
			batchErr:    fmt.Errorf("API error"),
			wantErr:     true,
		},
		{
			name:        "single item ticket",
			firstRow:    5,
			ticketItems: 1,
			batchResp:   &sheets.BatchUpdateSpreadsheetResponse{},
			batchErr:    nil,
			wantErr:     false,
		},
		{
			name:        "large ticket block",
			firstRow:    100,
			ticketItems: 50,
			batchResp:   &sheets.BatchUpdateSpreadsheetResponse{},
			batchErr:    nil,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := testManager(&fakeValues{
				batchUpdateResp: tt.batchResp,
				batchUpdateErr:  tt.batchErr,
			})

			err := FormatTicketBlock(m, tt.firstRow, tt.ticketItems)
			if (err != nil) != tt.wantErr {
				t.Fatalf("wantErr=%v, got err=%v", tt.wantErr, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// getStoreNameAndId
// ---------------------------------------------------------------------------

func TestGetStoreNameAndId(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantName string
		wantId   string
	}{
		{
			name:     "simple store name",
			input:    "MERCADONA",
			wantName: "MERCADONA",
			wantId:   "MERCADONA",
		},
		{
			name:     "store with ID",
			input:    "MERCADONA\nM001",
			wantName: "MERCADONA",
			wantId:   "M001",
		},
		{
			name:     "store with multiple newlines",
			input:    "DIA\nD001\nextra",
			wantName: "DIA",
			wantId:   "extra",
		},
		{
			name:     "empty input",
			input:    "",
			wantName: "",
			wantId:   "",
		},
		{
			name:     "store with newline and empty ID",
			input:    "CARREFOUR\n",
			wantName: "CARREFOUR",
			wantId:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotName, gotId := getStoreNameAndId(tt.input)
			if gotName != tt.wantName {
				t.Errorf("name = %q, want %q", gotName, tt.wantName)
			}
			if gotId != tt.wantId {
				t.Errorf("id = %q, want %q", gotId, tt.wantId)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Entry.ToValueRange
// ---------------------------------------------------------------------------

func TestEntryToValueRange(t *testing.T) {
	d := func(day, month, year int) time.Time {
		return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	}

	t.Run("empty items returns error", func(t *testing.T) {
		entry := Entry{Store: "MERCADONA", Date: d(15, 4, 2024), Id: "M001"}
		_, err := entry.ToValueRange()
		if err == nil {
			t.Fatal("expected error for empty items, got nil")
		}
	})

	t.Run("single item with date and store metadata", func(t *testing.T) {
		entry := Entry{
			Store: "MERCADONA",
			Id:    "M001",
			Date:  d(15, 4, 2024),
			Items: Items{{Name: "Apple", Amount: 2.5, Price: 3.99}},
		}
		vr, err := entry.ToValueRange()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if vr.MajorDimension != "ROWS" {
			t.Errorf("MajorDimension = %q, want %q", vr.MajorDimension, "ROWS")
		}
		if len(vr.Values) != 1 {
			t.Fatalf("got %d rows, want 1", len(vr.Values))
		}
		// First row should have date in column 0 and store+id in column 1
		if vr.Values[0][0] != "15/04/2024" {
			t.Errorf("row[0][0] = %v, want %v", vr.Values[0][0], "15/04/2024")
		}
		if vr.Values[0][1] != "MERCADONA\nM001" {
			t.Errorf("row[0][1] = %q, want %q", vr.Values[0][1], "MERCADONA\nM001")
		}
		// Item columns
		if vr.Values[0][2] != "Apple" {
			t.Errorf("row[0][2] = %v, want %v", vr.Values[0][2], "Apple")
		}
	})

	t.Run("multiple items", func(t *testing.T) {
		entry := Entry{
			Store: "DIA",
			Id:    "D001",
			Date:  d(1, 1, 2024),
			Items: Items{
				{Name: "Apple", Amount: 2.5, Price: 3.99},
				{Name: "Banana", Amount: 1.0, Price: 2.50},
			},
		}
		vr, err := entry.ToValueRange()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(vr.Values) != 2 {
			t.Fatalf("got %d rows, want 2", len(vr.Values))
		}
		// First row has date/store metadata
		if vr.Values[0][0] != "01/01/2024" {
			t.Errorf("row[0][0] = %v, want %v", vr.Values[0][0], "01/01/2024")
		}
		if vr.Values[0][1] != "DIA\nD001" {
			t.Errorf("row[0][1] = %q, want %q", vr.Values[0][1], "DIA\nD001")
		}
		// Second row should have empty date/store
		if vr.Values[1][0] != "" {
			t.Errorf("row[1][0] = %v, want empty", vr.Values[1][0])
		}
		if vr.Values[1][1] != "" {
			t.Errorf("row[1][1] = %v, want empty", vr.Values[1][1])
		}
	})

	t.Run("plus sign prefix is escaped", func(t *testing.T) {
		entry := Entry{
			Store: "MERCADONA",
			Id:    "",
			Date:  d(10, 6, 2024),
			Items: Items{{Name: "+Discount", Amount: -5.0, Price: 0}},
		}
		vr, err := entry.ToValueRange()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if vr.Values[0][2] != "'+Discount" {
			t.Errorf("row[0][2] = %q, want %q", vr.Values[0][2], "'+Discount")
		}
	})

	t.Run("plus sign in middle of name is not escaped", func(t *testing.T) {
		entry := Entry{
			Store: "DIA",
			Id:    "",
			Date:  d(5, 3, 2024),
			Items: Items{{Name: "A+B Mix", Amount: 1.0, Price: 10.0}},
		}
		vr, err := entry.ToValueRange()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if vr.Values[0][2] != "A+B Mix" {
			t.Errorf("row[0][2] = %q, want %q", vr.Values[0][2], "A+B Mix")
		}
	})

	t.Run("zero total and zero items amount", func(t *testing.T) {
		entry := Entry{
			Store: "CARREFOUR",
			Id:    "C001",
			Date:  d(1, 1, 2024),
			Items: Items{{Name: "Free Item", Amount: 0, Price: 0}},
		}
		vr, err := entry.ToValueRange()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(vr.Values) != 1 {
			t.Fatalf("got %d rows, want 1", len(vr.Values))
		}
	})
}

// ---------------------------------------------------------------------------
// getEntryItem
// ---------------------------------------------------------------------------

func TestGetEntryItem(t *testing.T) {
	tests := []struct {
		name      string
		row       []any
		wantName  string
		wantAmt   float64
		wantPrice float64
		wantNil   bool
	}{
		{
			name:      "valid item with all fields",
			row:       []any{"", "", "Apple", "2.5", "3.99"},
			wantName:  "Apple",
			wantAmt:   2.5,
			wantPrice: 3.99,
			wantNil:   false,
		},
		{
			name:    "missing name",
			row:     []any{"", "", "", "2.5", "3.99"},
			wantNil: true,
		},
		{
			name:    "missing amount",
			row:     []any{"", "", "Apple", "", "3.99"},
			wantNil: true,
		},
		{
			name:    "missing price",
			row:     []any{"", "", "Apple", "2.5", ""},
			wantNil: true,
		},
		{
			name:      "item with /kg price",
			row:       []any{"", "", "Banana", "1.5", "2.50/kg"},
			wantName:  "Banana",
			wantAmt:   1.5,
			wantPrice: 2.50,
			wantNil:   false,
		},
		{
			name:      "item with euro symbol",
			row:       []any{"", "", "Cheese", "1.0", "€15.99/kg"},
			wantName:  "Cheese",
			wantAmt:   1.0,
			wantPrice: 15.99,
			wantNil:   false,
		},
		{
			name:    "row too short",
			row:     []any{"", ""},
			wantNil: true,
		},
		{
			name:    "empty row",
			row:     []any{"", "", "", "", ""},
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getEntryItem(tt.row)
			if tt.wantNil {
				if got != nil {
					t.Errorf("got non-nil item, want nil")
				}
				return
			}
			if got == nil {
				t.Fatal("got nil item, want non-nil")
			}
			if got.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", got.Name, tt.wantName)
			}
			if !utils.FloatsEqual(got.Amount, tt.wantAmt) {
				t.Errorf("Amount = %f, want %f", got.Amount, tt.wantAmt)
			}
			if !utils.FloatsEqual(got.Price, tt.wantPrice) {
				t.Errorf("Price = %f, want %f", got.Price, tt.wantPrice)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// getEntryFromSheet edge cases
// ---------------------------------------------------------------------------

func TestGetEntryFromSheet_EdgeCases(t *testing.T) {
	t.Run("row length less than 6 returns error", func(t *testing.T) {
		rows := [][]any{{"15/04/2024", "MERCADONA"}} // only 2 columns
		resp := &sheets.ValueRange{Values: rows}
		_, _, err := getEntryFromSheet(resp, 0)
		if err == nil {
			t.Fatal("expected error for short row, got nil")
		}
	})

	t.Run("non-zero starting row", func(t *testing.T) {
		rows := [][]any{
			{"header1", "header2", "", "", "", ""},
			{"15/04/2024", "MERCADONA", "", "", "", ""},
			{"item1", "2.99", "", "", "", ""},
			{"", "", "", "", "TOTAL", "45.50 €"},
		}
		resp := &sheets.ValueRange{Values: rows}
		got, finalRow, err := getEntryFromSheet(resp, 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Store != "MERCADONA" {
			t.Errorf("Store = %q, want %q", got.Store, "MERCADONA")
		}
		if !utils.FloatsEqual(got.Total, 45.50) {
			t.Errorf("Total = %f, want %f", got.Total, 45.50)
		}
		if got.FirstRow != 3 { // startingRow + 2
			t.Errorf("FirstRow = %d, want %d", got.FirstRow, 3)
		}
		if finalRow != 3 { // 0-based index of TOTAL row
			t.Errorf("finalRow = %d, want %d", finalRow, 3)
		}
	})

	t.Run("items accumulated from multiple rows", func(t *testing.T) {
		rows := [][]any{
			{"20/06/2024", "DIA", "", "", "", ""},
			{"", "", "item1", "2.0", "5.00", ""},
			{"", "", "item2", "1.5", "3.00", ""},
			{"", "", "item3", "3.0", "8.99", ""},
			{"", "", "", "", "TOTAL", "89.99 €"},
		}
		resp := &sheets.ValueRange{Values: rows}
		got, _, err := getEntryFromSheet(resp, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got.Items) != 3 {
			t.Fatalf("got %d items, want 3", len(got.Items))
		}
		if got.Items[0].Name != "item1" || got.Items[1].Name != "item2" || got.Items[2].Name != "item3" {
			t.Errorf("unexpected item names: %v", got.Items)
		}
	})

	t.Run("empty date skips ticket parsing", func(t *testing.T) {
		rows := [][]any{{"", "MERCADONA", "", "", "", ""}, {"item1", "2.99", "", "", "", ""}}
		resp := &sheets.ValueRange{Values: rows}
		got, _, err := getEntryFromSheet(resp, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Empty date means the ticket parsing block is skipped; entry has zero values
		if got.Store != "" {
			t.Errorf("Store = %q, want empty (skipped)", got.Store)
		}
	})

	t.Run("ticket with items before TOTAL found", func(t *testing.T) {
		rows := [][]any{
			{"15/04/2024", "CARREFOUR", "", "", "", ""},
			{"", "", "item1", "5.00", "3.00", ""},
			{"", "", "item2", "10.00", "2.00", ""},
			{"", "", "", "", "TOTAL", "150.00 €"},
		}
		resp := &sheets.ValueRange{Values: rows}
		got, _, err := getEntryFromSheet(resp, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got.Items) != 2 {
			t.Fatalf("got %d items, want 2", len(got.Items))
		}
	})
}

// ---------------------------------------------------------------------------
// isRowEmpty edge cases
// ---------------------------------------------------------------------------

func TestIsRowEmpty_EdgeCases(t *testing.T) {
	tests := []struct {
		name string
		row  []any
		want bool
	}{
		{
			name: "single element not 0,00 €",
			row:  []any{"something"},
			want: false,
		},
		{
			name: "two elements with content in first",
			row:  []any{"data", "0,00 €"},
			want: false,
		},
		{
			name: "two elements empty first with 0,00 €",
			row:  []any{"", "0,00 €"},
			want: true,
		},
		{
			name: "three elements middle has content",
			row:  []any{"", "data", "", "0,00 €"},
			want: false,
		},
		{
			name: "last element not 0,00 €",
			row:  []any{"", "", "", "", "", "123.45 €"},
			want: false,
		},
		{
			name: "all empty except last with 0,00 €",
			row:  []any{"", "", "", "", "", "", "0,00 €"},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isRowEmpty(tt.row)
			if got != tt.want {
				t.Errorf("isRowEmpty(%v) = %v, want %v", tt.row, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// EntryToStr with items and total
// ---------------------------------------------------------------------------

func TestEntryToStr_WithItemsAndTotal(t *testing.T) {
	d := func(day, month, year int) time.Time {
		return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	}

	tests := []struct {
		name     string
		entry    Entry
		expected string
	}{
		{
			name: "entry with items shows count",
			entry: Entry{
				Store: "MERCADONA",
				Date:  d(15, 4, 2024),
				Items: Items{{Name: "Apple", Amount: 2.5, Price: 3.99}, {Name: "Banana", Amount: 1.0, Price: 2.50}},
				Total: 45.50,
			},
			expected: "Grocery Ticket: Shop: MERCADONA, Date: 15/04/2024, Item count: 2,  Total: 45.500000",
		},
		{
			name: "entry with single item",
			entry: Entry{
				Store: "DIA",
				Date:  d(1, 1, 2024),
				Items: Items{{Name: "Item1", Amount: 1.0, Price: 10.0}},
				Total: 10.0,
			},
			expected: "Grocery Ticket: Shop: DIA, Date: 01/01/2024, Item count: 1,  Total: 10.000000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.entry.EntryToStr()
			if got != tt.expected {
				t.Errorf("got %q, want %q", got, tt.expected)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// GetSheetTicketList row length validation
// ---------------------------------------------------------------------------

func TestGetSheetTicketList_RowLengthError(t *testing.T) {
	m := testManager(&fakeValues{
		getResp: &sheets.ValueRange{Values: [][]any{{"15/04/2024", "MERCADONA"}}}, // only 2 columns
	})

	_, err := GetSheetTicketList(m, "A1:F300")
	if err == nil {
		t.Fatal("expected error for short row, got nil")
	}
}

// ---------------------------------------------------------------------------
// GetLastEntryForStore returns correct entry fields
// ---------------------------------------------------------------------------

func TestGetLastEntryForStore_Fields(t *testing.T) {
	d := func(day int) time.Time { return time.Date(2024, 1, day, 0, 0, 0, 0, time.UTC) }

	tickets := []Entry{
		{Store: "MERCADONA", Date: d(1), Id: "M001", FirstRow: 1, LastRow: 5, Total: 45.50},
		{Store: "DIA", Date: d(2), Id: "D001", FirstRow: 6, LastRow: 10, Total: 23.75},
		{Store: "MERCADONA", Date: d(3), Id: "M002", FirstRow: 11, LastRow: 15, Total: 89.99},
	}

	got := GetLastEntryForStore(tickets, "MERCADONA")
	if got == nil {
		t.Fatal("got nil, want entry")
	}
	if got.Id != "M002" {
		t.Errorf("Id = %q, want %q", got.Id, "M002")
	}
	if got.LastRow != 15 {
		t.Errorf("LastRow = %d, want %d", got.LastRow, 15)
	}
	if !utils.FloatsEqual(got.Total, 89.99) {
		t.Errorf("Total = %f, want %f", got.Total, 89.99)
	}
}

// ---------------------------------------------------------------------------
// GetSheetTicketList with items parsed
// ---------------------------------------------------------------------------

func TestGetSheetTicketList_Items(t *testing.T) {
	m := testManager(&fakeValues{
		getResp: &sheets.ValueRange{Values: [][]any{
			{"15/04/2024", "MERCADONA", "", "", "", ""},
			{"", "", "Apple", "2.5", "3.99", ""},
			{"", "", "Banana", "1.0", "2.50/kg", ""},
			{"", "", "", "", "TOTAL", "45.50 €"},
		}},
	})

	got, err := GetSheetTicketList(m, "A1:F300")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d entries, want 1", len(got))
	}
	if len(got[0].Items) != 2 {
		t.Fatalf("got %d items, want 2", len(got[0].Items))
	}
	if got[0].Items[0].Name != "Apple" {
		t.Errorf("Item[0].Name = %q, want %q", got[0].Items[0].Name, "Apple")
	}
	if !utils.FloatsEqual(got[0].Items[0].Amount, 2.5) {
		t.Errorf("Item[0].Amount = %f, want %f", got[0].Items[0].Amount, 2.5)
	}
	if !utils.FloatsEqual(got[0].Items[1].Price, 2.50) {
		t.Errorf("Item[1].Price = %f, want %f", got[0].Items[1].Price, 2.50)
	}
}

// --- Tests for BUG-1 fix: empty item name does not panic in Entry.ToValueRange ---

func TestEntryToValueRange_EmptyItemName_NoPanic(t *testing.T) {
	// BUG-1 regression test: item.Name[0] would panic on empty string
	entry := Entry{
		Store: "MERCADONA",
		Id:    "M001",
		Date:  time.Date(2024, 4, 15, 0, 0, 0, 0, time.UTC),
		Items: Items{{Name: "", Amount: 1, Price: 5.00}},
	}

	vr, err := entry.ToValueRange()
	if err != nil {
		t.Fatalf("unexpected error for empty item name: %v", err)
	}
	if vr == nil {
		t.Fatal("expected non-nil ValueRange, got nil")
	}
	if len(vr.Values) != 1 {
		t.Fatalf("got %d rows, want 1", len(vr.Values))
	}
	// Empty name should be preserved as empty string (not panic)
	if vr.Values[0][2] != "" {
		t.Errorf("row[0][2] = %q, want empty string", vr.Values[0][2])
	}
}

func TestEntryToValueRange_MixedEmptyAndPlusNames(t *testing.T) {
	entry := Entry{
		Store: "DIA",
		Id:    "D001",
		Date:  time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		Items: Items{
			{Name: "", Amount: 1, Price: 2.00},       // empty name
			{Name: "+Bonus", Amount: 1, Price: 3.00}, // plus prefix
			{Name: "Normal", Amount: 1, Price: 5.00}, // normal name
		},
	}

	vr, err := entry.ToValueRange()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vr.Values) != 3 {
		t.Fatalf("got %d rows, want 3", len(vr.Values))
	}
	// First row: empty name preserved as-is (no + prefix to escape)
	if vr.Values[0][2] != "" {
		t.Errorf("row[0][2] = %q, want empty string", vr.Values[0][2])
	}
	// Second row: plus prefix escaped with single quote
	if vr.Values[1][2] != "'+Bonus" {
		t.Errorf("row[1][2] = %q, want '+Bonus", vr.Values[1][2])
	}
	// Third row: normal name unchanged
	if vr.Values[2][2] != "Normal" {
		t.Errorf("row[2][2] = %q, want Normal", vr.Values[2][2])
	}
}

func TestEntryToValueRange_OriginalItemsNotMutated(t *testing.T) {
	// BUG-1 regression: the fix copies item.Name to a local var so the original
	// Entry Items slice is not mutated.
	entry := Entry{
		Store: "MERCADONA",
		Id:    "",
		Date:  time.Date(2024, 6, 10, 0, 0, 0, 0, time.UTC),
		Items: Items{{Name: "+Escaped", Amount: 1, Price: 1.00}},
	}

	originalName := entry.Items[0].Name
	_, err := entry.ToValueRange()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.Items[0].Name != originalName {
		t.Errorf("original item name mutated from %q to %q", originalName, entry.Items[0].Name)
	}
}

func TestEntryToValueRange_PlusPrefixVariants(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"plus at start", "+Sale", "'+Sale"},
		{"plus in middle", "A+B", "A+B"},
		{"plus at end", "Item+", "Item+"},
		{"just plus sign", "+", "'+"},
		{"plus with space", "+ Bonus", "'+ Bonus"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := Entry{
				Store: "MERCADONA",
				Id:    "",
				Date:  time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				Items: Items{{Name: tt.input, Amount: 1, Price: 5.00}},
			}
			vr, err := entry.ToValueRange()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if vr.Values[0][2] != tt.expected {
				t.Errorf("row[0][2] = %q, want %q", vr.Values[0][2], tt.expected)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// WriteAllTickets
// ---------------------------------------------------------------------------

func TestWriteAllTickets_EmptyEntries(t *testing.T) {
	m := testManager(&fakeValues{})
	err := WriteAllTickets(m, []Entry{})
	if err != nil {
		t.Errorf("WriteAllTickets(empty) = %v, want nil", err)
	}
}

func TestWriteAllTickets_SingleEntry(t *testing.T) {
	m := testManager(&fakeValues{
		updateResp: &sheets.UpdateValuesResponse{UpdatedCells: 18},
	})

	entry := Entry{
		Store: "MERCADONA", Id: "M001",
		Date:     time.Date(2024, 4, 15, 0, 0, 0, 0, time.UTC),
		FirstRow: 2, LastRow: 3,
		Items: Items{{Name: "Apple", Amount: 2.5, Price: 3.99}},
	}

	err := WriteAllTickets(m, []Entry{entry})
	if err != nil {
		t.Fatalf("WriteAllTickets(single) error: %v", err)
	}
}

func TestWriteAllTickets_MultipleEntries(t *testing.T) {
	m := testManager(&fakeValues{
		updateResp: &sheets.UpdateValuesResponse{UpdatedCells: 54},
	})

	entries := []Entry{
		{
			Store: "MERCADONA", Id: "M001",
			Date:     time.Date(2024, 4, 15, 0, 0, 0, 0, time.UTC),
			FirstRow: 2, LastRow: 3,
			Items: Items{{Name: "Apple", Amount: 2.5, Price: 3.99}},
		},
		{
			Store: "DIA", Id: "D001",
			Date:     time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC),
			FirstRow: 4, LastRow: 6,
			Items: Items{
				{Name: "Banana", Amount: 1.0, Price: 2.50},
				{Name: "Cheese", Amount: 0.5, Price: 15.00},
			},
		},
	}

	err := WriteAllTickets(m, entries)
	if err != nil {
		t.Fatalf("WriteAllTickets(multiple) error: %v", err)
	}
}

func TestWriteAllTickets_APIError(t *testing.T) {
	m := testManager(&fakeValues{
		updateErr: fmt.Errorf("network error"),
	})

	entry := Entry{
		Store: "MERCADONA", Id: "M001",
		Date:     time.Date(2024, 4, 15, 0, 0, 0, 0, time.UTC),
		FirstRow: 2, LastRow: 3,
		Items: Items{{Name: "Apple", Amount: 2.5, Price: 3.99}},
	}

	err := WriteAllTickets(m, []Entry{entry})
	if err == nil {
		t.Fatal("expected error from API failure, got nil")
	}
}

func TestWriteAllTickets_SkipsEntriesWithNoItems(t *testing.T) {
	m := testManager(&fakeValues{
		updateResp: &sheets.UpdateValuesResponse{UpdatedCells: 18},
	})

	entries := []Entry{
		{
			Store: "MERCADONA", Id: "M001",
			Date:     time.Date(2024, 4, 15, 0, 0, 0, 0, time.UTC),
			FirstRow: 2, LastRow: 3,
			Items: Items{{Name: "Apple", Amount: 2.5, Price: 3.99}},
		},
		{
			Store: "DIA", Id: "D001",
			Date:     time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC),
			FirstRow: 4, LastRow: 4,
			Items: Items{},
		},
	}

	err := WriteAllTickets(m, entries)
	if err != nil {
		t.Fatalf("WriteAllTickets(skip-empty) error: %v", err)
	}
}

func TestWriteAllTickets_AllEntriesEmpty(t *testing.T) {
	m := testManager(&fakeValues{})

	entries := []Entry{
		{Store: "MERCADONA", Id: "M001", Date: time.Date(2024, 4, 15, 0, 0, 0, 0, time.UTC), FirstRow: 2, LastRow: 2, Items: Items{}},
	}

	err := WriteAllTickets(m, entries)
	if err != nil {
		t.Errorf("WriteAllTickets(all-empty) = %v, want nil", err)
	}
}

// ---------------------------------------------------------------------------
// formatAllTicketBlocks
// ---------------------------------------------------------------------------

func TestFormatAllTicketBlocks_Empty(t *testing.T) {
	m := testManager(&fakeValues{batchUpdateResp: &sheets.BatchUpdateSpreadsheetResponse{}})
	err := formatAllTicketBlocks(m, []Entry{})
	if err != nil {
		t.Errorf("formatAllTicketBlocks(empty) = %v, want nil", err)
	}
}

func TestFormatAllTicketBlocks_SingleEntry(t *testing.T) {
	m := testManager(&fakeValues{batchUpdateResp: &sheets.BatchUpdateSpreadsheetResponse{}})

	entry := Entry{
		Store: "MERCADONA", Id: "M001",
		Date:     time.Date(2024, 4, 15, 0, 0, 0, 0, time.UTC),
		FirstRow: 2, LastRow: 3,
		Items: Items{{Name: "Apple", Amount: 2.5, Price: 3.99}},
	}

	err := formatAllTicketBlocks(m, []Entry{entry})
	if err != nil {
		t.Errorf("formatAllTicketBlocks(single) = %v, want nil", err)
	}
}

func TestFormatAllTicketBlocks_MultipleEntries(t *testing.T) {
	m := testManager(&fakeValues{batchUpdateResp: &sheets.BatchUpdateSpreadsheetResponse{}})

	entries := []Entry{
		{Store: "MERCADONA", Id: "M001", Date: time.Date(2024, 4, 15, 0, 0, 0, 0, time.UTC), FirstRow: 2, LastRow: 3, Items: Items{{Name: "Apple", Amount: 2.5, Price: 3.99}}},
		{Store: "DIA", Id: "D001", Date: time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC), FirstRow: 4, LastRow: 6, Items: Items{{Name: "Banana", Amount: 1.0, Price: 2.50}, {Name: "Cheese", Amount: 0.5, Price: 15.00}}},
	}

	err := formatAllTicketBlocks(m, entries)
	if err != nil {
		t.Errorf("formatAllTicketBlocks(multiple) = %v, want nil", err)
	}
}

func TestFormatAllTicketBlocks_APIError(t *testing.T) {
	m := testManager(&fakeValues{batchUpdateErr: fmt.Errorf("API error")})

	entry := Entry{
		Store: "MERCADONA", Id: "M001", Date: time.Date(2024, 4, 15, 0, 0, 0, 0, time.UTC),
		FirstRow: 2, LastRow: 3, Items: Items{{Name: "Apple", Amount: 2.5, Price: 3.99}},
	}

	err := formatAllTicketBlocks(m, []Entry{entry})
	if err == nil {
		t.Fatal("expected error from API failure, got nil")
	}
}
