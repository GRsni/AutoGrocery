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
	"strings"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

type Entry struct {
	Date     time.Time
	Id       string
	Store    string
	FirstRow int
	LastRow  int
	Total    float64
}

const HeaderSize = 1

func EntryToStr(ticket Entry) string {
	formattedDate := ticket.Date.Format(constants.TicketDateFormat)
	return fmt.Sprintf("Grocery Ticket: Shop=%s, Date=%s\n", ticket.Store, formattedDate)
}

type ValuesGetCall interface {
	Do(...googleapi.CallOption) (*sheets.ValueRange, error)
}
type ValuesUpdateCall interface {
	ValueInputOption(string) ValuesUpdateCall
	Do(...googleapi.CallOption) (*sheets.UpdateValuesResponse, error)
}
type SheetsAPI interface {
	Get(spreadsheetID, readRange string) (*sheets.ValueRange, error)
	Update(spreadsheetID, writeRange string, vr *sheets.ValueRange, inputOption string) (*sheets.UpdateValuesResponse, error)
	BatchUpdate(spreadsheetID string, req *sheets.BatchUpdateSpreadsheetRequest) (*sheets.BatchUpdateSpreadsheetResponse, error)
}

type Config struct {
	Sheets struct {
		MainID string `json:"main-id"`
	} `json:"sheets"`
}

type Manager struct {
	PageName string
	config   Config
	sheetId  int64
	service  SheetsAPI
}

type realService struct{ svc *sheets.Service }

func (r *realService) Get(id, rng string) (*sheets.ValueRange, error) {
	return r.svc.Spreadsheets.Values.Get(id, rng).Do()
}

func (r *realService) Update(id, rng string, vr *sheets.ValueRange, inputOption string) (*sheets.UpdateValuesResponse, error) {
	return r.svc.Spreadsheets.Values.Update(id, rng, vr).ValueInputOption(inputOption).Do()
}

func (r *realService) BatchUpdate(id string, req *sheets.BatchUpdateSpreadsheetRequest) (*sheets.BatchUpdateSpreadsheetResponse, error) {
	return r.svc.Spreadsheets.BatchUpdate(id, req).Do()
}

func GetSheetManager(ctx context.Context, config *oauth2.Config, tokFile string, credsFile string, sheetName string) Manager {
	sheetService, err := getSheetService(ctx, config, tokFile)
	if err != nil {
		log.Fatalf("Unable to retrieve Sheets client: %v", err)
	}

	sheetsConfig, err := getSheetsConfig(credsFile)
	if err != nil {
		log.Fatalf("Failed to get Sheets config: %v", err)
	}

	sheetId, _ := getSheetID(sheetService, sheetsConfig.Sheets.MainID, sheetName)

	return Manager{config: sheetsConfig, sheetId: sheetId, PageName: sheetName, service: &realService{svc: sheetService}}
}

func getSheetService(ctx context.Context, config *oauth2.Config, tokFile string) (*sheets.Service, error) {
	client := token.GetClient(config, tokFile)
	return sheets.NewService(ctx, option.WithHTTPClient(client))
}

