package internal

import (
	"autoGrocery/pkg/constants"
	"fmt"
	"strings"
	"testing"
	"time"
)

var baseDate = time.Date(2024, 4, 15, 0, 0, 0, 0, time.UTC)

var singleItemTicket = Ticket{
	Id:    "T001",
	Total: 9.99,
	Store: constants.MERCADONA,
	Date:  baseDate,
	Items: []Item{
		{Name: "Milk", Amount: 2, Price: 1.50},
	},
}

var multiItemTicket = Ticket{
	Id:    "T002",
	Total: 25.00,
	Store: constants.DIA,
	Date:  baseDate,
	Items: []Item{
		{Name: "Bread", Amount: 1, Price: 0.90},
		{Name: "Eggs", Amount: 12, Price: 2.10},
		{Name: "Butter", Amount: 1, Price: 1.80},
	},
}

var singleItemWithPlus = Ticket{
	Id:    "T003",
	Total: 0.90,
	Store: constants.DIA,
	Date:  baseDate,
	Items: []Item{
		{Name: "+ Item", Amount: 1, Price: 0.90},
	},
}

var FreeTicket = Ticket{Id: "T003", Total: 0, Store: "DIA", Date: baseDate, Items: []Item{{Name: "Freebie", Amount: 1, Price: 0}}}

// --- Items.String ---

func TestItemsString_Empty(t *testing.T) {
	items := Items{}
	got := items.String()
	want := "[]"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestItemsString_SingleItem(t *testing.T) {
	items := Items{{Name: "Milk", Amount: 2, Price: 1.50}}
	got := items.String()
	want := "[Milk (x2.000 @ €1.500)]"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestItemsString_MultipleItems(t *testing.T) {
	items := Items{
		{Name: "Bread", Amount: 1, Price: 0.90},
		{Name: "Eggs", Amount: 12, Price: 2.10},
		{Name: "Butter", Amount: 1, Price: 1.80},
	}
	got := items.String()
	want := "[Bread (x1.000 @ €0.900), Eggs (x12.000 @ €2.100), Butter (x1.000 @ €1.800)]"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestItemsString_ItemsWithPlusSign(t *testing.T) {
	items := Items{{Name: "+ Item", Amount: 1, Price: 0.90}}
	got := items.String()
	want := "[+ Item (x1.000 @ €0.900)]"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestItemsString_CommaSeparated(t *testing.T) {
	items := Items{
		{Name: "Item A", Amount: 1, Price: 2.50},
		{Name: "Item B", Amount: 1, Price: 2.50},
	}
	got := items.String()
	if !strings.Contains(got, ", ") {
		t.Errorf("expected comma-separated items, got %q", got)
	}
}

// --- TicketToStr ---

func TestTicketToStr(t *testing.T) {
	tests := []struct {
		name     string
		ticket   Ticket
		store    string
		expected string
	}{
		{
			name:     "single item",
			ticket:   singleItemTicket,
			expected: fmt.Sprintf("MERCADONA Ticket: T001, total: €%.2f, items: %s", 9.99, fmt.Sprint(singleItemTicket.Items.String())),
		},
		{
			name:     "multiple items",
			ticket:   multiItemTicket,
			expected: fmt.Sprintf("DIA Ticket: T002, total: €%.2f, items: %s", 25.00, fmt.Sprint(multiItemTicket.Items.String())),
		},
		{
			name:     "zero total",
			ticket:   FreeTicket,
			expected: fmt.Sprintf("DIA Ticket: T003, total: €%.2f, items: %s", 0.0, fmt.Sprint(FreeTicket.Items.String())),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.ticket.TicketToStr()
			if got != tt.expected {
				t.Errorf("got %q, want %q", got, tt.expected)
			}
		})
	}
}

// --- ToValueRange ---

func TestToValueRange_EmptyItems_ReturnsError(t *testing.T) {
	ticket := Ticket{Id: "T000", Store: "DIA", Date: baseDate, Items: []Item{}}
	vr, err := ticket.ToValueRange()
	if err == nil {
		t.Error("expected error for empty items, got nil")
	}
	if vr != nil {
		t.Errorf("expected nil ValueRange on error, got %+v", vr)
	}
}

func TestToValueRange_MajorDimension(t *testing.T) {
	vr, err := singleItemTicket.ToValueRange()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if vr.MajorDimension != "ROWS" {
		t.Errorf("MajorDimension = %q, want %q", vr.MajorDimension, "ROWS")
	}
}

func TestToValueRange_RowCount(t *testing.T) {
	tests := []struct {
		name         string
		ticket       Ticket
		expectedRows int
	}{
		{"single item", singleItemTicket, 1},
		{"multiple items", multiItemTicket, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vr, err := tt.ticket.ToValueRange()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(vr.Values) != tt.expectedRows {
				t.Errorf("got %d rows, want %d", len(vr.Values), tt.expectedRows)
			}
		})
	}
}

func TestToValueRange_FirstRowMetadata(t *testing.T) {
	vr, err := singleItemTicket.ToValueRange()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	row := vr.Values[0]

	wantDate := baseDate.Format(constants.TicketDateFormat)
	if row[0] != wantDate {
		t.Errorf("row[0] date = %q, want %q", row[0], wantDate)
	}
	if row[1] != singleItemTicket.Store {
		t.Errorf("row[1] store = %q, want %q", row[1], singleItemTicket.Store)
	}
}

func TestToValueRange_ItemColumns(t *testing.T) {
	tests := []struct {
		name       string
		rowIndex   int
		wantName   string
		wantAmount float64
		wantPrice  float64
	}{
		{"first item", 0, "Bread", 1, 0.90},
		{"second item", 1, "Eggs", 12, 2.10},
		{"third item", 2, "Butter", 1, 1.80},
	}

	vr, err := multiItemTicket.ToValueRange()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row := vr.Values[tt.rowIndex]
			if row[2] != tt.wantName {
				t.Errorf("name = %q, want %q", row[2], tt.wantName)
			}
			if row[3] != tt.wantAmount {
				t.Errorf("amount = %v, want %v", row[3], tt.wantAmount)
			}
			if row[4] != tt.wantPrice {
				t.Errorf("price = %v, want %v", row[4], tt.wantPrice)
			}
		})
	}
}

