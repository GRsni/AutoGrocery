package main

import (
	"autoGrocery/internal/dia"
	"autoGrocery/internal/google/gm"
	"autoGrocery/internal/google/sh"
	"autoGrocery/internal/mercadona"
	"autoGrocery/internal/token"
	"autoGrocery/pkg/constants"
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/sheets/v4"
)

const CredsFilePath = "config/credentials/secrets.json"
const TokenFilePath = "config/credentials/token.json"
const CookiesPath = "config/credentials/cookies-www-dia-es.txt"

func main() {
	setupLogger()

	ctx := context.Background()

	currentYear, currentMonth, currentDay := time.Now().Date()
	fmt.Println("Current date:", currentYear, constants.FromTimeMonth(currentMonth), currentDay)
	//yearStr := fmt.Sprintf("%02d", currentYear%100)
	//sheetPageName := fmt.Sprintf("%s %s", string(constants.FromTimeMonth(currentMonth)), yearStr)
	sheetPageName := "testpage"
	readRange := fmt.Sprintf("%s!%s", sheetPageName, "A2:B200")

	oAuthConfig := token.GetOauthConfig(CredsFilePath, sheets.SpreadsheetsScope, gmail.GmailReadonlyScope)

	gmailManager := gm.GetGmailManager(ctx, oAuthConfig, TokenFilePath)


	sheetsManager := sh.GetSheetManager(ctx, oAuthConfig, TokenFilePath, CredsFilePath, sheetPageName)

	slog.Debug("Fetching date column data from " + readRange)

	ticketsFromSheet, _ := sh.GetSheetTicketList(sheetsManager, "A2:B200")

	fmt.Println("Fetched", len(ticketsFromSheet), "tickets for", currentYear, constants.FromTimeMonth(currentMonth), currentDay)

	lastWrittenRow := sh.GetLastWrittenRowIndex(sheetsManager)
	fmt.Println("last row: ", lastWrittenRow)

	lastMercadonaTicket, err := sh.GetLastTicketForShop(ticketsFromSheet, constants.MERCADONA)
	if err != nil {
		log.Fatalf("Unable to retrieve last ticket for Mercadona: %v", err)
	}

	mercadonaTickets := mercadona.GetTicketList(gmailManager, lastMercadonaTicket.Date)

	for _, mercadonaTicket := range mercadonaTickets {
		valueRange := mercadonaTicket.ToValueRange(constants.MERCADONA)

		updatedLines := sh.WriteToSheet(sheetsManager, valueRange, lastWrittenRow+1)
		lastWrittenRow += updatedLines
	}

	return

	lastDiaTicket, err := sh.GetLastTicketForShop(ticketsFromSheet, constants.DIA)
	if err != nil {
		log.Fatalf("Unable to retrieve last ticket for Dia: %v", err)
	}
	fmt.Printf("Last ticket found for DIA %v\n", sh.GroceryTicketToString(lastDiaTicket))

	diaPage, cleanup, err := dia.LoginToDia(CredsFilePath, CookiesPath)
	if err != nil {
		cleanup()
		return
	}
	newDiaTickets := dia.GetTicketList(diaPage, lastDiaTicket.Date)
	fmt.Printf("Found %d new tickets for DIA\n", len(newDiaTickets))
	cleanup()
	diaPage.Close()

	for _, diaTicket := range newDiaTickets {
		valueRange := diaTicket.ToValueRange(constants.DIA)

		updatedLines := sh.WriteToSheet(sheetsManager, valueRange, lastWrittenRow+1)
		lastWrittenRow += updatedLines
	}
}

func setupLogger() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)
}
