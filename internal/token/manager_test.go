package token

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/oauth2"
)

// TestTokenFromFile_Success tests a successful token loading from a valid file
func TestTokenFromFile_Success(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "token_test_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	validToken := &oauth2.Token{
		AccessToken:  "test_access_token_123",
		RefreshToken: "test_refresh_token_456",
		TokenType:    "Bearer",
	}

	if err := json.NewEncoder(tmpFile).Encode(validToken); err != nil {
		tmpFile.Close()
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

	os.Remove(tmpFile.Name())
}

// TestTokenFromFile_FileNotFound tests error handling when the file doesn't exist
func TestTokenFromFile_FileNotFound(t *testing.T) {
	_, err := tokenFromFile("nonexistent_file.json")
	if err == nil {
		t.Error("Expected error for nonexistent file, got nil")
	}
}

// TestTokenFromFile_EmptyFile tests handling of empty files
func TestTokenFromFile_EmptyFile(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "empty_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	if _, err := tmpFile.WriteString(""); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		t.Fatalf("Failed to write empty content: %v", err)
	}
	tmpFile.Close()

	_, err = tokenFromFile(tmpFile.Name())
	if err == nil {
		t.Error("Expected error for empty file, got nil")
	}

	os.Remove(tmpFile.Name())
}

// TestTokenFromFile_MalformedJSON tests handling of malformed JSON
func TestTokenFromFile_MalformedJSON(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "malformed_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	malformedContent := `{"AccessToken": "test_token"` // Missing closing brace

	if _, err := tmpFile.WriteString(malformedContent); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		t.Fatalf("Failed to write malformed content: %v", err)
	}
	tmpFile.Close()

	_, err = tokenFromFile(tmpFile.Name())
	if err == nil {
		t.Error("Expected error for malformed JSON, got nil")
	}

	os.Remove(tmpFile.Name())
}

// TestTokenFromFile_MissingAccessToken tests handling when AccessToken is missing
func TestTokenFromFile_MissingAccessToken(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "missing_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	tokenData := map[string]string{
		"RefreshToken": "test_refresh",
	}

	if err := json.NewEncoder(tmpFile).Encode(tokenData); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		t.Fatalf("Failed to encode token: %v", err)
	}
	tmpFile.Close()

	tok, err := tokenFromFile(tmpFile.Name())
	if err == nil {
		t.Errorf("tokenFromFile() expected error when AccessToken is missing, got nil")
		return
	}
	defer os.Remove(tmpFile.Name())

	if tok != nil && tok.AccessToken != "" {
		t.Errorf("Expected AccessToken to be empty, got: %q", tok.AccessToken)
	}
}

// TestSaveToken_Success tests successful token saving
func TestSaveToken_Success(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "token_test_*.dir")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

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

	os.RemoveAll(tmpDir)
}

// TestSaveToken_FileAlreadyExists tests that existing files are overwritten
func TestSaveToken_FileAlreadyExists(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "overwrite_*.dir")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	tokenPath := filepath.Join(tmpDir, "test_token.json")
	oldToken := &oauth2.Token{
		AccessToken:  "old_access_token",
		RefreshToken: "old_refresh_token",
		TokenType:    "Bearer",
	}
	file, err := os.Create(tokenPath)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("Failed to create initial token file: %v", err)
	}

	if err := json.NewEncoder(file).Encode(oldToken); err != nil {
		file.Close()
		os.Remove(tokenPath)
		os.RemoveAll(tmpDir)
		t.Fatalf("Failed to create initial token file: %v", err)
	}
	file.Close()

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

	os.RemoveAll(tmpDir)
}

// TestSaveToken_FilePermissions tests that saved files are readable
func TestSaveToken_FilePermissions(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "permissions_*.dir")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

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

	os.RemoveAll(tmpDir)
}

// TestSaveToken_EmptyToken tests saving a token with empty values
func TestSaveToken_EmptyToken(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "empty_token_*.dir")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	tokenPath := filepath.Join(tmpDir, "test_token.json")
	token := &oauth2.Token{
		AccessToken:  "",
		RefreshToken: "",
		TokenType:    "",
	}

	saveToken(tokenPath, token)

	if _, err := os.Stat(tokenPath); os.IsNotExist(err) {
		t.Error("Expected file to be created, but it doesn't exist")
	}

	_, err = tokenFromFile(tokenPath)
	if err == nil {
		t.Fatalf("Error should be empty here: %v", err)
	}

	os.RemoveAll(tmpDir)
}

