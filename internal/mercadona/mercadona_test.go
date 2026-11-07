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
		name     string
		snippet  string
		want     string
	}{
		{
			name:    "standard format with ticket ID",
			snippet: "Mercadona ticket ABC123XYZ",
			want:    "ABC123XYZ",
		},
		{
			name:    "numeric only snippet",
			snippet: "123456789",
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
			filename: "20231115 Mercadona 999.99.pdf",
			want:     999.99,
		},
		{
			name:     "zero total",
			filename: "20231115 Mercadona 0.00.pdf",
			want:     0.00,
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
			if !utils.FloatsEqual(got, tt.want) {
				t.Errorf("getTicketTotal() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ============================================================================
// Tests for getTicketTotal with malformed filename
// ============================================================================

func Test_getTicketTotal_MalformedFilename(t *testing.T) {
	message := &gmail.Message{
		Payload: &gmail.MessagePart{
			Parts: []*gmail.MessagePart{
				nil,
				{Filename: "invalid.pdf"},
			},
		},
	}
	got := getTicketTotal(message)
	if got != 0.0 {
		t.Errorf("getTicketTotal() for malformed filename = %v, want 0.0", got)
	}
}

// ============================================================================
// Tests for isTotalValid (Items method)
// ============================================================================

func Test_Items_IsTotalValid(t *testing.T) {
	tests := []struct {
		name        string
		items       internal.Items
		total       float64
		expected    bool
	}{
		{
			name:     "single item exact match",
			items:    internal.Items{{Name: "Milk", Amount: 2, Price: 1.50}},
			total:    3.00,
			expected: true,
		},
		{
			name:     "multiple items sum match",
			items:    internal.Items{{Name: "A", Amount: 1, Price: 2.50}, {Name: "B", Amount: 1, Price: 3.50}},
			total:    6.00,
			expected: true,
		},
		{
			name:     "total mismatch",
			items:    internal.Items{{Name: "A", Amount: 1, Price: 2.50}},
			total:    3.00,
			expected: false,
		},
		{
			name:     "empty items with zero total",
			items:    internal.Items{},
			total:    0.00,
			expected: true,
		},
		{
			name:     "floating point precision match",
			items:    internal.Items{{Name: "A", Amount: 3, Price: 0.33}},
			total:    0.99,
			expected: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.items.IsTotalValid(tt.total); got != tt.expected {
				t.Errorf("Items.IsTotalValid() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// ============================================================================
// Tests for Items_IsTotalValidWithPrecision
// ============================================================================

func Test_Items_IsTotalValidWithPrecision(t *testing.T) {
	tests := []struct {
		name        string
		items       internal.Items
		total       float64
		precision   float64
		expected    bool
	}{
		{
			name:      "exact match within precision",
			items:     internal.Items{{Name: "A", Amount: 1, Price: 2.50}},
			total:     2.50,
			precision: 0.01,
			expected:  true,
		},
		{
			name:      "slight deviation within precision delta",
			items:     internal.Items{{Name: "A", Amount: 1, Price: 2.50}},
			total:     2.505,
			precision: 0.02,
			expected:  true,
		},
		{
			name:      "deviation outside precision delta",
			items:     internal.Items{{Name: "A", Amount: 1, Price: 2.50}},
			total:     3.00,
			precision: 0.01,
			expected:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.items.IsTotalValidWithPrecision(tt.total, tt.precision); got != tt.expected {
				t.Errorf("Items.IsTotalValidWithPrecision() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// ============================================================================
// Tests for ToEntryItem (CastableItem interface)
// ============================================================================

func Test_ToEntryItem(t *testing.T) {
	items := internal.Items{
		{Name: "Milk", Amount: 2, Price: 1.50},
		{Name: "Bread", Amount: 1, Price: 2.00},
	}
	entryItems := items.ToEntryItem()

	if len(entryItems) != 2 {
		t.Errorf("Expected 2 entry items, got %d", len(entryItems))
	}
	if entryItems[0].Name != "Milk" || entryItems[0].Amount != 2 {
		t.Errorf("First entry item mismatch: got %+v", entryItems[0])
	}
	if entryItems[1].Name != "Bread" || entryItems[1].Price != 2.00 {
		t.Errorf("Second entry item mismatch: got %+v", entryItems[1])
	}
}

// ============================================================================
// Tests for ToEntry (CastableTicket interface)
// ============================================================================

func Test_Ticket_ToEntry(t *testing.T) {
	ticket := internal.Ticket{
		Id:    "T001",
		Total: 25.50,
		Store: "MERCADONA",
		Date:  time.Date(2024, 4, 15, 0, 0, 0, 0, time.UTC),
		Items: internal.Items{
			{Name: "Milk", Amount: 2, Price: 1.50},
			{Name: "Bread", Amount: 1, Price: 2.00},
		},
	}

	entry := ticket.ToEntry(10)

	if entry.Id != "T001" {
		t.Errorf("Entry Id = %v, want T001", entry.Id)
	}
	if entry.Total != 25.50 {
		t.Errorf("Entry Total = %v, want 25.50", entry.Total)
	}
	if entry.FirstRow != 10 {
		t.Errorf("Entry FirstRow = %v, want 10", entry.FirstRow)
	}
	if entry.LastRow != 12 {
		t.Errorf("Entry LastRow = %v, want 12", entry.LastRow)
	}
	if len(entry.Items) != 2 {
		t.Errorf("Entry Items count = %v, want 2", len(entry.Items))
	}
}

// ============================================================================
// Tests for IsExcluded (Ticket method)
// ============================================================================

func Test_Ticket_IsExcluded(t *testing.T) {
	ticket := internal.Ticket{Id: "T001"}

	tests := []struct {
		name     string
		ids      []string
		expected bool
	}{
		{
			name:     "ticket ID in excluded list",
			ids:      []string{"T001", "T002"},
			expected: true,
		},
		{
			name:     "ticket ID not in excluded list",
			ids:      []string{"T003", "T004"},
			expected: false,
		},
		{
			name:     "empty excluded list",
			ids:      []string{},
			expected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ticket.IsExcluded(tt.ids); got != tt.expected {
				t.Errorf("Ticket.IsExcluded() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// ============================================================================
// Skip-able browser tests (require actual Gmail/PDF access)
// ============================================================================

func TestGetTicketList_SkipNoBrowser(t *testing.T) {
	t.Skip("Skipping GetTicketList test - requires Gmail access")
}

func Test_getTicketDetails_SkipNoBrowser(t *testing.T) {
	t.Skip("Skipping getTicketDetails test - requires PDF parsing")
}

func Test_getTicketItems_SkipNoBrowser(t *testing.T) {
	t.Skip("Skipping getTicketItems test - requires PDF attachment access")
}
