package main

import (
	"autoGrocery/internal"
	"autoGrocery/internal/carrefour"
	"autoGrocery/internal/dia"
	"autoGrocery/internal/google/gm"
	"autoGrocery/internal/google/sh"
	"autoGrocery/internal/mercadona"
	"autoGrocery/internal/token"
	"autoGrocery/pkg/constants"
	"context"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"time"

	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/sheets/v4"
)

const CredsFilePath = "config/credentials/secrets.json"
const TokenFilePath = "config/credentials/token.json"
const CookiesPath = "config/credentials/cookies-www-dia-es.txt"


func setupLogger() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)
}

func main() {
	setupLogger()

	ctx := context.Background()

	currentYear, currentMonth, currentDay := time.Now().Date()
	fmt.Println("Current date:", currentYear, constants.FromTimeMonth(currentMonth), currentDay)
	sheetPageName := getSheetName(currentYear, currentMonth)
	readRange := fmt.Sprintf("%s!%s", sheetPageName, "A2:B200")

	firstOfMonth := time.Date(currentYear, currentMonth, 1, 0, 0, 0, 0, time.Now().Location())

	oAuthConfig := token.GetOauthConfig(CredsFilePath, sheets.SpreadsheetsScope, gmail.MailGoogleComScope)

	gmailManager := gm.GetGmailManager(ctx, oAuthConfig, TokenFilePath)

	sheetsManager := sh.GetSheetManager(ctx, oAuthConfig, TokenFilePath, CredsFilePath, sheetPageName)

	slog.Debug("Fetching date column data from " + readRange)

	ticketsFromSheet, _ := sh.GetSheetTicketList(sheetsManager, "A2:B300")

	slog.Info(fmt.Sprintf("Fetched %d tickets for %v %v %v", len(ticketsFromSheet), currentYear, constants.FromTimeMonth(currentMonth), currentDay))

	lastWrittenRow := sh.GetLastWrittenRowIndex(sheetsManager)

	newTicketsMap := getAllTicketsFromStores(gmailManager, ticketsFromSheet, firstOfMonth)

	newTicketsList := CombineTickets(newTicketsMap)
	slog.Info("New tickets found", "TICKETS", newTicketsList)

	uploadNewTickets(newTicketsList, sheetsManager, lastWrittenRow)

}

func getSheetName(currentYear int, currentMonth time.Month) string {
	yearStr := fmt.Sprintf("%02d", currentYear%100)
	sheetPageName := fmt.Sprintf("%s %s", string(constants.FromTimeMonth(currentMonth)), yearStr)
	return sheetPageName
}

func uploadNewTickets(newTicketsList []internal.Ticket, sheetsManager sh.Manager, lastWrittenRow int) {
	for _, ticketToUpload := range newTicketsList {
		valueRange, err := ticketToUpload.ToValueRange()
		if err != nil {
			slog.Info("Unable to create value range object, skipping", "ERROR", err)
			return
		}
		updatedLines := sh.WriteToSheet(sheetsManager, valueRange, lastWrittenRow+1)
		lastWrittenRow += updatedLines
	}
}

func getAllTicketsFromStores(gmManager gm.Manager, sheetEntries []sh.Entry, firstOfMonth time.Time) map[string][]internal.Ticket {
	allTickets := make(map[string][]internal.Ticket)

	for _, store := range []string{constants.MERCADONA, constants.DIA, constants.CARREFOUR} {
		lastEntryFromSheets := sh.GetLastEntryForStore(sheetEntries, store)
		lastDate := getDateForLastTicket(lastEntryFromSheets, store, firstOfMonth)

		switch store {
		case constants.MERCADONA:
			allTickets[constants.MERCADONA] = getMercadonaTickets(gmManager, lastDate)
			break
		case constants.DIA:
			allTickets[constants.DIA] = getDiaTickets(lastDate)
			break
		case constants.CARREFOUR:
			allTickets[constants.CARREFOUR] = getCarrefourTickets(gmManager, lastDate)
		}
	}
	return allTickets
}

func getDateForLastTicket(lastEntryFromSheets *sh.Entry, store string, firstOfMonth time.Time) time.Time {
	var lastDate time.Time
	if lastEntryFromSheets == nil {
		slog.Info("No last ticket found for store, using first of month date", "STORE", store, "DATE", firstOfMonth)
		lastDate = firstOfMonth
	} else {
		slog.Info("Last ticket found for store, using entry date", "STORE", store, "DATE", lastEntryFromSheets.Date)
		lastDate = lastEntryFromSheets.Date
	}
	return lastDate
}

func getMercadonaTickets(manager gm.Manager, lastDate time.Time) []internal.Ticket {
	tickets := mercadona.GetTicketList(manager, lastDate)
	slog.Info("Found new tickets", "TICKETS", len(tickets), "STORE", constants.MERCADONA)
	return tickets
}

func getDiaTickets(lastDate time.Time) []internal.Ticket {
	page, cleanup, err := dia.LoginToDia(CredsFilePath, CookiesPath)
	if err != nil {
		cleanup()
		return []internal.Ticket{}
	}
	newDiaTickets := dia.GetTicketList(page, lastDate)
	slog.Info("Found new tickets", "TICKETS", len(newDiaTickets), "STORE", constants.DIA)
	cleanup()
	page.Close()
	return newDiaTickets
}

func getCarrefourTickets(manager gm.Manager, lastDate time.Time) []internal.Ticket {
	page, cleanup, err := carrefour.LoginToCarrefour(manager, CredsFilePath)
	if err != nil {
		cleanup()
		return []internal.Ticket{}
	}
	newTickets := carrefour.GetTicketList(page, lastDate)
	slog.Info("Found new tickets", "TICKETS", len(newTickets), "STORE", constants.CARREFOUR)
	cleanup()
	page.Close()
	return newTickets
}

func CombineTickets(allTickets map[string][]internal.Ticket) []internal.Ticket {
	ticketList := allTickets[constants.MERCADONA]
	ticketList = append(ticketList, allTickets[constants.DIA]...)
	ticketList = append(ticketList, allTickets[constants.CARREFOUR]...)

	sort.SliceStable(ticketList, func(i, j int) bool {
		return ticketList[i].Date.Before(ticketList[j].Date)
	})

	return ticketList
}
