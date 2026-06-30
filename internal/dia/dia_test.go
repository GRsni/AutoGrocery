package dia

import (
	"os"
	"testing"
	"time"
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

func TestGetDateFromTicket(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expectErr bool
		expected  time.Time // Only useful if parsing succeeds
	}{
		{
			name:      "Valid Date",
			input:     "Header\n15/02/2023\nFooter",
			expectErr: false,
		},
		{
			name:      "Invalid Date Format",
			input:     "Header\nNot A Date\nFooter",
			expectErr: true,
		},
		{
			name:      "Empty Input",
			input:     "",
			expectErr: true,
		},
		{
			name:      "Missing Newline (Header only)",
			input:     "Just Header",
			expectErr: true,
		},
		{
			name:      "Valid Date with Spaces",
			input:     "Ticket Details\n 15/02/2023 \nEnd",
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := getDateFromTicket(tt.input)

			if tt.expectErr && err == nil {
				t.Error("Expected error but got nil")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// Optional: Check if the date is reasonable (year > 0) for valid cases
			if !tt.expectErr && !result.IsZero() {
				if result.Year() < 2000 || result.Year() > time.Now().Year()+50 {
					t.Errorf("Date seems invalid: %v", result)
				}
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
