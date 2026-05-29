package dia

import (
	"os"
	"testing"
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile, err := os.CreateTemp("", "cookies_*.txt")
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer os.Remove(tmpFile.Name())

			_, err = tmpFile.WriteString(tt.input + "\n")
			if err != nil {
				t.Fatalf("Failed to write temp file: %v", err)
			}

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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := getDateFromTicket(tt.input)
			if tt.expectErr && err == nil {
				t.Error("Expected error but got nil")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}
