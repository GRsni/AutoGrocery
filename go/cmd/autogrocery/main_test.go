package main

import (
	"testing"
	"time"

	"autoGrocery/internal"
	"autoGrocery/internal/google/sh"
	"autoGrocery/pkg/constants"
)

// ---------------------------------------------------------------------------
// Tests for parseCurrentTimeOverride
// ---------------------------------------------------------------------------

func TestParseCurrentTimeOverride_CustomDate(t *testing.T) {
	dateStr := "15-06-2024"
	result := parseCurrentTimeOverride(&dateStr)
	expected := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)
	if !result.Equal(expected) {
		t.Errorf("parseCurrentTimeOverride(%q) = %v, want %v", dateStr, result, expected)
	}
}

func TestParseCurrentTimeOverride_InvalidDate(t *testing.T) {
	dateStr := "not-a-date"
	result := parseCurrentTimeOverride(&dateStr)
	// Should fall back to time.Now()
	if result.Year() != time.Now().Year() || result.Month() != time.Now().Month() {
		t.Errorf("parseCurrentTimeOverride(%q) = %v, expected fallback to current date", dateStr, result)
	}
}

func TestParseCurrentTimeOverride_Default(t *testing.T) {
	defaultDate := "01-01-0001"
	result := parseCurrentTimeOverride(&defaultDate)
	// Should fall back to time.Now() because the default is treated as "no override"
	if result.Year() != time.Now().Year() || result.Month() != time.Now().Month() {
		t.Errorf("parseCurrentTimeOverride(%q) = %v, expected fallback to current date", defaultDate, result)
	}
}

