package sh

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

	"golang.org/x/oauth2"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

type Ticket struct {
	Date     time.Time
	shop     string
	FirstRow int
}

func GroceryTicketToString(ticket Ticket) string {
	formattedDate := ticket.Date.Format(constants.TicketDateFormat)
	return fmt.Sprintf("Grocery Ticket: Shop=%s, Date=%s\n", ticket.shop, formattedDate)
}

type Config struct {
	Sheets struct {
		MainID string `json:"main-id"`
	} `json:"sheets"`
}

type Manager struct {
	Service  *sheets.Service
	PageName string
	config   Config
	sheetId  int64
}

func GetSheetManager(ctx context.Context, config *oauth2.Config, tokFile string, credsFile string, sheetName string) Manager {
	sheetService, err := GetSheetService(ctx, config, tokFile)
	if err != nil {
		log.Fatalf("Unable to retrieve Sheets client: %v", err)
	}

	sheetsConfig, err := GetSheetsConfig(credsFile)
	if err != nil {
		log.Fatalf("Failed to get Sheets config: %v", err)
	}

	sheetId, _ := GetSheetID(sheetService, sheetsConfig.Sheets.MainID, sheetName)

	return Manager{Service: sheetService, config: sheetsConfig, sheetId: sheetId, PageName: sheetName}
}

func GetSheetService(ctx context.Context, config *oauth2.Config, tokFile string) (*sheets.Service, error) {
	client := token.GetClient(config, tokFile)
	return sheets.NewService(ctx, option.WithHTTPClient(client))
}

func GetSheetsConfig(credentialsPath string) (Config, error) {
	b, err := os.ReadFile(credentialsPath)
	if err != nil {
		return Config{}, fmt.Errorf("unable to read client secret file: %w", err)
	}
	var sheetsConfig Config
	if err := json.Unmarshal(b, &sheetsConfig); err != nil {
		slog.Info("Failed to unmarshal credentials config", "ERROR", err)
		return Config{}, err
	}
	if sheetsConfig.Sheets.MainID == "" {
		return Config{}, fmt.Errorf("main sheet ID is empty")
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

func ReadFromSheet(manager Manager, readRange string) *sheets.ValueRange {
	resp, err := manager.Service.Spreadsheets.Values.Get(manager.config.Sheets.MainID, readRange).Do()
	if err != nil {
		log.Fatalf("Unable to retrieve data from sheet: %v", err)
	}
	return resp
}

func GetSheetTicketList(manager Manager, cellRange string) ([]Ticket, error) {
	readRange := fmt.Sprintf("%s!%s", manager.PageName, cellRange)

	resp := ReadFromSheet(manager, readRange)

	tickets := make([]Ticket, 0)
	if len(resp.Values) == 0 {
		slog.Info("No data found in searched sheets range.")
	} else {
		for i, row := range resp.Values {
			if len(row) >= 2 {
				date := utils.StringToDate(utils.ExtractString(row[0]))
				ticket := Ticket{Date: date, shop: utils.ExtractString(row[1]), FirstRow: i}
				tickets = append(tickets, ticket)
			}
		}
	}
	return tickets, nil
}

func GetLastWrittenRowIndex(manager Manager) int {
	readRange := fmt.Sprintf("%s!E2:300", manager.PageName)
	data := ReadFromSheet(manager, readRange)
	var lastRow = 0
	for i, row := range data.Values {
		if len(row) > 1 && row[0].(string) == "TOTAL" && len(row[0].(string)) > 0 {
			lastRow = i + 2
		}
	}

	return lastRow
}

func GetLastTicketForShop(tickets []Ticket, shopName string) (Ticket, error) {
	for in := len(tickets) - 1; in >= 0; in-- {
		if tickets[in].shop == shopName {
			return tickets[in], nil
		}
	}
	return Ticket{}, fmt.Errorf("no ticket found for shop %s", shopName)
}

func WriteToSheet(manager Manager, valueRange *sheets.ValueRange, firstRow int) int {
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

func FormatTicketBlock(manager Manager, firstRow int, ticketItems int) error {
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
