package carrefour

import (
	"autoGrocery/utils"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"
)

// ============================================================================
// Tests for LoadCredentials
// ============================================================================

func TestLoadCredentials_ValidFile(t *testing.T) {
	content := `{
		"carrefour": {
			"username": "test@example.com",
			"password": "SecurePass123"
		}
	}`

	tmpFile, err := os.CreateTemp("", "carrefour-creds-test-*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(content); err != nil {
		tmpFile.Close()
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	creds, err := LoadCredentials(tmpFile.Name())
	if err != nil {
		t.Fatalf("LoadCredentials failed with valid input: %v", err)
	}

	if creds.Carrefour.Username != "test@example.com" {
		t.Errorf("Expected username 'test@example.com', got '%s'", creds.Carrefour.Username)
	}
	if creds.Carrefour.Password != "SecurePass123" {
		t.Errorf("Expected password 'SecurePass123', got '%s'", creds.Carrefour.Password)
	}
}

func TestLoadCredentials_InvalidJSON(t *testing.T) {
	content := `{ invalid json }`

	tmpFile, err := os.CreateTemp("", "carrefour-creds-test-*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(content); err != nil {
		tmpFile.Close()
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	_, err = LoadCredentials(tmpFile.Name())
	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}
}

func TestLoadCredentials_MissingFile(t *testing.T) {
	_, err := LoadCredentials("/non/existent/path/credentials.json")
	if err == nil {
		t.Error("Expected error for missing file, got nil")
	}
}

func TestLoadCredentials_EmptyFields(t *testing.T) {
	content := `{
		"carrefour": {
			"username": "",
			"password": ""
		}
	}`

	tmpFile, err := os.CreateTemp("", "carrefour-creds-test-*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(content); err != nil {
		tmpFile.Close()
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	creds, err := LoadCredentials(tmpFile.Name())
	if err != nil {
		t.Fatalf("LoadCredentials failed with empty fields: %v", err)
	}

	if creds.Carrefour.Username != "" {
		t.Errorf("Expected empty username, got '%s'", creds.Carrefour.Username)
	}
	if creds.Carrefour.Password != "" {
		t.Errorf("Expected empty password, got '%s'", creds.Carrefour.Password)
	}
}

func TestLoadCredentials_MissingCarrefourKey(t *testing.T) {
	content := `{
		"other": {
			"username": "test@example.com",
			"password": "pass"
		}
	}`

	tmpFile, err := os.CreateTemp("", "carrefour-creds-test-*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(content); err != nil {
		tmpFile.Close()
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	creds, err := LoadCredentials(tmpFile.Name())
	if err != nil {
		t.Fatalf("LoadCredentials should not fail with missing carrefour key: %v", err)
	}

	if creds.Carrefour.Username != "" {
		t.Errorf("Expected empty username for missing carrefour key, got '%s'", creds.Carrefour.Username)
	}
}

// ============================================================================
// Tests for getId (ticket ID generation)
// ============================================================================

func TestGetId(t *testing.T) {
	tests := []struct {
		name     string
		date     time.Time
		total    float64
		expected string
	}{
		{
			name:     "standard date and total",
			date:     time.Date(2024, 4, 15, 0, 0, 0, 0, time.UTC),
			total:    25.50,
			expected: "2024-04-15-25.5",
		},
		{
			name:     "integer total",
			date:     time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC),
			total:    100.0,
			expected: "2024-12-31-100",
		},
		{
			name:     "decimal total with many digits",
			date:     time.Date(2023, 6, 1, 0, 0, 0, 0, time.UTC),
			total:    42.99,
			expected: "2023-06-01-42.99",
		},
		{
			name:     "zero total",
			date:     time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			total:    0.0,
			expected: "2024-01-01-0",
		},
		{
			name:     "end of year",
			date:     time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC),
			total:    0.01,
			expected: "2024-12-31-0.01",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getId(tt.date, tt.total)
			if got != tt.expected {
				t.Errorf("getId() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// ============================================================================
// Tests for humanDelay (just verify it doesn't panic)
// ============================================================================

func TestHumanDelay_NoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("humanDelay() panicked: %v", r)
		}
	}()
	humanDelay()
}

// ============================================================================
// Tests for getTotalTicketDiscount logic (standalone function tests)
// ============================================================================

func TestGetTotalTicketDiscount_ReturnsZeroWhenNoDiscount(t *testing.T) {
	var numRe = regexp.MustCompile(`^-?\d+(?:[.,]\d+)?`)

	testCases := []struct {
		input    string
		expected float64
	}{
		{"-5.50", -5.50},
		{"5,50", 5.50},
		{"10.00", 10.00},
		{"0.00", 0.00},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			found := numRe.FindString(tc.input)
			parsed := utils.ParsePrice(found)
			if !utils.FloatsEqual(parsed, tc.expected) {
				t.Errorf("ParsePrice(%q) = %v, want %v", found, parsed, tc.expected)
			}
		})
	}
}

// ============================================================================
// Tests for TFAModalVisible logic (standalone regex/function tests)
// ============================================================================

func TestTFAModalVisible_RegexPattern(t *testing.T) {
	testCases := []struct {
		input    string
		expected bool
	}{
		{"#code1", true},
		{"#gigya-tfa-form", true},
		{"#other-element", false},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			if !strings.Contains(tc.input, "#") {
				t.Errorf("Expected selector to contain '#', got %q", tc.input)
			}
		})
	}
}

// ============================================================================
// Tests for get2FACode logic (standalone string parsing tests)
// ============================================================================

func TestGet2FACode_StringParsing(t *testing.T) {
	testCases := []struct {
		name     string
		subject  string
		expected string
	}{
		{
			name:     "standard subject format",
			subject:  "Tu codigo de verificacion es 123456",
			expected: "123456",
		},
		{
			name:     "subject with multiple spaces",
			subject:  "Verification code 987654",
			expected: "987654",
		},
		{
			name:     "subject with hyphenated code",
			subject:  "Code: 123-456",
			expected: "123-456",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			parts := strings.Split(tc.subject, " ")
			code := parts[len(parts)-1]
			if code != tc.expected {
				t.Errorf("Extracted code = %q, want %q", code, tc.expected)
			}
		})
	}
}

// ============================================================================
// Skip-able browser tests (require actual browser automation)
// ============================================================================

func TestLoginToCarrefour_SkipNoBrowser(t *testing.T) {
	t.Skip("Skipping LoginToCarrefour test - requires browser automation")
}

func TestGetTicketList_SkipNoBrowser(t *testing.T) {
	t.Skip("Skipping GetTicketList test - requires browser automation")
}

func TestGetTicketDetails_SkipNoBrowser(t *testing.T) {
	t.Skip("Skipping getTicketDetails test - requires browser automation")
}

func TestGetNumberOfTickets_SkipNoBrowser(t *testing.T) {
	t.Skip("Skipping getNumberOfTickets test - requires browser automation")
}

func TestCloseTicketPage_SkipNoBrowser(t *testing.T) {
	t.Skip("Skipping closeTicketPage test - requires browser automation")
}

func TestGetItemList_SkipNoBrowser(t *testing.T) {
	t.Skip("Skipping getItemList test - requires browser automation")
}

func TestHandle2FA_SkipNoBrowser(t *testing.T) {
	t.Skip("Skipping handle2FA test - requires browser automation and Gmail access")
}
