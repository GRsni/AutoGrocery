package main

import (
	"autoGrocery/internal/dia"
	"autoGrocery/internal/sheets_handler"
	"autoGrocery/pkg/constants"
	"context"
	"fmt"
	"log"
	"time"
)

type GroceryTicket struct {
	date time.Time
	shop string
}

func ticketToString(ticket GroceryTicket) string {
	formattedDate := ticket.date.Format(constants.TicketDateFormat)
	return fmt.Sprintf("Grocery Ticket: Shop=%s, Date=%s\n", ticket.shop, formattedDate)
}

func main() {
	credentialsPath := "config/credentials/credentials.json"
	ctx := context.Background()

	tokFile := "config/credentials/token.json"
	sheetService, err := sheets_handler.GetSheetService(ctx, tokFile, credentialsPath)
	if err != nil {
		log.Fatalf("Unable to retrieve Sheets client: %v", err)
	}

	sheetsConfig, err := sheets_handler.GetSheetsConfig(credentialsPath)
	if err != nil {
		log.Fatalf("Failed to get Sheets config: %v", err)
	}

	currentYear, currentMonth, currentDay := time.Now().Date()
	fmt.Println("Current date:", currentYear, constants.FromTimeMonth(currentMonth), currentDay)
	//yearStr := fmt.Sprintf("%02d", currentYear%100)
	//readRange := fmt.Sprintf("%s %s!%s", string(constants.FromTimeMonth(currentMonth)), yearStr, "A2:B200")
	readRange := fmt.Sprintf("%s!%s", "testpage", "A2:B200")
	fmt.Println("Fetching date column data from" + readRange)

	resp, err := sheetService.Spreadsheets.Values.Get(sheetsConfig.Sheets.MainID, readRange).Do()
	if err != nil {
		log.Fatalf("Unable to retrieve data from sheet: %v", err)
	}

	tickets := make([]GroceryTicket, 0)
	if len(resp.Values) == 0 {
		fmt.Println("No data found.")
	} else {
		for _, row := range resp.Values {
			//for j, cell := range row {
			//fmt.Printf("[%d][%d]: %v\n", i, j, cell)
			//}
			if len(row) == 2 {
				tickets = append(tickets, getTicketFromCell(extractString(row[0]), extractString(row[1])))
			}
		}
	}

	fmt.Println("Fetched", len(tickets), "tickets for", currentYear, constants.FromTimeMonth(currentMonth), currentDay)

	diaTicket, err := getLastTicketForShop(tickets, constants.DIA)
	if err != nil {
		log.Fatalf("Unable to retrieve last ticket for Dia: %v", err)
	}
	fmt.Printf("Last ticket found for DIA %v\n", ticketToString(diaTicket))

	cookiesPath := "config/credentials/cookies-www-dia-es.txt"
	diaPage, cleanup, err := dia.LoginToDia(credentialsPath, cookiesPath)
	if err != nil {
		cleanup()
		return
	}
	newDiaTickets := dia.GetTicketList(diaPage, diaTicket.date)
	fmt.Printf("Found %d new tickets for DIA\n", len(newDiaTickets))
	defer cleanup()

	diaPage.Close()
}

func extractString(val any) string {
	switch v := val.(type) {
	case string:
		return v
	case float64: // Google Sheets returns numbers as float64
		return fmt.Sprintf("%g", v)
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", v)
	}
}

func getTicketFromCell(dateCell string, shop string) GroceryTicket {
	date, err := time.Parse("02/01/2006", dateCell)
	if err != nil {
		log.Print("Failed to parse date cell with default format, trying another.\n", err)
		date, _ = time.Parse("02/1/2006", dateCell)
	}
	return GroceryTicket{
		date: date,
		shop: shop,
	}
}

func getLastTicketForShop(tickets []GroceryTicket, shopName string) (GroceryTicket, error) {
	for in := len(tickets) - 1; in >= 0; in-- {
		if tickets[in].shop == shopName {
			return tickets[in], nil
		}
	}
	return GroceryTicket{}, fmt.Errorf("no ticket found for shop %s", shopName)
}
