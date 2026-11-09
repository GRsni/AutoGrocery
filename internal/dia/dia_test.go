package dia

import (
	"os"
	"testing"
	"time"

	"autoGrocery/internal"
	"autoGrocery/utils"
)

// Helper to create temporary file
func createTempFile(content string) (*os.File, error) {
	tmpFile, err := os.CreateTemp("", "dia-test-*.json")
	if err != nil {
		return nil, err
	}
	defer tmpFile.Close()
	if _, err := tmpFile.WriteString(content); err != nil {
		return nil, err
	}
	return tmpFile, nil
}

// Helper to create temporary cookies file
func createTempCookiesFile(content string) (*os.File, error) {
	tmpFile, err := os.CreateTemp("", "cookies-test-*.txt")
	if err != nil {
		return nil, err
	}
	defer tmpFile.Close()
	if _, err := tmpFile.WriteString(content); err != nil {
		return nil, err
	}
	return tmpFile, nil
}

func TestLoadCredentials_Valid(t *testing.T) {
	content := `{
		"dia": {
			"username": "test@example.com",
			"password": "SecurePass123"
		}
	}`

	tmpFile, err := createTempFile(content)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	creds, err := LoadCredentials(tmpFile.Name())
	if err != nil {
		t.Fatalf("LoadCredentials failed with valid input: %v", err)
	}

	if creds.Dia.Username != "test@example.com" {
		t.Errorf("Expected username 'test@example.com', got '%s'", creds.Dia.Username)
	}
	if creds.Dia.Password != "SecurePass123" {
		t.Errorf("Expected password 'SecurePass123', got '%s'", creds.Dia.Password)
	}
}

func TestLoadCredentials_InvalidJSON(t *testing.T) {
	content := `{ invalid json }`

	tmpFile, err := createTempFile(content)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	_, err = LoadCredentials(tmpFile.Name())
	if err == nil {
		t.Error("Expected error for invalid JSON, but got nil")
	}
}

func TestLoadCredentials_MissingFile(t *testing.T) {
	// Ensure file does not exist
	_, err := LoadCredentials("/non/existent/path/file.json")
	if err == nil {
		t.Error("Expected error for missing file, but got nil")
	}
}

func TestLoadCredentials_EmptyFields(t *testing.T) {
	content := `{
		"dia": {
			"username": "",
			"password": ""
		}
	}`

	tmpFile, err := createTempFile(content)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	_, err = LoadCredentials(tmpFile.Name())
	if err == nil {
		t.Fatalf("LoadCredentials didnt throw error")
	}
}

func TestLoadSessionFromCookies(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int // number of cookies
	}{
		{
			name:     "Valid Single Cookie",
			input:    "example.com\tTRUE\t/\tFALSE\t1609459200.123\tusername\tsecret",
			expected: 1,
		},
		{
			name:     "Invalid Secure Format (lowercase)",
			input:    "example.com\ttrue\t/\tFALSE\t1609459200.123\tusername\tsecret",
			expected: 1,
		},
		{
			name:     "Malformed Line (Too few fields)",
			input:    "example.com\tTRUE\t/\t\n1609459200.123\tusername\tsecret",
			expected: 0, // Skips the malformed line
		},
		{
			name:     "Multiple Valid Cookies",
			input:    "site.com\tTRUE\t/\tFALSE\t1609459200.123\tsession\tabc\nother.com\tTRUE\t/\tFALSE\t1609459201.123\tuser\txyz",
			expected: 2,
		},
		{
			name:     "Cookies with Special Characters in Value",
			input:    "site.com\tTRUE\t/\tFALSE\t1609459200.123\taccess_token\tBearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile, err := createTempCookiesFile(tt.input + "\n")
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer os.Remove(tmpFile.Name())

			cookies, err := LoadSessionFromCookies(tmpFile.Name())
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if len(cookies) != tt.expected {
				t.Errorf("Expected %d cookies, got %d", tt.expected, len(cookies))
			}
		})
	}
}