func TestParseCurrentTimeOverride_EdgeOfYear(t *testing.T) {
	tests := []struct {
		name     string
		dateStr  string
		wantYear int
		wantMonth time.Month
		wantDay  int
	}{
		{"start of year", "01-01-2024", 2024, 1, 1},
		{"end of year", "31-12-2024", 2024, 12, 31},
		{"february leap", "29-02-2024", 2024, 2, 29},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseCurrentTimeOverride(&tt.dateStr)
			if result.Year() != tt.wantYear || result.Month() != tt.wantMonth || result.Day() != tt.wantDay {
				t.Errorf("parseCurrentTimeOverride(%q) = %v, want %v/%v/%d", tt.dateStr, result, tt.wantYear, tt.wantMonth, tt.wantDay)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tests for parseStoreOverride
// ---------------------------------------------------------------------------

func TestParseStoreOverride_Empty(t *testing.T) {
	stores := ""
	result := parseStoreOverride(&stores)
	if len(result) != len(constants.Stores) {
		t.Errorf("parseStoreOverride(\"\") = %v, expected all stores", result)
	}
	for i, store := range constants.Stores {
		if result[i] != store {
			t.Errorf("parseStoreOverride(\"\")[%d] = %q, want %q", i, result[i], store)
		}
	}
}

func TestParseStoreOverride_SingleValidStore(t *testing.T) {
	stores := "MERCADONA"
	result := parseStoreOverride(&stores)
	if len(result) != 1 || result[0] != "MERCADONA" {
		t.Errorf("parseStoreOverride(%q) = %v, want [MERCADONA]", stores, result)
	}
}

func TestParseStoreOverride_MultipleValidStores(t *testing.T) {
	stores := "DIA,MERCADONA"
	result := parseStoreOverride(&stores)
	if len(result) != 2 || result[0] != "DIA" || result[1] != "MERCADONA" {
		t.Errorf("parseStoreOverride(%q) = %v, want [DIA MERCADONA]", stores, result)
	}
}

func TestParseStoreOverride_InvalidStoreIgnored(t *testing.T) {
	stores := "INVALID_STORE"
	result := parseStoreOverride(&stores)
	// Invalid store should fall back to default
	if len(result) != len(constants.Stores) {
		t.Errorf("parseStoreOverride(%q) = %v, expected fallback to all stores", stores, result)
	}
}

func TestParseStoreOverride_MixedValidAndInvalid(t *testing.T) {
	stores := "DIA,INVALID,CARREFOUR"
	result := parseStoreOverride(&stores)
	expected := []string{"DIA", "CARREFOUR"}
	if len(result) != len(expected) {
		t.Errorf("parseStoreOverride(%q) = %v, want %v", stores, result, expected)
	}
	for i, want := range expected {
		if result[i] != want {
			t.Errorf("parseStoreOverride(%q)[%d] = %q, want %q", stores, i, result[i], want)
		}
	}
}

func TestParseStoreOverride_WithSpaces(t *testing.T) {
	stores := "DIA,MERCADONA"
	result := parseStoreOverride(&stores)
	if len(result) != 2 || result[0] != "DIA" || result[1] != "MERCADONA" {
		t.Errorf("parseStoreOverride(%q) = %v, want [DIA MERCADONA]", stores, result)
	}
}

func TestParseStoreOverride_AllInvalid(t *testing.T) {
	stores := "INVALID1,INVALID2"
	result := parseStoreOverride(&stores)
	// Should fall back to default stores
	if len(result) != len(constants.Stores) {
		t.Errorf("parseStoreOverride(%q) = %v, expected fallback to all stores", stores, result)
	}
}

// ---------------------------------------------------------------------------
// Tests for getSheetName
// ---------------------------------------------------------------------------

func TestGetSheetName_TestMode(t *testing.T) {
	original := constants.TestMode
	constants.TestMode = true
	defer func() { constants.TestMode = original }()

	result := getSheetName(2024, time.June)
	if result != "testpage" {
		t.Errorf("getSheetName(2024, June) in test mode = %q, want %q", result, "testpage")
	}
}

func TestGetSheetName_ProdMode(t *testing.T) {
	original := constants.TestMode
	constants.TestMode = false
	defer func() { constants.TestMode = original }()

	result := getSheetName(2024, time.June)
	expected := "Junio 24"
	if result != expected {
		t.Errorf("getSheetName(2024, June) = %q, want %q", result, expected)
	}
}

func TestGetSheetName_ProdModeJanuary(t *testing.T) {
	original := constants.TestMode
	constants.TestMode = false
	defer func() { constants.TestMode = original }()

	result := getSheetName(2025, time.January)
	expected := "Enero 25"
	if result != expected {
		t.Errorf("getSheetName(2025, January) = %q, want %q", result, expected)
	}
}

func TestGetSheetName_ProdModeDecember(t *testing.T) {
	original := constants.TestMode
	constants.TestMode = false
	defer func() { constants.TestMode = original }()

	result := getSheetName(2024, time.December)
	expected := "Diciembre 24"
	if result != expected {
		t.Errorf("getSheetName(2024, December) = %q, want %q", result, expected)
	}
}

func TestGetSheetName_YearModulo(t *testing.T) {
	original := constants.TestMode
	constants.TestMode = false
	defer func() { constants.TestMode = original }()

	result := getSheetName(2099, time.March)
	expected := "Marzo 99"
	if result != expected {
		t.Errorf("getSheetName(2099, March) = %q, want %q", result, expected)
	}
}

// ---------------------------------------------------------------------------
// Tests for getLastEntryToCompare
// ---------------------------------------------------------------------------

func TestGetLastEntryToCompare_NilEntry(t *testing.T) {
	firstOfMonth := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	result := getLastEntryToCompare(nil, "MERCADONA", firstOfMonth)
	if !result.Date.Equal(firstOfMonth) || result.Total != 0 {
		t.Errorf("getLastEntryToCompare(nil) = %+v, want Date=%v Total=0", result, firstOfMonth)
	}
}

func TestGetLastEntryToCompare_HasEntry(t *testing.T) {
	entry := &sh.Entry{Date: time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC), Total: 25.50}
	firstOfMonth := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	result := getLastEntryToCompare(entry, "MERCADONA", firstOfMonth)
	if !result.Date.Equal(entry.Date) || result.Total != entry.Total {
		t.Errorf("getLastEntryToCompare(entry) = %+v, want Date=%v Total=%f", result, entry.Date, entry.Total)
	}
}

// ---------------------------------------------------------------------------
// Tests for CombineTickets
// ---------------------------------------------------------------------------

func TestCombineTickets_EmptyMap(t *testing.T) {
	result := CombineTickets(map[string][]internal.Ticket{})
	if len(result) != 0 {
		t.Errorf("CombineTickets(empty) = %v, want []", result)
	}
}

func TestCombineTickets_SingleStore(t *testing.T) {
	tickets := map[string][]internal.Ticket{
		"MERCADONA": {
			{Date: time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC), Id: "T1", Total: 25.50},
		},
	}
	result := CombineTickets(tickets)
	if len(result) != 1 || result[0].Id != "T1" {
		t.Errorf("CombineTickets(single store) = %+v, want [T1]", result)
	}
}

func TestCombineTickets_MultipleStoresSortedByDate(t *testing.T) {
	now := time.Now()
	tickets := map[string][]internal.Ticket{
		"DIA": {
			{Date: now.Add(24 * time.Hour), Id: "T2", Total: 10.00},
		},
		"MERCADONA": {
			{Date: now, Id: "T1", Total: 25.50},
		},
	}
	result := CombineTickets(tickets)
	if len(result) != 2 {
		t.Fatalf("CombineTickets(multiple stores) returned %d tickets, want 2", len(result))
	}
	// Should be sorted by date: T1 (now) before T2 (now+24h)
	if result[0].Id != "T1" || result[1].Id != "T2" {
		t.Errorf("CombineTickets(multiple stores) = [%s, %s], want [T1, T2]", result[0].Id, result[1].Id)
	}
}

func TestCombineTickets_MultipleTicketsPerStore(t *testing.T) {
	now := time.Now()
	tickets := map[string][]internal.Ticket{
		"MERCADONA": {
			{Date: now.Add(48 * time.Hour), Id: "T3", Total: 30.00},
			{Date: now, Id: "T1", Total: 25.50},
			{Date: now.Add(24 * time.Hour), Id: "T2", Total: 20.00},
		},
	}
	result := CombineTickets(tickets)
	if len(result) != 3 {
		t.Fatalf("CombineTickets(multiple per store) returned %d tickets, want 3", len(result))
	}
	for i, expectedId := range []string{"T1", "T2", "T3"} {
		if result[i].Id != expectedId {
			t.Errorf("CombineTickets()[%d] = %s, want %s", i, result[i].Id, expectedId)
		}
	}
}

// ---------------------------------------------------------------------------
// Tests for MergeAllTickets
// ---------------------------------------------------------------------------

func TestMergeAllTickets_Empty(t *testing.T) {
	result := MergeAllTickets([]sh.Entry{}, []internal.Ticket{})
	if len(result) != 0 {
		t.Errorf("MergeAllTickets(empty) = %v, want []", result)
	}
}

func TestMergeAllTickets_OnlySheetEntries(t *testing.T) {
	sheetEntries := []sh.Entry{
		{Date: time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC), Store: "MERCADONA", Id: "S1", Items: sh.Items{{Name: "Item1", Amount: 2, Price: 1.50}}},
	}
	result := MergeAllTickets(sheetEntries, []internal.Ticket{})
	if len(result) != 1 {
		t.Errorf("MergeAllTickets(only sheet) = %d entries, want 1", len(result))
	}
}

