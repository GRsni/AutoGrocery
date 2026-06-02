package main

import (
	"autoGrocery/internal/dia"
	"autoGrocery/internal/sheets_handler"
	"autoGrocery/pkg/constants"
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)

	credentialsPath := "config/credentials/secrets.json"
	ctx := context.Background()

	tokFile := "config/credentials/token.json"

	currentYear, currentMonth, currentDay := time.Now().Date()
	fmt.Println("Current date:", currentYear, constants.FromTimeMonth(currentMonth), currentDay)
	//yearStr := fmt.Sprintf("%02d", currentYear%100)
	//sheetPageName := fmt.Sprintf("%s %s", string(constants.FromTimeMonth(currentMonth)), yearStr)
	sheetPageName := "testpage"
	readRange := fmt.Sprintf("%s!%s", sheetPageName, "A2:B200")

	sheetsManager := sheets_handler.GetSheetManager(ctx, tokFile, credentialsPath, sheetPageName)

	slog.Debug("Fetching date column data from " + readRange)

	ticketsFromSheet, _ := sheets_handler.GetSheetTicketList(sheetsManager, "A2:B200")

	fmt.Println("Fetched", len(ticketsFromSheet), "tickets for", currentYear, constants.FromTimeMonth(currentMonth), currentDay)

	lastWrittenRow := getLastWrittenLine(sheetsManager)
	fmt.Println("last row: ", lastWrittenRow)

	diaTicket, err := sheets_handler.GetLastTicketForShop(ticketsFromSheet, constants.DIA)
	if err != nil {
		log.Fatalf("Unable to retrieve last ticket for Dia: %v", err)
	}
	fmt.Printf("Last ticket found for DIA %v\n", sheets_handler.GroceryTicketToString(diaTicket))

	cookiesPath := "config/credentials/cookies-www-dia-es.txt"
	diaPage, cleanup, err := dia.LoginToDia(credentialsPath, cookiesPath)
	if err != nil {
		cleanup()
		return
	}
	newDiaTickets := dia.GetTicketList(diaPage, diaTicket.Date)
	fmt.Printf("Found %d new tickets for DIA\n", len(newDiaTickets))
	cleanup()
	diaPage.Close()

	for _, diaTicket := range newDiaTickets {
		valueRange := dia.ToValueRange(diaTicket)
		fmt.Println(valueRange)

		updatedLines := sheets_handler.WriteToSheet(sheetsManager, valueRange, lastWrittenRow+1)
		lastWrittenRow += updatedLines
	}
}

func getLastWrittenLine(manager sheets_handler.SheetsManager) int {
	readRange := fmt.Sprintf("%s!E2:300", manager.PageName)
	data := sheets_handler.ReadFromSheet(manager, readRange)
	var lastRow = 0
	for i, row := range data.Values {
		if len(row) > 1 && row[0].(string) == "TOTAL" && len(row[0].(string)) > 0 {
			lastRow = i + 2
		}
	}

	return lastRow
}