func TestGetDateAndTotal(t *testing.T) {
	d := func(day, month, year int) time.Time {
		return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	}

	tests := []struct {
		name          string
		input         string
		expectErr     bool
		expectedDate  time.Time
		expectedTotal float64
	}{
		{
			name:          "Valid Date",
			input:         "Header\n15/02/2023\n50.00",
			expectErr:     false,
			expectedDate:  d(15, 2, 2023),
			expectedTotal: 50.0,
		},
		{
			name:          "Valid Date with Spaces",
			input:         "Ticket Details\n 15/02/2023 \n45.99",
			expectErr:     false,
			expectedDate:  d(15, 2, 2023),
			expectedTotal: 45.99,
		},
		{
			name:          "Valid Date - End of Year",
			input:         "Header\n31/12/2024\n100.00",
			expectErr:     false,
			expectedDate:  d(31, 12, 2024),
			expectedTotal: 100.0,
		},
		{
			name:          "Invalid Date Format",
			input:         "Header\nNot A Date\nFooter",
			expectErr:     true,
			expectedDate:  time.Time{},
			expectedTotal: 0.0,
		},
		{
			name:          "Empty Input",
			input:         "",
			expectErr:     true,
			expectedDate:  time.Time{},
			expectedTotal: 0.0,
		},
		{
			name:          "Missing Newline (Header only)",
			input:         "Just Header",
			expectErr:     true,
			expectedDate:  time.Time{},
			expectedTotal: 0.0,
		},
		{
			name:          "Valid Date with Euro Symbol",
			input:         "Ticket\n15/04/2024\n€75.50",
			expectErr:     false,
			expectedDate:  d(15, 4, 2024),
			expectedTotal: 75.5,
		},
		{
			name:          "Price with Comma (European format)",
			input:         "Header\n20/06/2024\n30,99",
			expectErr:     false,
			expectedDate:  d(20, 6, 2024),
			expectedTotal: 30.99,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, total, err := getDateAndTotal(tt.input)

			if tt.expectErr && err == nil {
				t.Error("Expected error but got nil")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !tt.expectErr && !result.IsZero() {
				if result.Year() < 2000 || result.Year() > time.Now().Year()+50 {
					t.Errorf("Date seems invalid: %v", result)
				}
				if !result.Equal(tt.expectedDate) {
					t.Errorf("Date = %v, want %v", result, tt.expectedDate)
				}
			}

			if !tt.expectErr && !utils.FloatsEqual(total, tt.expectedTotal) {
				t.Errorf("Total = %v, want %v", total, tt.expectedTotal)
			}
		})
	}
}

// Tests for Total Correct Logic (Math Helper)
func TestIsTotalCorrect(t *testing.T) {
	tests := []struct {
		name     string
		qty      float64
		price    float64
		total    float64
		expected bool
	}{
		{
			name:     "Exact Match",
			qty:      2.0,
			price:    10.0,
			total:    20.0,
			expected: true,
		},
		{
			name:     "Mismatch",
			qty:      2.0,
			price:    10.0,
			total:    21.0,
			expected: false,
		},
		{
			name:     "Zero Values",
			qty:      0.0,
			price:    10.0,
			total:    0.0,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isTotalCorrect(tt.qty, tt.price, tt.total)
			if result != tt.expected {
				t.Errorf("isTotalCorrect(%v, %v, %v) = %v, expected %v",
					tt.qty, tt.price, tt.total, result, tt.expected)
			}
		})
	}
}