// TestGetClient_ReturnsNonNilClient tests that GetClient returns a non-nil client
func TestGetClient_ReturnsNonNilClient(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "token.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	existingToken := &oauth2.Token{
		AccessToken:  "existing_token",
		RefreshToken: "existing_refresh",
		TokenType:    "Bearer",
	}

	if err := json.NewEncoder(tmpFile).Encode(existingToken); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		t.Fatalf("Failed to encode existing token: %v", err)
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

	os.Remove(tmpFile.Name())
}

// TestGetClient_WithExistingToken tests client creation with existing token file
func TestGetClient_WithExistingToken(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "existing_token_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	existingToken := &oauth2.Token{
		AccessToken:  "existing_access_token",
		RefreshToken: "existing_refresh_token",
		TokenType:    "Bearer",
	}

	if err := json.NewEncoder(tmpFile).Encode(existingToken); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		t.Fatalf("Failed to encode existing token: %v", err)
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
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		t.Fatal("Expected non-nil client")
	}

	t.Logf("Client created successfully with token embedded in transport")

	os.Remove(tmpFile.Name())
}

// TestGetClient_NoExistingToken_TriesWeb tests behavior when no token file exists
func TestGetClient_NoExistingToken_TriesWeb(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "nonexistent_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	tmpFile.Close()
	os.Remove(tmpFile.Name())

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

// TestGetClient_InvalidConfig tests client creation with invalid config
func TestGetClient_InvalidConfig(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "token.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	token := &oauth2.Token{
		AccessToken:  "test_token",
		RefreshToken: "test_refresh",
		TokenType:    "Bearer",
	}

	if err := json.NewEncoder(tmpFile).Encode(token); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		t.Fatalf("Failed to encode token: %v", err)
	}
	tmpFile.Close()

	config := &oauth2.Config{
		ClientID:    "", // Empty client ID is invalid
		RedirectURL: "http://localhost",
	}

	client := GetClient(config, tmpFile.Name())
	if client != nil {
		t.Log("Client was created with empty ClientID - may need validation")
	}

	os.Remove(tmpFile.Name())
}

// TestTokenFromFile_DecodesEmptyFile tests that empty files return an error
func TestTokenFromFile_DecodesEmptyFile(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "empty_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	if _, err := tmpFile.WriteString(""); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		t.Fatalf("Failed to write empty content: %v", err)
	}
	tmpFile.Close()

	_, err = tokenFromFile(tmpFile.Name())
	if err == nil {
		t.Error("Expected error for empty file, got nil")
	}

	os.Remove(tmpFile.Name())
}

// TestTokenFromFile_DecodesMalformedJSON tests malformed JSON handling
func TestTokenFromFile_DecodesMalformedJSON(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "malformed_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	malformedContent := `{"AccessToken": "test_token"` // Missing closing brace

	if _, err := tmpFile.WriteString(malformedContent); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		t.Fatalf("Failed to write malformed content: %v", err)
	}
	tmpFile.Close()

	_, err = tokenFromFile(tmpFile.Name())
	if err == nil {
		t.Error("Expected error for malformed JSON, got nil")
	}

	os.Remove(tmpFile.Name())
}

// TestTokenFromFile_InvalidTokenType tests handling of invalid token type
func TestTokenFromFile_InvalidTokenType(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "invalid_type_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	tokenData := map[string]string{
		"AccessToken":  "test_token",
		"RefreshToken": "test_refresh",
		"TokenType":    "InvalidType",
	}

	if err := json.NewEncoder(tmpFile).Encode(tokenData); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		t.Fatalf("Failed to encode token: %v", err)
	}
	tmpFile.Close()

	_, err = tokenFromFile(tmpFile.Name())
	if err == nil {
		t.Errorf("tokenFromFile() expected error for invalid TokenType, got nil")
		return
	}
	defer os.Remove(tmpFile.Name())
}

