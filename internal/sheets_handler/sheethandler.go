package sheets_handler

import (
	"autoGrocery/internal/token"
	"autoGrocery/pkg/constants"
	"autoGrocery/utils"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

type SheetsTicket struct {
	Date     time.Time
	shop     string
	FirstRow int
}

func GroceryTicketToString(ticket SheetsTicket) string {
	formattedDate := ticket.Date.Format(constants.TicketDateFormat)
	return fmt.Sprintf("Grocery Ticket: Shop=%s, Date=%s\n", ticket.shop, formattedDate)
}

type SheetsConfig struct {
	Sheets struct {
		MainID string `json:"main-id"`
	} `json:"sheets"`
}

type SheetsManager struct {
	Service  *sheets.Service
	PageName string
	config   SheetsConfig
	sheetId  int64
}

func GetSheetManager(ctx context.Context, tokFile string, credsFile string, sheetName string) SheetsManager {
	sheetService, err := GetSheetService(ctx, tokFile, credsFile)
	if err != nil {
		log.Fatalf("Unable to retrieve Sheets client: %v", err)
	}

	sheetsConfig, err := GetSheetsConfig(credsFile)
	if err != nil {
		log.Fatalf("Failed to get Sheets config: %v", err)
	}

	sheetId, _ := GetSheetID(sheetService, sheetsConfig.Sheets.MainID, sheetName)

	return SheetsManager{Service: sheetService, config: sheetsConfig, sheetId: sheetId, PageName: sheetName}
}

func GetSheetService(ctx context.Context, tokFile string, credsFile string) (*sheets.Service, error) {
	b, err := os.ReadFile(credsFile)
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}
	// If modifying these scopes, delete your previously saved token.json.
	config, err := google.ConfigFromJSON(b, "https://www.googleapis.com/auth/spreadsheets")
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	client := token.GetClient(config, tokFile)

	return sheets.NewService(ctx, option.WithHTTPClient(client))
}

func GetSheetsConfig(credentialsPath string) (SheetsConfig, error) {
	b, err := os.ReadFile(credentialsPath)
	if err != nil {
		return SheetsConfig{}, fmt.Errorf("unable to read client secret file: %w", err)
	}
	var sheetsConfig SheetsConfig
	if err := json.Unmarshal(b, &sheetsConfig); err != nil {
		slog.Info("Failed to unmarshal credentials config", "ERROR", err)
		return SheetsConfig{}, err
	}
	if sheetsConfig.Sheets.MainID == "" {
		return SheetsConfig{}, fmt.Errorf("main sheet ID is empty")
	}
	return sheetsConfig, nil
}

func GetSheetID(service *sheets.Service, spreadsheetID string, sheetName string) (int64, error) {
	spreadsheet, err := service.Spreadsheets.Get(spreadsheetID).Do()
	if err != nil {
		return 0, fmt.Errorf("unable to get spreadsheet: %w", err)
	}

	for _, sheet := range spreadsheet.Sheets {
		if sheet.Properties.Title == sheetName {
			return sheet.Properties.SheetId, nil
		}
	}

	return 0, fmt.Errorf("sheet %q not found", sheetName)
}

func ReadFromSheet(manager SheetsManager, readRange string) *sheets.ValueRange {
	resp, err := manager.Service.Spreadsheets.Values.Get(manager.config.Sheets.MainID, readRange).Do()
	if err != nil {
		log.Fatalf("Unable to retrieve data from sheet: %v", err)
	}
	return resp
}

func GetSheetTicketList(manager SheetsManager, cellRange string) ([]SheetsTicket, error) {
	readRange := fmt.Sprintf("%s!%s", manager.PageName, cellRange)

	resp := ReadFromSheet(manager, readRange)

	tickets := make([]SheetsTicket, 0)
	if len(resp.Values) == 0 {
		slog.Info("No data found in searched sheets range.")
	} else {
		for i, row := range resp.Values {
			if len(row) >= 2 {
				date := utils.GetDateFromCell(utils.ExtractString(row[0]))
				ticket := SheetsTicket{Date: date, shop: utils.ExtractString(row[1]), FirstRow: i}
				tickets = append(tickets, ticket)
			}
		}
	}
	return tickets, nil
}

func GetLastTicketForShop(tickets []SheetsTicket, shopName string) (SheetsTicket, error) {
	for in := len(tickets) - 1; in >= 0; in-- {
		if tickets[in].shop == shopName {
			return tickets[in], nil
		}
	}
	return SheetsTicket{}, fmt.Errorf("no ticket found for shop %s", shopName)
}