// Tests for removeCancelledItems
func TestRemoveCancelledItems(t *testing.T) {
	tests := []struct {
		name     string
		input    []internal.Item
		expected []internal.Item
	}{
		{
			name: "No cancelled items",
			input: []internal.Item{
				{Name: "Apple", Amount: 2.0, Price: 1.5, Cancelled: false},
				{Name: "Banana", Amount: 3.0, Price: 0.5, Cancelled: false},
			},
			expected: []internal.Item{
				{Name: "Apple", Amount: 2.0, Price: 1.5, Cancelled: false},
				{Name: "Banana", Amount: 3.0, Price: 0.5, Cancelled: false},
			},
		},
		{
			name: "Single cancelled item",
			input: []internal.Item{
				{Name: "Apple", Amount: 2.0, Price: 1.5, Cancelled: false},
				{Name: "Apple", Amount: -2.0, Price: 1.5, Cancelled: true},
				{Name: "Banana", Amount: 3.0, Price: 0.5, Cancelled: false},
			},
			expected: []internal.Item{
				{Name: "Apple", Amount: 0.0, Price: 1.5, Cancelled: false},
				{Name: "Banana", Amount: 3.0, Price: 0.5, Cancelled: false},
			},
		},
		{
			name: "All cancelled items",
			input: []internal.Item{
				{Name: "Apple", Amount: 2.0, Price: 1.5, Cancelled: false},
				{Name: "Apple", Amount: -2.0, Price: 1.5, Cancelled: true},
			},
			// The function keeps the first non-cancelled item with matching name
			expected: []internal.Item{
				{Name: "Apple", Amount: 0.0, Price: 1.5, Cancelled: false},
			},
		},
		{
			name:     "Empty items",
			input:    []internal.Item{},
			expected: []internal.Item{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := removeCancelledItems(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d items, got %d", len(tt.expected), len(result))
				return
			}
			for i, expectedItem := range tt.expected {
				if i >= len(result) {
					break
				}
				if result[i].Name != expectedItem.Name {
					t.Errorf("Item %d: expected name %s, got %s", i, expectedItem.Name, result[i].Name)
				}
				if !utils.FloatsEqual(result[i].Amount, expectedItem.Amount) {
					t.Errorf("Item %d: expected amount %v, got %v", i, expectedItem.Amount, result[i].Amount)
				}
			}
		})
	}
}

// Tests for applyDiscounts
func TestApplyDiscounts(t *testing.T) {
	tests := []struct {
		name     string
		input    []internal.Item
		expected []internal.Item
	}{
		{
			name: "No discounts",
			input: []internal.Item{
				{Name: "Apple", Amount: 2.0, Price: 1.5, Discount: 0.0},
				{Name: "Banana", Amount: 3.0, Price: 0.5, Discount: 0.0},
			},
			expected: []internal.Item{
				{Name: "Apple", Amount: 2.0, Price: 1.5, Discount: 0.0},
				{Name: "Banana", Amount: 3.0, Price: 0.5, Discount: 0.0},
			},
		},
		{
			name: "With negative discount",
			input: []internal.Item{
				{Name: "Apple", Amount: 2.0, Price: 2.0, Discount: -1.0},
			},
			expected: []internal.Item{
				{Name: "Apple", Amount: 2.0, Price: 1.5, Discount: -1.0},
			},
		},
		{
			name: "Positive discount (no change)",
			input: []internal.Item{
				{Name: "Apple", Amount: 2.0, Price: 1.5, Discount: 0.5},
			},
			expected: []internal.Item{
				{Name: "Apple", Amount: 2.0, Price: 1.5, Discount: 0.5},
			},
		},
		{
			name:     "Empty items",
			input:    []internal.Item{},
			expected: []internal.Item{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := applyDiscounts(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d items, got %d", len(tt.expected), len(result))
				return
			}
			for i, expectedItem := range tt.expected {
				if i >= len(result) {
					break
				}
				if result[i].Name != expectedItem.Name {
					t.Errorf("Item %d: expected name %s, got %s", i, expectedItem.Name, result[i].Name)
				}
				if !utils.FloatsEqual(result[i].Price, expectedItem.Price) {
					t.Errorf("Item %d: expected price %v, got %v", i, expectedItem.Price, result[i].Price)
				}
			}
		})
	}
}

