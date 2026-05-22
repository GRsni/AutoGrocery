package sheets_handler

import (
	"autoGrocery/internal/token"
	"context"
	"log"
	"os"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

type CredentialsConfig struct {
	Sheets struct {
		MainID string `json:"main-id"`
	} `json:"sheets"`
	// Add other sections as needed
	Carrefour struct {
		Username string `json:"username"`
		Password string `json:"password"`
	} `json:"carrefour"`
	Dia struct {
		Username string `json:"username"`
		Password string `json:"password"`
	} `json:"dia"`
}

func GetSheetService(ctx context.Context, tokFile string, credsFile string) (*sheets.Service, error) {
	b, err := os.ReadFile(credsFile)
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}
	// If modifying these scopes, delete your previously saved token.json.
	config, err := google.ConfigFromJSON(b, "https://www.googleapis.com/auth/spreadsheets.readonly")
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	client := token.GetClient(config, tokFile)

	return sheets.NewService(ctx, option.WithHTTPClient(client))
}
