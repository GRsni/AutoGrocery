package main

import (
	"autoGrocery/internal/sheets_handler"
	"autoGrocery/pkg/constants"
	"context"
	"fmt"
	"log"
	"time"
)

func main() {
	tokFile := "config/token.json"
	credsFile := "config/credentials.json"
	spreadsheetId := "foo"

	ctx := context.Background()

	srv, err := sheets_handler.GetSheetService(ctx, tokFile, credsFile)
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