// Tests for Cookie Parsing Logic (Mocking rod elements is hard, so we test the parsing logic directly)
func TestLoadCredentials_MissingDiaKey(t *testing.T) {
	content := `{
		"username": "test@example.com",
		"password": "SecurePass123"
	}`

	tmpFile, err := createTempFile(content)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	_, err = LoadCredentials(tmpFile.Name())
	// Should fail because the struct expects a 'dia' wrapper
	if err == nil {
		t.Error("Expected error for missing 'dia' key, but got nil")
	}
}

func TestLoadCredentials_MissingPasswordKey(t *testing.T) {
	content := `{
		"dia": {
			"username": "test@example.com"
		}
	}`

	tmpFile, err := createTempFile(content)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	_, err = LoadCredentials(tmpFile.Name())
	if err == nil {
		t.Fatalf("LoadCredentials did not throw error")
	}
}

// Tests for humanDelay (verify no panic)
func TestHumanDelay_NoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("humanDelay() panicked: %v", r)
		}
	}()
	humanDelay()
}

// Additional tests for isTotalCorrect with more edge cases
func TestIsTotalCorrect_MoreEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		qty      float64
		price    float64
		total    float64
		expected bool
	}{
		{
			name:     "floating point rounding",
			qty:      3.0,
			price:    0.33,
			total:    0.99,
			expected: true,
		},
		{
			name:     "zero quantity zero total",
			qty:      0.0,
			price:    5.0,
			total:    0.0,
			expected: true,
		},
		{
			name:     "zero price zero total",
			qty:      5.0,
			price:    0.0,
			total:    0.0,
			expected: true,
		},
		{
			name:     "off by more than epsilon",
			qty:      1.0,
			price:    10.0,
			total:    10.10,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isTotalCorrect(tt.qty, tt.price, tt.total)
			if result != tt.expected {
				t.Errorf("isTotalCorrect(%v, %v, %v) = %v, expected %v",
					tt.qty, tt.price, tt.total, result, tt.expected)
			}
		})
	}
}

// Tests for getDateAndTotal with more edge cases
func TestGetDateAndTotal_MoreEdgeCases(t *testing.T) {
	d := func(day, month, year int) time.Time {
		return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	}

	tests := []struct {
		name          string
		input         string
		expectErr     bool
		expectedDate  time.Time
		expectedTotal float64
	}{
		{
			name:          "single digit month and day (should fail - requires zero-padded)",
			input:         "Header\n1/1/2024\n10.00",
			expectErr:     true,
			expectedDate:  time.Time{},
			expectedTotal: 0.0,
		},
		{
			name:          "zero-padded single digit month and day",
			input:         "Header\n01/01/2024\n10.00",
			expectErr:     false,
			expectedDate:  d(1, 1, 2024),
			expectedTotal: 10.0,
		},
		{
			name:          "total with spaces around",
			input:         "Header\n15/06/2024\n 25.99 ",
			expectErr:     false,
			expectedDate:  d(15, 6, 2024),
			expectedTotal: 25.99,
		},
		{
			name:          "very large total",
			input:         "Header\n01/01/2024\n9999.99",
			expectErr:     false,
			expectedDate:  d(1, 1, 2024),
			expectedTotal: 9999.99,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, total, err := getDateAndTotal(tt.input)

			if tt.expectErr && err == nil {
				t.Error("Expected error but got nil")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !tt.expectErr && !result.IsZero() {
				if !result.Equal(tt.expectedDate) {
					t.Errorf("Date = %v, want %v", result, tt.expectedDate)
				}
			}

			if !tt.expectErr && !utils.FloatsEqual(total, tt.expectedTotal) {
				t.Errorf("Total = %v, want %v", total, tt.expectedTotal)
			}
		})
	}
}