func TestToValueRange_NonFirstRowsHaveEmptyMetadata(t *testing.T) {
	vr, err := multiItemTicket.ToValueRange()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for i := 1; i < len(vr.Values); i++ {
		row := vr.Values[i]
		if row[0] != "" || row[1] != "" {
			t.Errorf("row[%d] cols 0/1 should be empty, got %q %q", i, row[0], row[1])
		}
	}
}

func TestToValueRange_PlusSignIsHandled(t *testing.T) {
	vr, err := singleItemWithPlus.ToValueRange()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for i := 0; i < len(vr.Values); i++ {
		row := vr.Values[i]
		if len(row[2].(string)) == 0 || row[2].(string)[0] != '\'' {
			t.Errorf("row[%d] item name should start with ', got %q", i, row[2])
		}
	}
}

func Test_ticketIsValid(t *testing.T) {
	type args struct {
		total float64
		items Items
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "valid ticket - single item",
			args: args{
				total: 2.50,
				items: Items{{Name: "Milk", Amount: 1, Price: 2.50}},
			},
			want: true,
		},
		{
			name: "valid ticket - multiple items",
			args: args{
				total: 5.70,
				items: Items{
					{Name: "Milk", Amount: 1, Price: 2.50},
					{Name: "Bread", Amount: 2, Price: 1.60},
				},
			},
			want: true,
		},
		{
			name: "invalid ticket - total too high",
			args: args{
				total: 10.00,
				items: Items{{Name: "Milk", Amount: 1, Price: 2.50}},
			},
			want: false,
		},
		{
			name: "invalid ticket - total too low",
			args: args{
				total: 1.00,
				items: Items{{Name: "Milk", Amount: 2, Price: 2.50}},
			},
			want: false,
		},
		{
			name: "empty items with zero total",
			args: args{
				total: 0.00,
				items: Items{},
			},
			want: true,
		},
		{
			name: "floating point accumulation rounds correctly",
			args: args{
				total: 1.50,
				items: Items{
					{Name: "Item A", Amount: 1, Price: 0.50},
					{Name: "Item B", Amount: 1, Price: 0.50},
					{Name: "Item C", Amount: 1, Price: 0.50},
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.args.items.IsTotalValid(tt.args.total); got != tt.want {
				t.Errorf("ticketIsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}
