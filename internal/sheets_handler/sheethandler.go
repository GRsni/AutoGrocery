package sheets_handler

import (
	"autoGrocery/internal/token"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

type SheetsConfig struct {
	Sheets struct {
		MainID string `json:"main-id"`
	} `json:"sheets"`
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

func GetSheetsConfig(credentialsPath string) (*SheetsConfig, error) {
	b, err := os.ReadFile(credentialsPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read client secret file: %w", err)
	}
	var sheetsConfig SheetsConfig
	if err := json.Unmarshal(b, &sheetsConfig); err != nil {
		fmt.Println("Failed to unmarshal credentials config:", err)
		return nil, err
	}
	if sheetsConfig.Sheets.MainID == "" {
		return nil, fmt.Errorf("main sheet ID is empty")
	}
	return &sheetsConfig, nil
}
