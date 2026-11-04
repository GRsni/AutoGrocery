package mercadona

import (
	"testing"
	"time"

	"autoGrocery/internal"
	"autoGrocery/utils"

	"google.golang.org/api/gmail/v1"
)

// ============================================================================
// Tests for isTotalCorrect
// ============================================================================

func Test_isTotalCorrect(t *testing.T) {
	type args struct {
		qty        float64
		pricePer   float64
		totalFound float64
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "exact match single unit",
			args: args{qty: 1, pricePer: 2.50, totalFound: 2.50},
			want: true,
		},
		{
			name: "exact match multiple units",
			args: args{qty: 3, pricePer: 1.20, totalFound: 3.60},
			want: true,
		},
		{
			name: "floating point that rounds correctly",
			args: args{qty: 2, pricePer: 0.55, totalFound: 1.10},
			want: true,
		},
		{
			name: "mismatch",
			args: args{qty: 2, pricePer: 1.50, totalFound: 4.00},
			want: false,
		},
		{
			name: "rounding edge case - rounds to match",
			args: args{qty: 3, pricePer: 0.333, totalFound: 1.00},
			want: true,
		},
		{
			name: "zero quantity",
			args: args{qty: 0, pricePer: 5.00, totalFound: 0.00},
			want: true,
		},
		{
			name: "zero price",
			args: args{qty: 5, pricePer: 0.00, totalFound: 0.00},
			want: true,
		},
		{
			name: "large values",
			args: args{qty: 10, pricePer: 99.99, totalFound: 999.90},
			want: true,
		},
		{
			name: "off by 0.01",
			args: args{qty: 1, pricePer: 10.00, totalFound: 10.01},
			want: false,
		},
		{
			name: "off by 0.10",
			args: args{qty: 2, pricePer: 5.00, totalFound: 6.00},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isTotalCorrect(tt.args.qty, tt.args.pricePer, tt.args.totalFound); got != tt.want {
				t.Errorf("isTotalCorrect() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ============================================================================
// Tests for getTicketId
// ============================================================================

func Test_getTicketId(t *testing.T) {
	tests := []struct {
		name    string
		snippet string
		want    string
	}{
		{
			name:    "extracts last word from snippet",
			snippet: "Tu compra en Mercadona - Ticket 123456789",
			want:    "123456789",
		},
		{
			name:    "single word snippet",
			snippet: "987654321",
			want:    "987654321",
		},
		{
			name:    "snippet with extra spaces between words",
			snippet: "Ref ticket ABC-001",
			want:    "ABC-001",
		},
		{
			name:    "snippet with special characters",
			snippet: "Ticket ABC@123",
			want:    "ABC@123",
		},
		{
			name:    "snippet with hyphens",
			snippet: "Mercadona purchase REF-123-XYZ",
			want:    "REF-123-XYZ",
		},
		{
			name:    "empty snippet",
			snippet: "",
			want:    "",
		},
		{
			name:    "single space snippet",
			snippet: "   ",
			want:    "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			message := &gmail.Message{Snippet: tt.snippet}
			if got := getTicketId(message); got != tt.want {
				t.Errorf("getTicketId() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ============================================================================
// Tests for getDateFromTicket
// ============================================================================

func Test_getDateFromTicket(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		wantDate time.Time
		wantErr  bool
	}{
		{
			name:     "valid date format YYYYMMDD",
			filename: "20231115 Mercadona 25.50.pdf",
			wantDate: time.Date(2023, 11, 15, 0, 0, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "valid date - end of year",
			filename: "20241231 Mercadona 100.00.pdf",
			wantDate: time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "valid date - start of year",
			filename: "20230101 Mercadona 10.00.pdf",
			wantDate: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "valid date - February",
			filename: "20230228 Mercadona 15.00.pdf",
			wantDate: time.Date(2023, 2, 28, 0, 0, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "valid leap year date",
			filename: "20240229 Mercadona 20.00.pdf",
			wantDate: time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "invalid date - month 13",
			filename: "20231301 Mercadona 10.00.pdf",
			wantDate: time.Time{},
			wantErr:  true,
		},
		{
			name:     "invalid date - day 0",
			filename: "20230200 Mercadona 10.00.pdf",
			wantDate: time.Time{},
			wantErr:  true,
		},
		{
			name:     "invalid date - day 32",
			filename: "20230132 Mercadona 10.00.pdf",
			wantDate: time.Time{},
			wantErr:  true,
		},
		{
			name:     "invalid date - non-leap year February 29",
			filename: "20230229 Mercadona 10.00.pdf",
			wantDate: time.Time{},
			wantErr:  true,
		},
		{
			name:     "invalid format - dashes",
			filename: "2023-11-15 Mercadona 10.00.pdf",
			wantDate: time.Time{},
			wantErr:  true,
		},
		{
			name:     "invalid format - slashes",
			filename: "2023/11/15 Mercadona 10.00.pdf",
			wantDate: time.Time{},
			wantErr:  true,
		},
		{
			name:     "empty filename",
			filename: "",
			wantDate: time.Time{},
			wantErr:  true,
		},
		{
			name:     "too short filename",
			filename: "2023",
			wantDate: time.Time{},
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			message := &gmail.Message{
				Payload: &gmail.MessagePart{
					Parts: []*gmail.MessagePart{
						nil,
						{Filename: tt.filename},
					},
				},
			}
			got, err := getDateFromTicket(message)
			if (err != nil) != tt.wantErr {
				t.Errorf("getDateFromTicket() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !got.Equal(tt.wantDate) {
				t.Errorf("getDateFromTicket() = %v, want %v", got, tt.wantDate)
			}
		})
	}
}

// ============================================================================
// Tests for getTicketTotal
// ============================================================================

func Test_getTicketTotal(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     float64
	}{
		{
			name:     "valid total with dot decimal",
			filename: "20231115 Mercadona 25.50.pdf",
			want:     25.50,
		},
		{
			name:     "total with cents",
			filename: "20231115 Mercadona 42.99.pdf",
			want:     42.99,
		},
		{
			name:     "total with comma decimal",
			filename: "20231115 Mercadona 10,50.pdf",
			want:     10.50,
		},
		{
			name:     "large total",
			filename: "20231115 Mercadona 150.00.pdf",
			want:     150.00,
		},
		{
			name:     "small total",
			filename: "20231115 Mercadona 0.99.pdf",
			want:     0.99,
		},
		{
			name:     "zero total",
			filename: "20231115 Mercadona 0.00.pdf",
			want:     0.00,
		},
		{
			name:     "single digit total",
			filename: "20231115 Mercadona 5.00.pdf",
			want:     5.00,
		},
		{
			name:     "total with comma and no cents",
			filename: "20231115 Mercadona 100,00.pdf",
			want:     100.00,
		},
		{
			name:     "total with three digits",
			filename: "20231115 Mercadona 999.99.pdf",
			want:     999.99,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			message := &gmail.Message{
				Payload: &gmail.MessagePart{
					Parts: []*gmail.MessagePart{
						nil,
						{Filename: tt.filename},
					},
				},
			}
			got := getTicketTotal(message)
			if got != tt.want {
				t.Errorf("getTicketTotal() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getTicketTotal_InvalidFilename(t *testing.T) {
	tests := []struct {
		name     string
		filename string
	}{
		{
			name:     "empty filename",
			filename: "",
		},
		{
			name:     "missing parts",
			filename: "20231115.pdf",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			message := &gmail.Message{
				Payload: &gmail.MessagePart{
					Parts: []*gmail.MessagePart{
						nil,
						{Filename: tt.filename},
					},
				},
			}
			// This should panic or return 0 due to invalid filename
			got := getTicketTotal(message)
			t.Logf("getTicketTotal() with invalid filename = %v", got)
		})
	}
}

// ============================================================================
// Tests for internal.Item and internal.Ticket structures
// ============================================================================

func Test_Items_IsTotalValid(t *testing.T) {
	tests := []struct {
		name     string
		items    internal.Items
		expected float64
		want     bool
	}{
		{
			name: "single item exact match",
			items: internal.Items{
				{Name: "Item1", Amount: 1, Price: 10.00},
			},
			expected: 10.00,
			want:     true,
		},
		{
			name: "single item mismatch",
			items: internal.Items{
				{Name: "Item1", Amount: 1, Price: 10.00},
			},
			expected: 11.00,
			want:     false,
		},
		{
			name: "multiple items exact match",
			items: internal.Items{
				{Name: "Item1", Amount: 2, Price: 5.00},
				{Name: "Item2", Amount: 1, Price: 3.00},
			},
			expected: 13.00,
			want:     true,
		},
		{
			name: "multiple items mismatch",
			items: internal.Items{
				{Name: "Item1", Amount: 2, Price: 5.00},
				{Name: "Item2", Amount: 1, Price: 3.00},
			},
			expected: 12.00,
			want:     false,
		},
		{
			name:     "empty items",
			items:    internal.Items{},
			expected: 0.00,
			want:     true,
		},
		{
			name: "floating point rounding",
			items: internal.Items{
				{Name: "Item1", Amount: 3, Price: 0.33},
			},
			expected: 0.99,
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.items.IsTotalValid(tt.expected); got != tt.want {
				t.Errorf("Items.IsTotalValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ============================================================================
// Tests for constants
// ============================================================================

func TestConstants(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"GmailLabelId is set", GmailLabelId, "Label_2031551581397134603"},
		{"TicketHeaderRows is 7", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.want != "" && tt.got != tt.want {
				t.Errorf("%s: got %v, want %v", tt.name, tt.got, tt.want)
			}
		})
	}

	if TicketHeaderRows != 7 {
		t.Errorf("TicketHeaderRows = %v, want 7", TicketHeaderRows)
	}
}

// ============================================================================
// Tests for utils functions used by mercadona
// ============================================================================

func TestUtils_StringToFloat(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		prec     int
		expected float64
	}{
		{"dot decimal", "25.50", 2, 25.50},
		{"comma decimal", "10,50", 2, 10.50},
		{"integer", "100", 2, 100.00},
		{"zero", "0.00", 2, 0.00},
		{"negative", "-5.50", 2, -5.50},
		{"negative comma", "-5,50", 2, -5.50},
		{"large number", "1234.56", 2, 1234.56},
		{"large comma", "1234,56", 2, 1234.56},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := utils.StringToFloat(tt.input, tt.prec)
			if got != tt.expected {
				t.Errorf("StringToFloat(%q, %d) = %v, want %v", tt.input, tt.prec, got, tt.expected)
			}
		})
	}
}

func TestUtils_ParseQty(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected float64
	}{
		{"whole number", "1.000", 1.0},
		{"two units", "2.000", 2.0},
		{"three units", "3.000", 3.0},
		{"zero", "0.000", 0.0},
		{"decimal", "1.500", 1.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := utils.ParseQty(tt.input)
			if got != tt.expected {
				t.Errorf("ParseQty(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestUtils_ParsePrice(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected float64
	}{
		{"comma decimal", "5,50", 5.50},
		{"comma zero cents", "100,00", 100.00},
		{"comma small", "0,99", 0.99},
		{"comma large", "999,99", 999.99},
		{"negative comma", "-5,50", -5.50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := utils.ParsePrice(tt.input)
			if got != tt.expected {
				t.Errorf("ParsePrice(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestUtils_FloatsEqual(t *testing.T) {
	tests := []struct {
		name     string
		a        float64
		b        float64
		expected bool
	}{
		{"equal", 5.50, 5.50, true},
		{"equal zero", 0.00, 0.00, true},
		{"equal large", 1234.56, 1234.56, true},
		{"not equal", 5.50, 5.51, false},
		{"not equal small diff", 10.00, 10.01, false},
		{"negative equal", -5.50, -5.50, true},
		{"negative not equal", -5.50, -5.51, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := utils.FloatsEqual(tt.a, tt.b)
			if got != tt.expected {
				t.Errorf("FloatsEqual(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.expected)
			}
		})
	}
}

func TestUtils_ToFixed(t *testing.T) {
	tests := []struct {
		name     string
		input    float64
		prec     int
		expected float64
	}{
		{"round up", 5.555, 2, 5.56},
		{"round down", 5.554, 2, 5.55},
		{"no rounding", 5.50, 2, 5.50},
		{"zero precision", 5.555, 0, 6.0},
		{"zero precision round down", 5.499, 0, 5.0},
		{"negative", -5.555, 2, -5.56},
		{"zero", 0.005, 2, 0.01},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := utils.ToFixed(tt.input, tt.prec)
			if got != tt.expected {
				t.Errorf("ToFixed(%v, %d) = %v, want %v", tt.input, tt.prec, got, tt.expected)
			}
		})
	}
}

func TestUtils_ParseQtyWithPrecision(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		prec     int
		expected float64
	}{
		{"precision 3", "2.000", 3, 2.0},
		{"precision 2", "1.50", 2, 1.5},
		{"precision 1", "3.0", 1, 3.0},
		{"precision 0", "5", 0, 5.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := utils.ParseQtyWithPrecision(tt.input, tt.prec)
			if got != tt.expected {
				t.Errorf("ParseQtyWithPrecision(%q, %d) = %v, want %v", tt.input, tt.prec, got, tt.expected)
			}
		})
	}
}

// ============================================================================
// Edge case tests for mercadona functions
// ============================================================================

func Test_getTicketId_MultipleSpaces(t *testing.T) {
	// Test with multiple consecutive spaces - last word is still extracted
	message := &gmail.Message{Snippet: "Ticket    ABC-123"}
	got := getTicketId(message)
	// Multiple spaces will result in empty strings between them
	if got != "ABC-123" {
		t.Errorf("getTicketId() with multiple spaces = %v, want ABC-123", got)
	}
}

func Test_getDateFromTicket_YearBoundary(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		wantYear int
		wantErr  bool
	}{
		{"year 2000", "20000101 Mercadona 10.00.pdf", 2000, false},
		{"year 2025", "20251231 Mercadona 10.00.pdf", 2025, false},
		{"year 1999", "19990615 Mercadona 10.00.pdf", 1999, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			message := &gmail.Message{
				Payload: &gmail.MessagePart{
					Parts: []*gmail.MessagePart{
						nil,
						{Filename: tt.filename},
					},
				},
			}
			got, err := getDateFromTicket(message)
			if (err != nil) != tt.wantErr {
				t.Errorf("getDateFromTicket() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got.Year() != tt.wantYear {
				t.Errorf("getDateFromTicket() year = %v, want %v", got.Year(), tt.wantYear)
			}
		})
	}
}

func Test_getTicketTotal_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     float64
	}{
		{"very small total", "20231115 Mercadona 0.01.pdf", 0.01},
		{"very large total", "20231115 Mercadona 9999.99.pdf", 9999.99},
		{"comma no cents", "20231115 Mercadona 100,00.pdf", 100.00},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			message := &gmail.Message{
				Payload: &gmail.MessagePart{
					Parts: []*gmail.MessagePart{
						nil,
						{Filename: tt.filename},
					},
				},
			}
			got := getTicketTotal(message)
			if got != tt.want {
				t.Errorf("getTicketTotal() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_isTotalCorrect_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		qty      float64
		pricePer float64
		total    float64
		want     bool
	}{
		{"very small qty", 0.001, 100.00, 0.10, true},
		{"very large qty", 1000, 0.01, 10.00, true},
		{"very large price", 1, 999999.99, 999999.99, true},
		{"both zero", 0, 0, 0, true},
		{"qty one price one", 1, 1, 1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isTotalCorrect(tt.qty, tt.pricePer, tt.total)
			if got != tt.want {
				t.Errorf("isTotalCorrect(%v, %v, %v) = %v, want %v", tt.qty, tt.pricePer, tt.total, got, tt.want)
			}
		})
	}
}