func getSheetsConfig(credentialsPath string) (Config, error) {
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

func getSheetID(service *sheets.Service, spreadsheetID string, sheetName string) (int64, error) {
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

func ReadFromSheet(manager Manager, readRange string) (*sheets.ValueRange, error) {
	resp, err := manager.service.Get(manager.config.Sheets.MainID, readRange)
	if err != nil {
		slog.Error("Unable to retrieve data from sheet", "ERROR", err)
		return nil, err
	}
	return resp, nil
}

func GetSheetTicketList(manager Manager, cellRange string) ([]Entry, error) {
	readRange := fmt.Sprintf("%s!%s", manager.PageName, cellRange)

	resp, err := ReadFromSheet(manager, readRange)
	if err != nil {
		return nil, fmt.Errorf("got no data from sheet: %v", err)
	}

	tickets := make([]Entry, 0)
	if len(resp.Values) == 0 {
		slog.Info("No data found in searched sheets range.")
	} else {
		for rowIndex := 0; rowIndex < len(resp.Values); rowIndex++ {
			row := resp.Values[rowIndex]
			if len(row) < 6 {
				return nil, fmt.Errorf("sheet only supports 6 row setup, found %v rows", len(row))
			}
			if isRowEmpty(row) {
				slog.Debug("Empty row found, no more tickets")
				break
			}
			ticket, finalRow, errGetSheetEntry := getEntryFromSheet(resp, rowIndex)
			if errGetSheetEntry != nil {
				slog.Warn("Error while trying to get entry from sheet", "ERROR", errGetSheetEntry)
				rowIndex = finalRow
				continue
			}
			rowIndex = finalRow
			tickets = append(tickets, *ticket)
		}
	}
	return tickets, nil
}

func getEntryFromSheet(resp *sheets.ValueRange, startingRow int) (*Entry, int, error) {
	var date time.Time
	var store string
	var total float64
	var finalRow int
	var ticketId string
	row := resp.Values[startingRow]
	if len(utils.ExtractString(row[0])) > 0 && len(utils.ExtractString(row[1])) > 0 {
		// Found a new ticket, iterate until TOTAL found
		date = utils.StringToDate(utils.ExtractString(row[0]))
		store, ticketId = getStoreNameAndId(utils.ExtractString(row[1]))
		for j := startingRow + 1; j < len(resp.Values); j++ {
			row = resp.Values[j]
			if utils.ExtractString(row[4]) != "TOTAL" {
				continue
			}
			total = utils.ParsePrice(utils.ExtractString(row[5]))
			finalRow = j
			break

		}
		if finalRow == 0 {
			return nil, len(resp.Values), fmt.Errorf("unable to find TOTAL row in rest of sheet")
		}
	}
	ticket := Entry{Date: date, Store: store, Id: ticketId, FirstRow: startingRow + 2, LastRow: finalRow + 2, Total: total}
	return &ticket, finalRow, nil
}

func getStoreNameAndId(input string) (string, string) {
	stringParts := strings.Split(input, "\n")
	id := stringParts[len(stringParts)-1]
	name := stringParts[0]
	return name, id
}

func isRowEmpty(row []any) bool {
	size := len(row)
	for i := 0; i < size-1; i++ {
		if len(utils.ExtractString(row[i])) != 0 {
			return false
		}
	}
	return utils.ExtractString(row[size-1]) == "0,00 €"
}

func GetLastWrittenRowIndex(entries []Entry) int {
	numEntries := len(entries)
	if numEntries == 0 {
		return HeaderSize
	}
	lastRow := entries[numEntries-1].LastRow
	if lastRow == 0 {
		return HeaderSize
	}
	return lastRow
}

func GetLastEntryForStore(tickets []Entry, shopName string) *Entry {
	for in := len(tickets) - 1; in >= 0; in-- {
		if tickets[in].Store == shopName {
			return &tickets[in]
		}
	}
	return nil
}

func WriteToSheet(manager Manager, valueRange *sheets.ValueRange, firstRow int) int {
	ticketItems := len(valueRange.Values)
	writeRange := fmt.Sprintf("%s!A%d:F%d", manager.PageName, firstRow, ticketItems+firstRow+1)
	totalFunction := fmt.Sprintf("=SUM(F%d:F%d)", firstRow, firstRow+ticketItems-1)
	valueRange.Values = append(valueRange.Values, []any{"", "", "", "", "TOTAL", totalFunction})

	resp, err := manager.service.Update(manager.config.Sheets.MainID, writeRange, valueRange, "USER_ENTERED")
	if err != nil {
		slog.Error("Unable to write data from sheet", "ERROR", err)
		return firstRow
	}
	slog.Info("Added rows to excel sheet", "CELL_COUNT", resp.UpdatedCells)
	err = FormatTicketBlock(manager, firstRow, ticketItems)
	if err != nil {
		slog.Warn("Unable to give format to ticket", "ERROR", err)
	}
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

	_, err := manager.service.BatchUpdate(manager.config.Sheets.MainID, &sheets.BatchUpdateSpreadsheetRequest{
		Requests: requests,
	})
	return err
}
