package token

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/oauth2"
)

func TestTokenFromFile_Success(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "token_test_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	validToken := &oauth2.Token{
		AccessToken:  "test_access_token_123",
		RefreshToken: "test_refresh_token_456",
		TokenType:    "Bearer",
	}

	if err := json.NewEncoder(tmpFile).Encode(validToken); err != nil {
		t.Fatalf("Failed to encode token: %v", err)
	}
	tmpFile.Close()

	tok, err := tokenFromFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("tokenFromFile() returned error: %v", err)
	}

	if tok.AccessToken != validToken.AccessToken {
		t.Errorf("Expected access token %s, got %s", validToken.AccessToken, tok.AccessToken)
	}
}

func TestTokenFromFile_FileNotFound(t *testing.T) {
	_, err := tokenFromFile("nonexistent_file.json")
	if err == nil {
		t.Error("Expected error for nonexistent file, got nil")
	}
}

func TestSaveToken_Success(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "token_test_*.dir")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	tokenPath := filepath.Join(tmpDir, "test_token.json")
	token := &oauth2.Token{
		AccessToken:  "new_access_token_789",
		RefreshToken: "new_refresh_token_012",
		TokenType:    "Bearer",
	}

	saveToken(tokenPath, token)

	if _, err := os.Stat(tokenPath); os.IsNotExist(err) {
		t.Error("Expected file to be created, but it doesn't exist")
	}

	savedToken, err := tokenFromFile(tokenPath)
	if err != nil {
		t.Fatalf("Failed to read saved token: %v", err)
	}

	if savedToken.AccessToken != token.AccessToken {
		t.Errorf("Expected access token %s, got %s", token.AccessToken, savedToken.AccessToken)
	}
}

func TestGetClient_ReturnsNonNilClient(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "token.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	existingToken := &oauth2.Token{
		AccessToken:  "existing_token",
		RefreshToken: "existing_refresh",
		TokenType:    "Bearer",
	}

	if err := json.NewEncoder(tmpFile).Encode(existingToken); err != nil {
		t.Fatalf("Failed to encode token: %v", err)
	}
	tmpFile.Close()

	config := &oauth2.Config{
		ClientID:    "test_client_id",
		RedirectURL: "http://localhost",
		Scopes:      []string{"sheets_handler.readonly"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://example.com/auth",
			TokenURL: "https://example.com/token",
		},
	}

	client := GetClient(config, tmpFile.Name())
	if client == nil {
		t.Error("Expected non-nil client, got nil")
	}
}

func TestGetClient_WithExistingToken(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "existing_token_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	existingToken := &oauth2.Token{
		AccessToken:  "existing_access_token",
		RefreshToken: "existing_refresh_token",
		TokenType:    "Bearer",
	}

	if err := json.NewEncoder(tmpFile).Encode(existingToken); err != nil {
		t.Fatalf("Failed to encode token: %v", err)
	}
	tmpFile.Close()

	config := &oauth2.Config{
		ClientID:    "test_client_id",
		RedirectURL: "http://localhost",
		Scopes:      []string{"sheets_handler.readonly"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://example.com/auth",
			TokenURL: "https://example.com/token",
		},
	}

	client := GetClient(config, tmpFile.Name())
	if client == nil {
		t.Fatal("Expected non-nil client")
	}

	// Verify the client can make requests (with a mock server or actual call)
	// For now, just verify it's created successfully
	t.Logf("Client created successfully with token embedded in transport")
}

func TestGetClient_NoExistingToken_TriesWeb(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "nonexistent_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	os.Remove(tmpFile.Name()) // Ensure file doesn't exist

	config := &oauth2.Config{
		ClientID:    "test_client_id",
		RedirectURL: "http://localhost",
		Scopes:      []string{"sheets_handler.readonly"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://example.com/auth",
			TokenURL: "https://example.com/token",
		},
	}

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic when calling getTokenFromWeb (no auth code provided)")
		}
	}()

	_ = GetClient(config, tmpFile.Name())
}

func TestTokenFromFile_CloseFile(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "close_test_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	token := &oauth2.Token{
		AccessToken:  "test_token",
		RefreshToken: "test_refresh",
		TokenType:    "Bearer",
	}

	if err := json.NewEncoder(tmpFile).Encode(token); err != nil {
		t.Fatalf("Failed to encode token: %v", err)
	}
	tmpFile.Close()

	tok, err := tokenFromFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("tokenFromFile() returned error: %v", err)
	}
	if tok.AccessToken != "test_token" {
		t.Errorf("Expected access token 'test_token', got %s", tok.AccessToken)
	}
}

func TestSaveToken_FilePermissions(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "permissions_*.dir")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	tokenPath := filepath.Join(tmpDir, "test_token.json")
	token := &oauth2.Token{
		AccessToken:  "test_token",
		RefreshToken: "test_refresh",
		TokenType:    "Bearer",
	}

	saveToken(tokenPath, token)

	info, err := os.Stat(tokenPath)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}

	if info.Mode().Perm()&0444 == 0 {
		t.Error("File should be readable")
	}
}

func TestTokenFromFile_DecodesEmptyFile(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "empty_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(""); err != nil {
		t.Fatalf("Failed to write empty content: %v", err)
	}
	tmpFile.Close()

	_, err = tokenFromFile(tmpFile.Name())
	if err == nil {
		t.Error("Expected error for empty file, got nil")
	}
}

func TestSaveToken_FileAlreadyExists(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "overwrite_*.dir")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	tokenPath := filepath.Join(tmpDir, "test_token.json")
	oldToken := &oauth2.Token{
		AccessToken:  "old_access_token",
		RefreshToken: "old_refresh_token",
		TokenType:    "Bearer",
	}
	file, _ := os.Create(tokenPath)

	if err := json.NewEncoder(file).Encode(oldToken); err != nil {
		t.Fatalf("Failed to create initial token file: %v", err)
	}

	newToken := &oauth2.Token{
		AccessToken:  "new_access_token",
		RefreshToken: "new_refresh_token",
		TokenType:    "Bearer",
	}

	saveToken(tokenPath, newToken)

	savedToken, err := tokenFromFile(tokenPath)
	if err != nil {
		t.Fatalf("Failed to read saved token: %v", err)
	}

	if savedToken.AccessToken != newToken.AccessToken {
		t.Errorf("Expected new access token, got old one")
	}
}

// Helper functions for testing
func newTestConfig() *oauth2.Config {
	return &oauth2.Config{
		ClientID:    "test_client_id",
		RedirectURL: "http://localhost/callback",
		Scopes:      []string{"sheets_handler.readonly"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://example.com/auth",
			TokenURL: "https://example.com/token",
		},
	}
}

func newTestToken() *oauth2.Token {
	return &oauth2.Token{
		AccessToken:  "test_access_token",
		RefreshToken: "test_refresh_token",
		TokenType:    "Bearer",
	}
}
