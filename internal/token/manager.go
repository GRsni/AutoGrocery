package token

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// GetClient retrieves a token, refreshing and saving it if needed, then returns an HTTP client.
func GetClient(config *oauth2.Config, tokFile string) (*http.Client, error) {
	tok, err := tokenFromFile(tokFile)

	// --- PHASE 1: No Token Found (or file corrupted) ---
	if err != nil {
		slog.Info("No valid token found in file. Initiating web flow.")
		newTok, err := getTokenFromWeb(config)
		if err != nil {
			return nil, fmt.Errorf("failed during initial web flow: %w", err)
		}
		saveToken(tokFile, newTok)
		tok = newTok
	}

	// --- PHASE 2: Token Found, Check Expiry ---
	if !tok.Expiry.IsZero() && time.Now().After(tok.Expiry) {
		slog.Info("Token expired, attempting refresh using refresh_token")

		// Attempt to refresh the token
		newTok, err := config.TokenSource(context.Background(), tok).Token()

		if err != nil {
			//Refresh failed (e.g., invalid_grant).
			slog.Error("Failed to refresh token. Token may be revoked or expired.", "error", err)

			// Clean up the dead token file so we don't keep trying it
			os.Remove(tokFile)

			// Since the refresh failed, we MUST abandon this token and force a fresh web flow
			slog.Info("Forcing full re-authentication via web flow.")

			newTok, err = getTokenFromWeb(config)
			if err != nil {
				return nil, fmt.Errorf("failed during re-authentication web flow: %w", err)
			}
			tok = newTok
			saveToken(tokFile, tok)
		} else {
			// Refresh succeeded
			tok = newTok
			saveToken(tokFile, tok)
		}
	}

	return config.Client(context.Background(), tok), nil
}

// getTokenFromWeb requests a token from the web via the OAuth consent flow.
func getTokenFromWeb(config *oauth2.Config) (*oauth2.Token, error) {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)

	// 1. Automate Browser Opening
	log.Printf("Opening authentication link in your default browser...")
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", authURL)
	} else if runtime.GOOS == "darwin" { // Mac
		cmd = exec.Command("open", authURL)
	} else { // Linux (assuming xdg-open)
		cmd = exec.Command("xdg-open", authURL)
	}

	if cmd != nil {
		cmd.Start()
	}

	// 2. Prompt User for Code
	fmt.Println("\n=========================================================")
	fmt.Printf("ACTION REQUIRED: Please visit the link above, grant permissions, and copy the authorization code.\n")
	fmt.Print("Paste the full authorization code below: ")

	reader := bufio.NewReader(os.Stdin)
	fullAuthString, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read authorization code: %w", err)
	}

	authCode := strings.Replace(fullAuthString, "http://localhost/?state=state-token&iss=https://accounts.google.com&code=", "", 1)
	authCode = strings.Replace(authCode, "&scope=https://mail.google.com/%20https://www.googleapis.com/auth/spreadsheets", "", 1)

	// 3. Exchange Code for Token
	log.Println("Exchanging code for access token...")
	tok, err := config.Exchange(context.Background(), authCode)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code for token: %w", err)
	}
	return tok, nil
}

// tokenFromFile retrieves a token from a local file.
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	tok := &oauth2.Token{}
	if err := json.NewDecoder(f).Decode(tok); err != nil {
		return tok, err
	}

	if tok.AccessToken == "" {
		return tok, fmt.Errorf("missing or empty AccessToken in token file")
	}

	return tok, nil
}

// saveToken saves a token to a file path.
func saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()

	if err := json.NewEncoder(f).Encode(token); err != nil {
		slog.Info("Failed to encode token", "ERROR", err)
	}
}

func GetOauthConfig(credsFile string, scopes ...string) *oauth2.Config {
	b, err := os.ReadFile(credsFile)
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}
	config, err := google.ConfigFromJSON(b, scopes...)
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	return config
}