func WriteToSheet(manager SheetsManager, valueRange *sheets.ValueRange, firstRow int) int {
	ticketItems := len(valueRange.Values)
	fmt.Println(ticketItems)
	writeRange := fmt.Sprintf("%s!A%d:F%d", manager.PageName, firstRow, ticketItems+firstRow+1)
	totalFunction := fmt.Sprintf("=SUM(F%d:F%d)", firstRow, firstRow+ticketItems-1)
	valueRange.Values = append(valueRange.Values, []any{"", "", "", "", "TOTAL", totalFunction})

	resp, err := manager.Service.Spreadsheets.Values.Update(manager.config.Sheets.MainID, writeRange, valueRange).ValueInputOption("USER_ENTERED").Do()
	if err != nil {
		slog.Error("Unable to write data from sheet", "ERROR", err)
	}
	slog.Info("Added rows to excel sheet", "CELL_COUNT", resp.UpdatedCells)
	FormatTicketBlock(manager, firstRow, ticketItems)
	return ticketItems + 1
}

func FormatTicketBlock(manager SheetsManager, firstRow int, ticketItems int) error {
	totalRowIdx := int64(firstRow + ticketItems - 1)

	// #cfe2f3 -> R:207 G:226 B:243 normalized to 0-1
	blueColor := &sheets.Color{
		Red:   207.0 / 255.0,
		Green: 226.0 / 255.0,
		Blue:  243.0 / 255.0,
		Alpha: 1,
	}

	requests := []*sheets.Request{
		// Blue background + bold for entire TOTAL row (columns C:F = indices 2:6)
		{
			RepeatCell: &sheets.RepeatCellRequest{
				Range: &sheets.GridRange{
					SheetId:          manager.sheetId,
					StartRowIndex:    totalRowIdx,
					EndRowIndex:      totalRowIdx + 1,
					StartColumnIndex: 0,
					EndColumnIndex:   6,
				},
				Cell: &sheets.CellData{
					UserEnteredFormat: &sheets.CellFormat{
						TextFormat:      &sheets.TextFormat{Bold: true},
						BackgroundColor: blueColor,
					},
				},
				Fields: "userEnteredFormat(textFormat,backgroundColor)",
			},
		},
		// Box border around the total VALUE cell (column F = index 5)
		{
			RepeatCell: &sheets.RepeatCellRequest{
				Range: &sheets.GridRange{
					SheetId:          manager.sheetId,
					StartRowIndex:    totalRowIdx,
					EndRowIndex:      totalRowIdx + 1,
					StartColumnIndex: 4,
					EndColumnIndex:   6,
				},
				Cell: &sheets.CellData{
					UserEnteredFormat: &sheets.CellFormat{
						Borders: &sheets.Borders{
							Top:    &sheets.Border{Style: "SOLID", Width: 1},
							Bottom: &sheets.Border{Style: "SOLID", Width: 1},
							Left:   &sheets.Border{Style: "SOLID", Width: 1},
							Right:  &sheets.Border{Style: "SOLID", Width: 1},
						},
					},
				},
				Fields: "userEnteredFormat(borders)",
			},
		},
		// Merge the date cell vertically across ticket rows (column A = index 0)
		{
			MergeCells: &sheets.MergeCellsRequest{
				Range: &sheets.GridRange{
					SheetId:          manager.sheetId,
					StartRowIndex:    int64(firstRow) - 1,
					EndRowIndex:      totalRowIdx,
					StartColumnIndex: 0,
					EndColumnIndex:   1,
				},
				MergeType: "MERGE_COLUMNS",
			},
		},
		// Merge the store name cell vertically (column B = index 1)
		{
			MergeCells: &sheets.MergeCellsRequest{
				Range: &sheets.GridRange{
					SheetId:          manager.sheetId,
					StartRowIndex:    int64(firstRow) - 1,
					EndRowIndex:      totalRowIdx,
					StartColumnIndex: 1,
					EndColumnIndex:   2,
				},
				MergeType: "MERGE_COLUMNS",
			},
		},
	}

	_, err := manager.Service.Spreadsheets.BatchUpdate(manager.config.Sheets.MainID, &sheets.BatchUpdateSpreadsheetRequest{
		Requests: requests,
	}).Do()
	return err
}
