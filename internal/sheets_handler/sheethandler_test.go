package sheets_handler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetSheetsConfig(t *testing.T) {
	tests := []struct {
		name          string
		setupFile     func(t *testing.T) string
		expectedError bool
		expectedID    string
	}{
		{
			name: "valid credentials file",
			setupFile: func(t *testing.T) string {
				tmpDir := t.TempDir()
				credentialsFile := filepath.Join(tmpDir, "credentials.json")
				content := `{
					"sheets": {
						"main-id": "1234"
					}
				}`
				if err := os.WriteFile(credentialsFile, []byte(content), 0644); err != nil {
					t.Fatalf("Failed to write credentials file: %v", err)
				}
				return credentialsFile
			},
			expectedError: false,
			expectedID:    "1234",
		},
		{
			name: "missing credentials file",
			setupFile: func(t *testing.T) string {
				tmpDir := t.TempDir()
				return filepath.Join(tmpDir, "credentials.json")
			},
			expectedError: true,
			expectedID:    "",
		},
		{
			name: "invalid JSON syntax",
			setupFile: func(t *testing.T) string {
				tmpDir := t.TempDir()
				credentialsFile := filepath.Join(tmpDir, "credentials.json")
				content := `{ invalid json }`
				if err := os.WriteFile(credentialsFile, []byte(content), 0644); err != nil {
					t.Fatalf("Failed to write credentials file: %v", err)
				}
				return credentialsFile
			},
			expectedError: true,
			expectedID:    "",
		},
		{
			name: "missing sheets field",
			setupFile: func(t *testing.T) string {
				tmpDir := t.TempDir()
				credentialsFile := filepath.Join(tmpDir, "credentials.json")
				content := `{
					"other": "field"
				}`
				if err := os.WriteFile(credentialsFile, []byte(content), 0644); err != nil {
					t.Fatalf("Failed to write credentials file: %v", err)
				}
				return credentialsFile
			},
			expectedError: true,
			expectedID:    "",
		},
		{
			name: "empty main-id field",
			setupFile: func(t *testing.T) string {
				tmpDir := t.TempDir()
				credentialsFile := filepath.Join(tmpDir, "credentials.json")
				content := `{
					"sheets": {
						"main-id": ""
					}
				}`
				if err := os.WriteFile(credentialsFile, []byte(content), 0644); err != nil {
					t.Fatalf("Failed to write credentials file: %v", err)
				}
				return credentialsFile
			},
			expectedError: true,
			expectedID:    "",
		},
		{
			name: "extra fields in JSON (should be ignored)",
			setupFile: func(t *testing.T) string {
				tmpDir := t.TempDir()
				credentialsFile := filepath.Join(tmpDir, "credentials.json")
				content := `{
					"sheets": {
						"main-id": "1234"
					},
					"other": "field",
					"nested": {
						"value": 123
					}
				}`
				if err := os.WriteFile(credentialsFile, []byte(content), 0644); err != nil {
					t.Fatalf("Failed to write credentials file: %v", err)
				}
				return credentialsFile
			},
			expectedError: false,
			expectedID:    "1234",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			credentialsPath := tt.setupFile(t)

			config, err := GetSheetsConfig(credentialsPath)

			if tt.expectedError {
				if err == nil {
					t.Errorf("GetSheetsConfig() expected error but got none")
				} else {
					if config != nil {
						t.Errorf("GetSheetsConfig() expected nil config but got: %v", config)
					}
					return
				}
			}

			if err != nil {
				t.Errorf("GetSheetsConfig() unexpected error: %v", err)
				return
			}

			if config.Sheets.MainID != tt.expectedID {
				t.Errorf("GetSheetsConfig() MainID = %s, want %s", config.Sheets.MainID, tt.expectedID)
			}
		})
	}
}

// TestGetSheetsConfig_Validation tests that the function validates required fields
func TestGetSheetsConfig_Validation(t *testing.T) {
	t.Run("validates non-empty main-id", func(t *testing.T) {
		tmpDir := t.TempDir()
		credentialsFile := filepath.Join(tmpDir, "credentials.json")
		content := `{
			"sheets": {
				"main-id": ""
			}
		}`
		if err := os.WriteFile(credentialsFile, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write credentials file: %v", err)
		}

		_, err := GetSheetsConfig(credentialsFile)
		if err == nil {
			t.Error("GetSheetsConfig() should return error for empty main-id")
		}
	})
}

func TestGetSheetsConfig_MissingFile(t *testing.T) {
	t.Run("handles missing file", func(t *testing.T) {
		tmpDir := t.TempDir()
		credentialsFile := filepath.Join(tmpDir, "credentials.json")

		_, err := GetSheetsConfig(credentialsFile)
		if err == nil {
			t.Error("GetSheetsConfig() should return error for unreadable file")
		}
	})
}
