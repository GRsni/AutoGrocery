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

func main() {
	setupLogger()

	ctx := context.Background()

	currentYear, currentMonth, currentDay := time.Now().Date()
	fmt.Println("Current date:", currentYear, constants.FromTimeMonth(currentMonth), currentDay)
	yearStr := fmt.Sprintf("%02d", currentYear%100)
	sheetPageName := fmt.Sprintf("%s %s", string(constants.FromTimeMonth(currentMonth)), yearStr)
	//sheetPageName := "testpage"
	readRange := fmt.Sprintf("%s!%s", sheetPageName, "A2:B200")

	oAuthConfig := token.GetOauthConfig(CredsFilePath, sheets.SpreadsheetsScope, gmail.MailGoogleComScope)

	gmailManager := gm.GetGmailManager(ctx, oAuthConfig, TokenFilePath)

	sheetsManager := sh.GetSheetManager(ctx, oAuthConfig, TokenFilePath, CredsFilePath, sheetPageName)

	slog.Debug("Fetching date column data from " + readRange)

	ticketsFromSheet, _ := sh.GetSheetTicketList(sheetsManager, "A2:B300")

	slog.Info(fmt.Sprintf("Fetched %d tickets for %v %v %v", len(ticketsFromSheet), currentYear, constants.FromTimeMonth(currentMonth), currentDay))

	lastWrittenRow := sh.GetLastWrittenRowIndex(sheetsManager)

	newTicketsMap := getAllTicketsFromStores(gmailManager, ticketsFromSheet)

	newTicketsList := CombineTickets(newTicketsMap)
	slog.Info("New tickets found", "TICKETS", newTicketsList)

	uploadNewTickets(newTicketsList, sheetsManager, lastWrittenRow)

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

func setupLogger() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)
}

func getAllTicketsFromStores(gmManager gm.Manager, sheetEntries []sh.Entry) map[string][]internal.Ticket {
	allTickets := make(map[string][]internal.Ticket, 3)

	for _, store := range []string{constants.MERCADONA, constants.DIA, constants.CARREFOUR} {
		lastEntryFromSheets := sh.GetLastEntryForStore(sheetEntries, store)
		if len(lastEntryFromSheets.Store) == 0 {
			slog.Info("No last ticket found for store ", "STORE", store)
		} else {
			slog.Info("Last ticket found for store ", "STORE", store, "ENTRY", sh.EntryToStr(lastEntryFromSheets))
		}

		switch store {
		case constants.MERCADONA:
			allTickets[constants.MERCADONA] = getMercadonaTickets(gmManager, lastEntryFromSheets)
			break
		case constants.DIA:
			allTickets[constants.DIA] = getDiaTickets(lastEntryFromSheets)
			break
		case constants.CARREFOUR:
			allTickets[constants.CARREFOUR] = getCarrefourTickets(gmManager, lastEntryFromSheets)
		}
	}
	return allTickets
}

func getMercadonaTickets(manager gm.Manager, lastEntry sh.Entry) []internal.Ticket {
	tickets := mercadona.GetTicketList(manager, lastEntry.Date)
	slog.Info("Found new tickets\n", "TICKETS", len(tickets), "STORE", constants.MERCADONA)
	return tickets
}

func getDiaTickets(lastEntry sh.Entry) []internal.Ticket {
	page, cleanup, err := dia.LoginToDia(CredsFilePath, CookiesPath)
	if err != nil {
		cleanup()
		return []internal.Ticket{}
	}
	newDiaTickets := dia.GetTicketList(page, lastEntry.Date)
	slog.Info("Found new tickets", "TICKETS", len(newDiaTickets), "STORE", constants.DIA)
	cleanup()
	page.Close()
	return newDiaTickets
}

func getCarrefourTickets(manager gm.Manager, lastEntry sh.Entry) []internal.Ticket {
	page, cleanup, err := carrefour.LoginToCarrefour(manager, CredsFilePath)
	if err != nil {
		cleanup()
		return []internal.Ticket{}
	}
	newTickets := carrefour.GetTicketList(page, lastEntry.Date)
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