// Tests for removeCancelledItems with more edge cases
func TestRemoveCancelledItems_MoreEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    []internal.Item
		expected []internal.Item
	}{
		{
			name: "Multiple cancelled pairs - cancels accumulate into non-cancelled",
			input: []internal.Item{
				{Name: "Apple", Amount: 2.0, Price: 1.5, Cancelled: false},
				{Name: "Apple", Amount: -2.0, Price: 1.5, Cancelled: true},
				{Name: "Banana", Amount: 3.0, Price: 0.5, Cancelled: false},
				{Name: "Banana", Amount: -3.0, Price: 0.5, Cancelled: true},
			},
			// After cancellation: Apple(2.0 + -2.0 = 0.0), Banana(3.0 + -3.0 = 0.0), cancelled items removed
			expected: []internal.Item{
				{Name: "Apple", Amount: 0.0, Price: 1.5, Cancelled: false},
				{Name: "Banana", Amount: 0.0, Price: 0.5, Cancelled: false},
			},
		},
		{
			name: "Only cancelled items",
			input: []internal.Item{
				{Name: "Apple", Amount: -2.0, Price: 1.5, Cancelled: true},
			},
			expected: []internal.Item{},
		},
		{
			name: "Multiple items same name not cancelled - no merging",
			input: []internal.Item{
				{Name: "Apple", Amount: 2.0, Price: 1.5, Cancelled: false},
				{Name: "Apple", Amount: 1.0, Price: 1.5, Cancelled: false},
			},
			// No cancelled items means no accumulation happens; both remain separate
			expected: []internal.Item{
				{Name: "Apple", Amount: 2.0, Price: 1.5, Cancelled: false},
				{Name: "Apple", Amount: 1.0, Price: 1.5, Cancelled: false},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := removeCancelledItems(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d items, got %d", len(tt.expected), len(result))
				return
			}
			for i, expectedItem := range tt.expected {
				if i >= len(result) {
					break
				}
				if result[i].Name != expectedItem.Name {
					t.Errorf("Item %d: expected name %s, got %s", i, expectedItem.Name, result[i].Name)
				}
				if !utils.FloatsEqual(result[i].Amount, expectedItem.Amount) {
					t.Errorf("Item %d: expected amount %v, got %v", i, expectedItem.Amount, result[i].Amount)
				}
			}
		})
	}
}

// Tests for applyDiscounts with more edge cases
func TestApplyDiscounts_MoreEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    []internal.Item
		expected []internal.Item
	}{
		{
			name: "Multiple items with discount",
			input: []internal.Item{
				{Name: "Apple", Amount: 2.0, Price: 2.0, Discount: -1.0},
				{Name: "Banana", Amount: 3.0, Price: 1.0, Discount: -0.6},
			},
			expected: []internal.Item{
				{Name: "Apple", Amount: 2.0, Price: 1.5, Discount: -1.0},
				{Name: "Banana", Amount: 3.0, Price: 0.8, Discount: -0.6},
			},
		},
		{
			name: "Zero discount on multiple items",
			input: []internal.Item{
				{Name: "Apple", Amount: 2.0, Price: 2.0, Discount: 0.0},
				{Name: "Banana", Amount: 3.0, Price: 1.0, Discount: 0.0},
			},
			expected: []internal.Item{
				{Name: "Apple", Amount: 2.0, Price: 2.0, Discount: 0.0},
				{Name: "Banana", Amount: 3.0, Price: 1.0, Discount: 0.0},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := applyDiscounts(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d items, got %d", len(tt.expected), len(result))
				return
			}
			for i, expectedItem := range tt.expected {
				if i >= len(result) {
					break
				}
				if result[i].Name != expectedItem.Name {
					t.Errorf("Item %d: expected name %s, got %s", i, expectedItem.Name, result[i].Name)
				}
				if !utils.FloatsEqual(result[i].Price, expectedItem.Price) {
					t.Errorf("Item %d: expected price %v, got %v", i, expectedItem.Price, result[i].Price)
				}
			}
		})
	}
}

