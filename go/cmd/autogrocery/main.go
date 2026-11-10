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
	"maps"
	"os"
	"slices"
	"sort"
	"strings"
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
	stores := flag.String("stores", "", "Override the default list of stores to fetch. Send a command separeted list (DIA,MERCADONA,...)")
	flag.Parse()

	if *showVersion {
		fmt.Printf("Version: %s\nBuild Date: %s\n", Version, BuildDate)
		os.Exit(0)
	}

	currentTime := parseCurrentTimeOverride(startDate)

	storesToFetch := parseStoreOverride(stores)

	currentYear, currentMonth, currentDay := currentTime.Date()
	fmt.Println("Current date:", currentYear, constants.FromTimeMonth(currentMonth), currentDay)

	sheetPageName := getSheetName(currentYear, currentMonth)

	firstOfMonth := time.Date(currentYear, currentMonth, 1, 0, 0, 0, 0, time.Now().Location())

	ctx := context.Background()

	oAuthConfig := token.GetOauthConfig(CredsFilePath, sheets.SpreadsheetsScope, gmail.MailGoogleComScope)

	gmailManager := gm.GetGmailManager(ctx, oAuthConfig, TokenFilePath)

	sheetsManager := sh.GetSheetManager(ctx, oAuthConfig, TokenFilePath, CredsFilePath, sheetPageName)

	slog.Debug("Fetching date column data from " + SheetReadRange)

	ticketsFromSheet, err := sh.GetSheetTicketList(sheetsManager, SheetReadRange)
	if err != nil {
		slog.Error("Couldnt load tickets from sheet", "ERROR", err)
		os.Exit(1)
	}

	slog.Info(fmt.Sprintf("Fetched %d tickets for %v %v %v", len(ticketsFromSheet), currentYear, constants.FromTimeMonth(currentMonth), currentDay))

	newTicketsMap := getAllTicketsFromStores(gmailManager, ticketsFromSheet, firstOfMonth, storesToFetch)

	newTicketsList := CombineTickets(newTicketsMap)

	if len(newTicketsList) == 0 {
		slog.Info("No new tickets were loaded from stores, exiting early")
		return
	}

	slog.Info("New tickets collected", "TICKETS", newTicketsList)

	combinedEntries := MergeAllTickets(ticketsFromSheet, newTicketsList)

	uploadNewTickets(combinedEntries, sheetsManager)

}

func parseCurrentTimeOverride(startDate *string) time.Time {
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
	return currentTime
}

func parseStoreOverride(stores *string) []string {
	if *stores == "" {
		return constants.Stores
	}

	storeOverride := make([]string, 0)
	parsedStores := strings.SplitSeq(strings.TrimSpace(*stores), ",")
	for store := range parsedStores {
		if slices.Contains(constants.Stores, store) {
			storeOverride = append(storeOverride, store)
		}
	}
	if len(storeOverride) > 0 {
		return storeOverride
	}
	return constants.Stores
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

func uploadNewTickets(newEntryList []sh.Entry, sheetsManager sh.Manager) {
	for _, entryToUpload := range newEntryList {
		valueRange, err := entryToUpload.ToValueRange()
		if err != nil {
			slog.Info("Unable to create value range object, skipping", "ERROR", err)
			continue
		}
		_ = sh.WriteToSheet(sheetsManager, valueRange, entryToUpload.FirstRow+1)
	}
}

func getAllTicketsFromStores(gmManager gm.Manager, sheetEntries []sh.Entry, firstOfMonth time.Time, storesToFetch []string) map[string][]internal.Ticket {
	allTickets := make(map[string][]internal.Ticket)

	excluded, errLoadExcluded := internal.LoadExcluded("config/excluded.json")
	if errLoadExcluded != nil {
		slog.Warn("Unable to load excluded Ids", "ERROR", errLoadExcluded)
		return allTickets
	}

	results := make(chan map[string][]internal.Ticket, len(storesToFetch))
	var wg sync.WaitGroup

	for _, store := range storesToFetch {
		wg.Add(1)
		go func(store string) {
			defer wg.Done()
			lastEntryFromSheets := sh.GetLastEntryForStore(sheetEntries, store)
			lastEntryToCompare := getLastEntryToCompare(lastEntryFromSheets, store, firstOfMonth)

			newFoundTickets := getTicketsForStore(gmManager, store, lastEntryToCompare, excluded)
			results <- map[string][]internal.Ticket{store: newFoundTickets}
		}(store)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	for res := range results {
		maps.Copy(allTickets, res)
	}

	return allTickets
}

func getLastEntryToCompare(lastEntryFromSheets *sh.Entry, store string, firstOfMonth time.Time) sh.Entry {
	if lastEntryFromSheets == nil {
		slog.Info("No last ticket found for store, using first of month date", "STORE", store, "DATE", firstOfMonth)
		return sh.Entry{Date: firstOfMonth, Total: 0}
	}

	slog.Info("Last ticket found for store, using entry date", "STORE", store, "DATE", lastEntryFromSheets.Date)
	return *lastEntryFromSheets
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

func getMercadonaTickets(manager gm.Manager, lastEntryToCompare sh.Entry, excluded internal.Excluded) []internal.Ticket {
	tickets := mercadona.GetTicketList(manager, lastEntryToCompare, excluded.MercadonaIds)
	return tickets
}

func getDiaTickets(lastEntryToCompare sh.Entry, excluded internal.Excluded) []internal.Ticket {
	page, cleanup, err := dia.LoginToDia(CredsFilePath, CookiesPath)
	if err != nil {
		slog.Warn("Unable to fetch tickets from DIA", "ERROR", err)
		if cleanup != nil {
			cleanup()
		}
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
	ticketList := make([]internal.Ticket, 0)

	fetchedStores := make([]string, 0, len(allTickets))
	for k := range allTickets {
		fetchedStores = append(fetchedStores, k)
	}

	for _, shop := range fetchedStores {
		ticketList = append(ticketList, allTickets[shop]...)
	}

	sort.SliceStable(ticketList, func(i, j int) bool {
		return ticketList[i].Date.Before(ticketList[j].Date)
	})

	return ticketList
}

func MergeAllTickets(sheetEntries []sh.Entry, newTickets []internal.Ticket) []sh.Entry {
	allEntries := make([]sh.Entry, 0)
	allEntries = append(allEntries, sheetEntries...)

	for _, ticket := range newTickets {
		allEntries = append(allEntries, ticket.ToEntry(0))
	}

	sort.SliceStable(allEntries, func(i, j int) bool {
		return allEntries[i].Date.Before(allEntries[j].Date)
	})

	var firstRow = sh.HeaderSize
	for i := range allEntries {
		allEntries[i].FirstRow = firstRow
		allEntries[i].LastRow = allEntries[i].FirstRow + len(allEntries[i].Items)
		firstRow = allEntries[i].LastRow + 1
	}

	return allEntries
}
