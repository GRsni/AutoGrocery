package main

import (
	"autoGrocery/internal/sheets_handler"
	"autoGrocery/pkg/constants"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"
)

type CredentialsConfig struct {
	Sheets struct {
		MainID string `json:"main-id"`
	} `json:"sheets"`
}

func main() {
	tokFile := "config/token.json"
	credsFile := "config/credentials.json"

	ctx := context.Background()

	srv, err := sheets_handler.GetSheetService(ctx, tokFile, credsFile)
	if err != nil {
		log.Fatalf("Unable to retrieve Sheets client: %v", err)
	}

	b, err := os.ReadFile(credsFile)
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}
	var config CredentialsConfig
	if err := json.Unmarshal(b, &config); err != nil {
		fmt.Println("Failed to unmarshal credentials config:", err)
		return
	}

	year, month, day := time.Now().Date()
	fmt.Println("Current date:", year, constants.FromTimeMonth(month), day)
	yearStr := fmt.Sprintf("%02d", year%100)
	readRange := fmt.Sprintf("%s %s!%s", string(constants.FromTimeMonth(month)), yearStr, "A1:F92")
	resp, err := srv.Spreadsheets.Values.Get(config.Sheets.MainID, readRange).Do()
	if err != nil {
		log.Fatalf("Unable to retrieve data from sheet: %v", err)
	}

	if len(resp.Values) == 0 {
		fmt.Println("No data found.")
	} else {
		for i, row := range resp.Values {
			for j, cell := range row {
				fmt.Printf("[%d][%d]: %v\n", i, j, cell)
			}
			fmt.Println()
		}
	}

}