func TestMergeAllTickets_OnlyNewTickets(t *testing.T) {
	newTickets := []internal.Ticket{
		{Date: time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC), Id: "T1", Total: 25.50, Store: "MERCADONA", Items: internal.Items{{Name: "Apple", Amount: 2, Price: 1.50}}},
	}
	result := MergeAllTickets([]sh.Entry{}, newTickets)
	if len(result) != 1 {
		t.Errorf("MergeAllTickets(only new) = %d entries, want 1", len(result))
	}
	if result[0].Id != "T1" {
		t.Errorf("result[0].Id = %q, want T1", result[0].Id)
	}
	// Verify FirstRow and LastRow are set (HeaderSize=1)
	if result[0].FirstRow != 1 {
		t.Errorf("result[0].FirstRow = %d, want 1", result[0].FirstRow)
	}
	if result[0].LastRow != 2 { // FirstRow + len(items)
		t.Errorf("result[0].LastRow = %d, want 2", result[0].LastRow)
	}
}

func TestMergeAllTickets_CombinedAndSorted(t *testing.T) {
	sheetEntries := []sh.Entry{
		{Date: time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC), Store: "MERCADONA", Id: "S1", Items: sh.Items{{Name: "Item1", Amount: 2, Price: 1.50}}},
	}
	newTickets := []internal.Ticket{
		{Date: time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC), Id: "T1", Total: 10.00, Store: "DIA", Items: internal.Items{{Name: "Apple", Amount: 1, Price: 2.50}}},
	}
	result := MergeAllTickets(sheetEntries, newTickets)
	if len(result) != 2 {
		t.Fatalf("MergeAllTickets(combined) = %d entries, want 2", len(result))
	}
	// Should be sorted by date: T1 (June 1) before S1 (June 15)
	if result[0].Id != "T1" || result[1].Id != "S1" {
		t.Errorf("MergeAllTickets() = [%s, %s], want [T1, S1]", result[0].Id, result[1].Id)
	}
}

