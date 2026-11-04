package internal

import (
	"os"
	"testing"
)

func TestLoadExcluded_ValidFile(t *testing.T) {
	content := `{
		"dia": ["id1", "id2"],
		"carrefour": ["id3"],
		"mercadona": []
	}`

	tmpFile, err := os.CreateTemp("", "excluded-test-*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	excluded, err := LoadExcluded(tmpFile.Name())
	if err != nil {
		t.Fatalf("LoadExcluded failed with valid input: %v", err)
	}

	if len(excluded.DiaIDs) != 2 {
		t.Errorf("Expected 2 dia IDs, got %d", len(excluded.DiaIDs))
	}
	if excluded.DiaIDs[0] != "id1" {
		t.Errorf("Expected first dia ID 'id1', got '%s'", excluded.DiaIDs[0])
	}
	if len(excluded.CarrefourIds) != 1 {
		t.Errorf("Expected 1 carrefour ID, got %d", len(excluded.CarrefourIds))
	}
	if excluded.CarrefourIds[0] != "id3" {
		t.Errorf("Expected carrefour ID 'id3', got '%s'", excluded.CarrefourIds[0])
	}
	if len(excluded.MercadonaIds) != 0 {
		t.Errorf("Expected 0 mercadona IDs, got %d", len(excluded.MercadonaIds))
	}
}

func TestLoadExcluded_MissingFile(t *testing.T) {
	_, err := LoadExcluded("/non/existent/path/excluded.json")
	if err == nil {
		t.Error("Expected error for missing file, but got nil")
	}
}

func TestLoadExcluded_InvalidJSON(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "excluded-test-*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	tmpFile.WriteString("{ invalid json }")
	tmpFile.Close()

	_, err = LoadExcluded(tmpFile.Name())
	if err == nil {
		t.Error("Expected error for invalid JSON, but got nil")
	}
}

func TestLoadExcluded_EmptyFile(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "excluded-test-*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	tmpFile.WriteString("{}")
	tmpFile.Close()

	excluded, err := LoadExcluded(tmpFile.Name())
	if err != nil {
		t.Fatalf("LoadExcluded failed with empty JSON: %v", err)
	}

	if len(excluded.DiaIDs) != 0 {
		t.Errorf("Expected 0 dia IDs for empty JSON, got %d", len(excluded.DiaIDs))
	}
}
