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
	"flag"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"sync"
	"time"

	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/sheets/v4"
)

// Version and BuildDate are set via ldflags at build time
var (
	Version   = "dev"
	BuildDate = "unknown"
)

const CredsFilePath = "config/credentials/secrets.json"
const TokenFilePath = "config/credentials/token.json"
const CookiesPath = "config/credentials/cookies-www-dia-es.txt"
const SheetReadRange = "A2:F300"

func setupLogger() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)
}

func main() {
	setupLogger()

	startDate := flag.String("start", "01-01-0001", "Override the default start date of 1st day of current month. Format is DD-MM-YYYY")
	showVersion := flag.Bool("version", false, "Show version and build date")
	flag.Parse()

	if *showVersion {
		fmt.Printf("Version: %s\nBuild Date: %s\n", Version, BuildDate)
		os.Exit(0)
	}

	ctx := context.Background()

	var currentTime time.Time

	if *startDate != "01-01-0001" {
		slog.Debug("Start date provided", "DATE", *startDate)
		parsedDateOverride, err := time.Parse("02-01-2006", *startDate)
		slog.Info(parsedDateOverride.String())
		if err != nil {
			slog.Info("Unable to parse date override, defaulting to today")
			currentTime = time.Now()
		} else {
			currentTime = parsedDateOverride
		}
	} else {
		currentTime = time.Now()
	}

	currentYear, currentMonth, currentDay := currentTime.Date()
	fmt.Println("Current date:", currentYear, constants.FromTimeMonth(currentMonth), currentDay)

	sheetPageName := getSheetName(currentYear, currentMonth)

	firstOfMonth := time.Date(currentYear, currentMonth, 1, 0, 0, 0, 0, time.Now().Location())

	oAuthConfig := token.GetOauthConfig(CredsFilePath, sheets.SpreadsheetsScope, gmail.MailGoogleComScope)

	gmailManager := gm.GetGmailManager(ctx, oAuthConfig, TokenFilePath)

	sheetsManager := sh.GetSheetManager(ctx, oAuthConfig, TokenFilePath, CredsFilePath, sheetPageName)

	slog.Debug("Fetching date column data from " + SheetReadRange)

	ticketsFromSheet, _ := sh.GetSheetTicketList(sheetsManager, SheetReadRange)

	slog.Info(fmt.Sprintf("Fetched %d tickets for %v %v %v", len(ticketsFromSheet), currentYear, constants.FromTimeMonth(currentMonth), currentDay))

	lastWrittenRow := sh.GetLastWrittenRowIndex(ticketsFromSheet)

	newTicketsMap := getAllTicketsFromStores(gmailManager, ticketsFromSheet, firstOfMonth)

	newTicketsList := CombineTickets(newTicketsMap)
	slog.Info("New tickets collected", "TICKETS", newTicketsList)

	uploadNewTickets(newTicketsList, sheetsManager, lastWrittenRow)

}

func getSheetName(currentYear int, currentMonth time.Month) string {
	var sheetPageName string
	if constants.TestMode {
		sheetPageName = "testpage"
	} else {
		yearStr := fmt.Sprintf("%02d", currentYear%100)
		sheetPageName = fmt.Sprintf("%s %s", string(constants.FromTimeMonth(currentMonth)), yearStr)
	}
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
	var wg sync.WaitGroup

	excluded, errLoadExcluded := internal.LoadExcluded("config/excluded.json")
	if errLoadExcluded != nil {
		slog.Warn("Unable to load excluded Ids", "ERROR", errLoadExcluded)
		return allTickets
	}

	for _, store := range []string{constants.MERCADONA, constants.DIA, constants.CARREFOUR} {
		wg.Go(func() {
			lastEntryFromSheets := sh.GetLastEntryForStore(sheetEntries, store)
			lastEntryToCompare := getLastEntryToCompare(lastEntryFromSheets, store, firstOfMonth)

			newFoundTickets := getTicketsForStore(gmManager, store, lastEntryToCompare, excluded)
			allTickets[store] = newFoundTickets
		})
	}
	wg.Wait()
	return allTickets
}

func getTicketsForStore(gmManager gm.Manager, store string, lastEntryToCompare sh.Entry, excluded internal.Excluded) []internal.Ticket {
	var newFoundTickets []internal.Ticket
	switch store {
	case constants.MERCADONA:
		newFoundTickets = getMercadonaTickets(gmManager, lastEntryToCompare, excluded)
	case constants.DIA:
		newFoundTickets = getDiaTickets(lastEntryToCompare, excluded)
	case constants.CARREFOUR:
		newFoundTickets = getCarrefourTickets(gmManager, lastEntryToCompare, excluded)
	}
	slog.Info("Found new tickets", "TICKETS", len(newFoundTickets), "STORE", store)
	return newFoundTickets
}

func getLastEntryToCompare(lastEntryFromSheets *sh.Entry, store string, firstOfMonth time.Time) sh.Entry {
	if lastEntryFromSheets == nil {
		slog.Info("No last ticket found for store, using first of month date", "STORE", store, "DATE", firstOfMonth)
		return sh.Entry{Date: firstOfMonth, Total: 0}
	}

	slog.Info("Last ticket found for store, using entry date", "STORE", store, "DATE", lastEntryFromSheets.Date)
	return *lastEntryFromSheets
}

func getMercadonaTickets(manager gm.Manager, lastEntryToCompare sh.Entry, excluded internal.Excluded) []internal.Ticket {
	tickets := mercadona.GetTicketList(manager, lastEntryToCompare, excluded.MercadonaIds)
	return tickets
}

func getDiaTickets(lastEntryToCompare sh.Entry, excluded internal.Excluded) []internal.Ticket {
	page, cleanup, err := dia.LoginToDia(CredsFilePath, CookiesPath)
	if err != nil {
		slog.Warn("Unable to fetch tickets from DIA", "ERROR", err)
		cleanup()
		return []internal.Ticket{}
	}
	newDiaTickets := dia.GetTicketList(page, lastEntryToCompare, excluded.DiaIDs)
	cleanup()
	page.Close()
	return newDiaTickets
}

func getCarrefourTickets(manager gm.Manager, lastEntryToCompare sh.Entry, excluded internal.Excluded) []internal.Ticket {
	page, cleanup, err := carrefour.LoginToCarrefour(manager, CredsFilePath)
	if err != nil {
		cleanup()
		return []internal.Ticket{}
	}
	newTickets := carrefour.GetTicketList(page, lastEntryToCompare, excluded.CarrefourIds)
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
