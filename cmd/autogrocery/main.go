package main

import (
	"autoGrocery/internal/token"
	"autoGrocery/pkg/constants"
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

func main() {
	tokFile := "config/token.json"
	spreadsheetId := "1QG3-vZCIqXut4oz_4r4OkGJeMV91SRZ5od_3rLYj1Ao"

	ctx := context.Background()
	b, err := os.ReadFile("config/credentials.json")
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	// If modifying these scopes, delete your previously saved token.json.
	config, err := google.ConfigFromJSON(b, "https://www.googleapis.com/auth/spreadsheets.readonly")
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	client := token.GetClient(config, tokFile)

	srv, err := sheets.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		log.Fatalf("Unable to retrieve Sheets client: %v", err)
	}

	year, month, day := time.Now().Date()
	fmt.Println("Current date:", year, constants.FromTimeMonth(month), day)
	yearStr := fmt.Sprintf("%02d", year%100)
	readRange := fmt.Sprintf("%s %s!%s", string(constants.FromTimeMonth(month)), yearStr, "A1:F92")
	resp, err := srv.Spreadsheets.Values.Get(spreadsheetId, readRange).Do()
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
