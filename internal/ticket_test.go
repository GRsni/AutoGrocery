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

// --- IsExcluded ---

func TestTicketIsExcluded(t *testing.T) {
	tests := []struct {
		name     string
		ticket   Ticket
		ids      []string
		expected bool
	}{
		{"ticket ID in excluded list", Ticket{Id: "T001"}, []string{"T001", "T002"}, true},
		{"ticket ID not in excluded list", Ticket{Id: "T003"}, []string{"T001", "T002"}, false},
		{"empty excluded list", Ticket{Id: "T001"}, []string{}, false},
		{"nil excluded list", Ticket{Id: "T001"}, nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ticket.IsExcluded(tt.ids); got != tt.expected {
				t.Errorf("Ticket.IsExcluded(%v) = %v, want %v", tt.ids, got, tt.expected)
			}
		})
	}
}

// --- IsTotalValidWithPrecision ---

func TestIsTotalValidWithPrecision(t *testing.T) {
	tests := []struct {
		name      string
		items     Items
		total     float64
		precision float64
		want      bool
	}{
		{
			name:      "exact match within precision",
			items:     Items{{Name: "A", Amount: 1, Price: 2.50}},
			total:     2.50,
			precision: 0.01,
			want:      true,
		},
		{
			name:      "slight deviation within precision delta",
			items:     Items{{Name: "A", Amount: 1, Price: 2.50}},
			total:     2.505,
			precision: 0.02,
			want:      true,
		},
		{
			name:      "deviation outside precision delta",
			items:     Items{{Name: "A", Amount: 1, Price: 2.50}},
			total:     3.00,
			precision: 0.01,
			want:      false,
		},
		{
			name:      "floating point precision with delta",
			items:     Items{{Name: "A", Amount: 3, Price: 0.33}},
			total:     0.99,
			precision: 0.01,
			want:      true,
		},
		{
			name:      "zero items zero total with precision",
			items:     Items{},
			total:     0.00,
			precision: 0.01,
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.items.IsTotalValidWithPrecision(tt.total, tt.precision); got != tt.want {
				t.Errorf("Items.IsTotalValidWithPrecision(%v, %v) = %v, want %v", tt.total, tt.precision, got, tt.want)
			}
		})
	}
}

// --- ToEntryItem ---

func TestToEntryItem(t *testing.T) {
	items := Items{
		{Name: "Milk", Amount: 2, Price: 1.50},
		{Name: "Bread", Amount: 1, Price: 2.00},
	}
	entryItems := items.ToEntryItem()

	if len(entryItems) != 2 {
		t.Fatalf("ToEntryItem() returned %d items, want 2", len(entryItems))
	}
	if entryItems[0].Name != "Milk" || entryItems[0].Amount != 2 || entryItems[0].Price != 1.50 {
		t.Errorf("First entry item = %+v, want {Name: Milk, Amount: 2, Price: 1.50}", entryItems[0])
	}
	if entryItems[1].Name != "Bread" || entryItems[1].Amount != 1 || entryItems[1].Price != 2.00 {
		t.Errorf("Second entry item = %+v, want {Name: Bread, Amount: 1, Price: 2.00}", entryItems[1])
	}
}

func TestToEntryItem_Empty(t *testing.T) {
	items := Items{}
	entryItems := items.ToEntryItem()
	if len(entryItems) != 0 {
		t.Errorf("ToEntryItem(empty) returned %d items, want 0", len(entryItems))
	}
}

// --- ToEntry ---

func TestTicketToEntry(t *testing.T) {
	ticket := Ticket{
		Id:    "T001",
		Total: 25.50,
		Store: "MERCADONA",
		Date:  time.Date(2024, 4, 15, 0, 0, 0, 0, time.UTC),
		Items: Items{
			{Name: "Milk", Amount: 2, Price: 1.50},
			{Name: "Bread", Amount: 1, Price: 2.00},
		},
	}

	entry := ticket.ToEntry(10)

	if entry.Id != "T001" {
		t.Errorf("Entry Id = %q, want T001", entry.Id)
	}
	if entry.Store != "MERCADONA" {
		t.Errorf("Entry Store = %q, want MERCADONA", entry.Store)
	}
	if !entry.Date.Equal(ticket.Date) {
		t.Errorf("Entry Date = %v, want %v", entry.Date, ticket.Date)
	}
	if entry.FirstRow != 10 {
		t.Errorf("Entry FirstRow = %d, want 10", entry.FirstRow)
	}
	if entry.LastRow != 12 { // 10 + len(items)
		t.Errorf("Entry LastRow = %d, want 12", entry.LastRow)
	}
	if entry.Total != 25.50 {
		t.Errorf("Entry Total = %v, want 25.50", entry.Total)
	}
	if len(entry.Items) != 2 {
		t.Errorf("Entry Items count = %d, want 2", len(entry.Items))
	}
	if entry.Items[0].Name != "Milk" || entry.Items[1].Name != "Bread" {
		t.Errorf("Entry items names = %+v, want [Milk Bread]", entry.Items)
	}
}

func TestTicketToEntry_ZeroItems(t *testing.T) {
	ticket := Ticket{
		Id:    "T002",
		Total: 0.00,
		Store: "DIA",
		Date:  time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
		Items: Items{},
	}

	entry := ticket.ToEntry(5)

	if entry.FirstRow != 5 {
		t.Errorf("Entry FirstRow = %d, want 5", entry.FirstRow)
	}
	if entry.LastRow != 5 { // 5 + len(0 items)
		t.Errorf("Entry LastRow = %d, want 5 (no items)", entry.LastRow)
	}
	if len(entry.Items) != 0 {
		t.Errorf("Entry Items count = %d, want 0", len(entry.Items))
	}
}

func TestTicketToEntry_NonZeroStartRow(t *testing.T) {
	ticket := Ticket{
		Id:    "T003",
		Total: 10.00,
		Store: "CARREFOUR",
		Date:  time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		Items: Items{{Name: "Item1", Amount: 1, Price: 10.00}},
	}

	entry := ticket.ToEntry(20)

	if entry.FirstRow != 20 {
		t.Errorf("Entry FirstRow = %d, want 20", entry.FirstRow)
	}
	if entry.LastRow != 21 { // 20 + 1 item
		t.Errorf("Entry LastRow = %d, want 21", entry.LastRow)
	}
}