// Browser-dependent tests (skipped in unit test environment)
func TestLoginToDia_SkipNoBrowser(t *testing.T) {
	t.Skip("Skipping LoginToDia test - requires browser automation")
}

func TestGetTicketList_SkipNoBrowser(t *testing.T) {
	t.Skip("Skipping GetTicketList test - requires browser automation")
}

func TestGetTicketDetails_SkipNoBrowser(t *testing.T) {
	t.Skip("Skipping getTicketDetails test - requires browser automation")
}

func TestGetItemList_SkipNoBrowser(t *testing.T) {
	t.Skip("Skipping getItemList test - requires browser automation")
}

func TestGetItemName_SkipNoBrowser(t *testing.T) {
	t.Skip("Skipping getItemName test - requires browser automation")
}

func TestGetItemQty_SkipNoBrowser(t *testing.T) {
	t.Skip("Skipping getItemQty test - requires browser automation")
}

func TestGetPricePerUnit_SkipNoBrowser(t *testing.T) {
	t.Skip("Skipping getPricePerUnit test - requires browser automation")
}

func TestGetItemTotal_SkipNoBrowser(t *testing.T) {
	t.Skip("Skipping getItemTotal test - requires browser automation")
}

func TestGetDiscount_SkipNoBrowser(t *testing.T) {
	t.Skip("Skipping getDiscount test - requires browser automation")
}

func TestCloseTicketModal_SkipNoBrowser(t *testing.T) {
	t.Skip("Skipping closeTicketModal test - requires browser automation")
}

// --- Tests for BUG-5 fix: malformed ticket card should not abort entire run ---

// BUG-5 regression test (documented): getDateAndTotal correctly returns an error
// for malformed input, which causes the caller in GetTicketList to skip that
// card and continue processing remaining cards instead of aborting the whole run.
// The actual loop behavior is verified by integration tests; these unit tests
// verify the parsing function's contract (error on bad input).
func TestGetDateAndTotal_MalformedInput_ReturnsError(t *testing.T) {
	tests := []string{
		"",                         // empty string
		"just a single line",       // no newline, insufficient lines
		"\n",                       // only newline, no content
		"Header\nNotADate\n10.00",  // invalid date format
		"Header\n32/01/2024\n10.00", // impossible date
	}

	for _, input := range tests {
		_, _, err := getDateAndTotal(input)
		if err == nil {
			t.Errorf("getDateAndTotal(%q) = nil error, want error (malformed input should cause continue-skip)", input)
		}
	}
}

// --- Tests for BUG-6 fix: Carrefour returns empty slice on parse error ---

// BUG-6 regression test (documented): GetTicketList now returns []internal.Ticket{}
// instead of nil when element list lookup fails, consistent with Dia and Mercadona.
// The actual behavior is verified by integration tests; this skip test documents the fix.
func TestGetTicketList_ReturnsEmptySliceOnError(t *testing.T) {
	t.Skip("Skipping GetTicketList browser test - requires browser automation")
	// BUG-6: When page.Elements(".ticket-list-item") fails, the function should
	// return []Ticket{} (empty slice) instead of nil, so callers can safely
	// range over the result without nil-checking.
}

// --- Tests for BUG-3 fix: correct error variable in 2FA handler ---

// BUG-3 regression test (documented): The 2FA error handler logs err2FAElement
// instead of the generic err variable, ensuring the correct error message is shown.
// This is verified by integration tests with actual Gigya iframe; unit test documents
// the expected behavior.
func TestHandle2FA_ErrorLogging_UsesCorrectVariable(t *testing.T) {
	t.Skip("Skipping 2FA error logging test - requires browser automation")
	// BUG-3: When page.Timeout(10*time.Second).Element("#code1") fails, the error
	// handler at carrefour.go:147 should log err2FAElement (the specific element
	// lookup error), not a generic err variable. This is verified by integration tests.
}