func TestMergeAllTickets_MultipleNewTicketsSorted(t *testing.T) {
	newTickets := []internal.Ticket{
		{Date: time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC), Id: "T3", Store: "DIA", Items: internal.Items{{Name: "C", Amount: 1, Price: 1.0}}},
		{Date: time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC), Id: "T1", Store: "MERCADONA", Items: internal.Items{{Name: "A", Amount: 1, Price: 2.0}}},
		{Date: time.Date(2024, 6, 10, 0, 0, 0, 0, time.UTC), Id: "T2", Store: "CARREFOUR", Items: internal.Items{{Name: "B", Amount: 1, Price: 3.0}}},
	}
	result := MergeAllTickets([]sh.Entry{}, newTickets)
	if len(result) != 3 {
		t.Fatalf("MergeAllTickets(multiple sorted) = %d entries, want 3", len(result))
	}
	for i, expectedId := range []string{"T1", "T2", "T3"} {
		if result[i].Id != expectedId {
			t.Errorf("result[%d].Id = %s, want %s", i, result[i].Id, expectedId)
		}
	}
	// Verify row numbers are sequential (HeaderSize=1, each entry gets its own row)
	if result[0].FirstRow != 1 || result[0].LastRow != 2 {
		t.Errorf("result[0] rows = (%d,%d), want (1,2)", result[0].FirstRow, result[0].LastRow)
	}
	if result[1].FirstRow != 3 || result[1].LastRow != 4 {
		t.Errorf("result[1] rows = (%d,%d), want (3,4)", result[1].FirstRow, result[1].LastRow)
	}
	if result[2].FirstRow != 5 || result[2].LastRow != 6 {
		t.Errorf("result[2] rows = (%d,%d), want (5,6)", result[2].FirstRow, result[2].LastRow)
	}
}

func TestMergeAllTickets_StableSortSameDate(t *testing.T) {
	sameDate := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)
	newTickets := []internal.Ticket{
		{Date: sameDate, Id: "T3", Store: "DIA", Items: internal.Items{{Name: "C", Amount: 1, Price: 1.0}}},
		{Date: sameDate, Id: "T1", Store: "MERCADONA", Items: internal.Items{{Name: "A", Amount: 1, Price: 2.0}}},
	}
	result := MergeAllTickets([]sh.Entry{}, newTickets)
	// SliceStable preserves original order for equal elements
	if result[0].Id != "T3" || result[1].Id != "T1" {
		t.Errorf("MergeAllTickets(stable sort) = [%s, %s], want [T3, T1]", result[0].Id, result[1].Id)
	}
}