// TestTokenFromFile_MultipleFields tests token with multiple fields
func TestTokenFromFile_MultipleFields(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "multi_field_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	tokenData := map[string]any{
		"access_token":  "test_access_token_123",
		"refresh_token": "test_refresh_token_456",
		"token_type":    "Bearer",
		"expiry":        "2026-12-31T23:59:59Z",
	}

	if err := json.NewEncoder(tmpFile).Encode(tokenData); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		t.Fatalf("Failed to encode token: %v", err)
	}
	tmpFile.Close()

	tok, err := tokenFromFile(tmpFile.Name())
	if err != nil {
		t.Errorf("tokenFromFile() returned error: %v", err)
		return
	}

	if tok.AccessToken != "test_access_token_123" {
		t.Errorf("Expected access token 'test_access_token_123', got '%s'", tok.AccessToken)
	}

	os.Remove(tmpFile.Name())
}

// TestSaveToken_OverwriteLargeFile tests saving a new token that overwrites an existing large file
func TestSaveToken_OverwriteLargeFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "overwrite_large_*.dir")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	tokenPath := filepath.Join(tmpDir, "test_token.json")
	largeToken := &oauth2.Token{
		AccessToken:  "old_access_token_" + "very_long_string_to_make_it_larger",
		RefreshToken: "old_refresh_token_" + "very_long_string_to_make_it_larger",
		TokenType:    "Bearer",
	}

	file, err := os.Create(tokenPath)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("Failed to create initial token file: %v", err)
	}

	if err := json.NewEncoder(file).Encode(largeToken); err != nil {
		file.Close()
		os.Remove(tokenPath)
		os.RemoveAll(tmpDir)
		t.Fatalf("Failed to encode large token: %v", err)
	}
	file.Close()

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

	os.RemoveAll(tmpDir)
}

// TestTokenFromFile_ReadFromDifferentPaths tests reading tokens from different file paths
func TestTokenFromFile_ReadFromDifferentPaths(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "token_test_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	token := &oauth2.Token{
		AccessToken:  "test_access_token",
		RefreshToken: "test_refresh_token",
		TokenType:    "Bearer",
	}

	if err := json.NewEncoder(tmpFile).Encode(token); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		t.Fatalf("Failed to encode token: %v", err)
	}
	tmpFile.Close()

	testPaths := []string{
		tmpFile.Name(),
		filepath.Dir(tmpFile.Name()) + "/" + filepath.Base(tmpFile.Name()),
	}

	for _, path := range testPaths {
		tok, err := tokenFromFile(path)
		if err != nil {
			t.Errorf("Failed to read token from path %s: %v", path, err)
		} else if tok.AccessToken != "test_access_token" {
			t.Errorf("Expected access token 'test_access_token', got '%s'", tok.AccessToken)
		}
	}

	os.Remove(tmpFile.Name())
}

// TestSaveToken_SpecialCharactersInToken tests saving tokens with special characters
func TestSaveToken_SpecialCharactersInToken(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "special_chars_*.dir")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	tokenPath := filepath.Join(tmpDir, "test_token.json")
	token := &oauth2.Token{
		AccessToken:  "token_with_special_chars!@#$%^&*()_+-=[]{}|;':\",./<>?",
		RefreshToken: "refresh_with_special_chars!@#$%^&*()_+-=[]{}|;':\",./<>?",
		TokenType:    "Bearer",
	}

	saveToken(tokenPath, token)

	savedToken, err := tokenFromFile(tokenPath)
	if err != nil {
		t.Fatalf("Failed to read saved token: %v", err)
	}

	if savedToken.AccessToken != token.AccessToken {
		t.Errorf("Expected access token with special chars, got '%s'", savedToken.AccessToken)
	}

	os.RemoveAll(tmpDir)
}

// TestGetClient_RefreshTokenPresent tests client creation when refresh token is present
func TestGetClient_RefreshTokenPresent(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "refresh_token.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	token := &oauth2.Token{
		AccessToken:  "access_token_123",
		RefreshToken: "refresh_token_456",
		TokenType:    "Bearer",
	}

	if err := json.NewEncoder(tmpFile).Encode(token); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		t.Fatalf("Failed to encode token: %v", err)
	}
	tmpFile.Close()

	config := &oauth2.Config{
		ClientID:    "test_client_id",
		RedirectURL: "http://localhost/callback",
		Scopes:      []string{"sheets_handler.readonly"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://example.com/auth",
			TokenURL: "https://example.com/token",
		},
	}

	client := GetClient(config, tmpFile.Name())
	if client == nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		t.Fatal("Expected non-nil client")
	}

	t.Log("Client created successfully with refresh token available")

	os.Remove(tmpFile.Name())
}
