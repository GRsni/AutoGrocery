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

const HeaderSize = 1

type Item struct {
	Name   string
	Amount float64
	Price  float64
}

type Items []Item

type Entry struct {
	Date     time.Time
	Id       string
	Store    string
	FirstRow int
	LastRow  int
	Total    float64
	Items    Items
}

type UploadableEntry interface {
	EntryToStr() string
	ToValueRange() *sheets.ValueRange
}

func (entry Entry) EntryToStr() string {
	formattedDate := entry.Date.Format(constants.TicketDateFormat)
	return fmt.Sprintf("Grocery Ticket: Shop: %s, Date: %s, Item count: %d,  Total: %2f", entry.Store, formattedDate, len(entry.Items), entry.Total)
}

func (entry Entry) ToValueRange() (*sheets.ValueRange, error) {
	var values [][]any

	for _, item := range entry.Items {
		name := item.Name // copy to avoid mutating the original
		if len(name) > 0 && name[0] == '+' {
			name = "'" + name
		}
		row := []any{"", "", name, item.Amount, item.Price}
		values = append(values, row)
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("ticket contains no items to upload")
	}
	values[0][0] = entry.Date.Format(constants.TicketDateFormat)
	values[0][1] = entry.Store + "\n" + entry.Id

	valRange := sheets.ValueRange{MajorDimension: "ROWS", Values: values}

	return &valRange, nil
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
	client, errGetClient := token.GetClient(config, tokFile)
	if errGetClient != nil {
		return nil, errGetClient
	}
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
	items := make(Items, 0)
	row := resp.Values[startingRow]

	if len(utils.ExtractString(row[0])) > 0 && len(utils.ExtractString(row[1])) > 0 {
		// Found a new ticket, iterate until TOTAL found
		date = utils.StringToDate(utils.ExtractString(row[0]))
		store, ticketId = getStoreNameAndId(utils.ExtractString(row[1]))
		for j := startingRow + 1; j < len(resp.Values); j++ {

			item := getEntryItem(row)

			if item != nil {
				items = append(items, *item)
			}

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
	ticket := Entry{
		Date:     date,
		Store:    store,
		Id:       ticketId,
		FirstRow: startingRow + 2,
		LastRow:  finalRow + 2,
		Total:    total,
		Items:    items}
	return &ticket, finalRow, nil
}

func getStoreNameAndId(input string) (string, string) {
	stringParts := strings.Split(input, "\n")
	id := stringParts[len(stringParts)-1]
	name := stringParts[0]
	return name, id
}

func getEntryItem(row []any) *Item {
	if len(row) < 5 {
		return nil
	}
	if utils.ExtractString(row[2]) != "" && utils.ExtractString(row[3]) != "" && utils.ExtractString(row[4]) != "" {
		amount := utils.ParseQty(utils.ExtractString(row[3]))
		price := utils.ParsePrice(utils.ExtractString(row[4]))

		return &Item{
			Name:   fmt.Sprint(row[2]),
			Amount: amount,
			Price:  price,
		}
	}

	return nil
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

func WriteSingleTicketToSheet(manager Manager, valueRange *sheets.ValueRange, firstRow int) int {
	ticketItems := len(valueRange.Values)

	for i, row := range valueRange.Values {
		totalFunction := fmt.Sprintf("=D%d*E%d", firstRow+i, firstRow+i)
		valueRange.Values[i] = append(row, totalFunction)
	}

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

// WriteAllTickets writes all entries in a single Update call (data) plus one
// BatchUpdate call for formatting — O(1) API calls instead of O(N) per-ticket.
func WriteAllTickets(manager Manager, entries []Entry) error {
	if len(entries) == 0 {
		return nil
	}

	// Add per-ticket rows with TOTAL formulas to a single [][]any slice.
	var finalValues [][]any
	for _, entry := range entries {
		vr, err := entry.ToValueRange()
		if err != nil {
			slog.Warn("Skipping entry with no items", "ID", entry.Id, "ERROR", err)
			continue
		}
		for _, row := range vr.Values {
			totalFunc := fmt.Sprintf("=D%d*E%d", len(finalValues)+HeaderSize+1, len(finalValues)+HeaderSize+1)
			finalValues = append(finalValues, append(row, totalFunc))
		}
		ticketItems := len(vr.Values)
		sumStart := len(finalValues) - ticketItems
		sumEnd := len(finalValues) - 1

		totalFunc := fmt.Sprintf("=SUM(F%d:F%d)", sumStart+HeaderSize+1, sumEnd+HeaderSize+1)
		finalValues = append(finalValues, []any{"", "", "", "", "TOTAL", totalFunc})
	}

	if len(finalValues) == 0 {
		return nil
	}
	var unmergeRequests []*sheets.Request
	unmergeRequests = append(unmergeRequests, &sheets.Request{
		UnmergeCells: &sheets.UnmergeCellsRequest{
			Range: &sheets.GridRange{
				SheetId:          manager.sheetId,
				StartRowIndex:    2,
				EndRowIndex:      200,
				StartColumnIndex: 0,
				EndColumnIndex:   2,
			}},
	})

	//unmerge all cells before inserting any data
	_, err := manager.service.BatchUpdate(manager.config.Sheets.MainID, &sheets.BatchUpdateSpreadsheetRequest{
		Requests: unmergeRequests})
	if err != nil {
		return fmt.Errorf("unmerge cells requests failed before loading new data: %s", err)
	}

	// Write all data in one call.
	firstRow := HeaderSize + 1 // row after header
	lastRow := firstRow + len(finalValues) - 1
	writeRange := fmt.Sprintf("%s!A%d:F%d", manager.PageName, firstRow, lastRow)
	valueRange := &sheets.ValueRange{MajorDimension: "ROWS", Values: finalValues}

	resp, err := manager.service.Update(manager.config.Sheets.MainID, writeRange, valueRange, "USER_ENTERED")
	if err != nil {
		slog.Error("Unable to batch-write data to sheet", "ERROR", err)
		return fmt.Errorf("batch write failed: %w", err)
	}
	slog.Info("Batch-written all entries to sheet", "CELL_COUNT", resp.UpdatedCells, "ENTRIES", len(entries))

	// Batch-format all ticket blocks in one call.
	if err := formatAllTicketBlocks(manager, entries); err != nil {
		slog.Warn("Unable to batch-format tickets", "ERROR", err)
	}

	return nil
}

// ticketFormatRequests builds the 6 formatting requests for a single ticket block.
// firstRow and totalRowIdx are 1-based row numbers; sheetId is the numeric sheet ID.
func ticketFormatRequests(firstRow, totalRowIdx int, sheetId int64) []*sheets.Request {
	startIdx := int64(firstRow)
	totalRowIdxInt64 := int64(totalRowIdx)
	endIdx := totalRowIdxInt64 + 1 // exclusive, includes TOTAL row

	blueColor := &sheets.Color{Red: 207.0 / 255.0, Green: 226.0 / 255.0, Blue: 243.0 / 255.0}

	fullRange := &sheets.GridRange{
		SheetId:          sheetId,
		StartRowIndex:    startIdx,
		EndRowIndex:      endIdx,
		StartColumnIndex: 0,
		EndColumnIndex:   6,
	}

	return []*sheets.Request{
		// Clear any existing merges in the ticket range
		{
			UnmergeCells: &sheets.UnmergeCellsRequest{Range: fullRange},
		},
		// Reset background, text format, and borders for all cells except TOTAL row
		{
			RepeatCell: &sheets.RepeatCellRequest{
				Range:  fullRange,
				Cell:   &sheets.CellData{},
				Fields: "userEnteredFormat(backgroundColorStyle,textFormat,borders)",
			},
		},
		// Blue background + bold for TOTAL row (columns A:F = indices 0:6)
		{
			RepeatCell: &sheets.RepeatCellRequest{
				Range: &sheets.GridRange{
					SheetId:          sheetId,
					StartRowIndex:    totalRowIdxInt64,
					EndRowIndex:      totalRowIdxInt64 + 1,
					StartColumnIndex: 0,
					EndColumnIndex:   6,
				},
				Cell: &sheets.CellData{
					UserEnteredFormat: &sheets.CellFormat{
						TextFormat:           &sheets.TextFormat{Bold: true},
						BackgroundColorStyle: &sheets.ColorStyle{RgbColor: blueColor},
					},
				},
				Fields: "userEnteredFormat(textFormat,backgroundColorStyle)",
			},
		},
		// Box border around the total VALUE cell (column F = index 5)
		{
			UpdateBorders: &sheets.UpdateBordersRequest{
				Range: &sheets.GridRange{
					SheetId:          sheetId,
					StartRowIndex:    totalRowIdxInt64,
					EndRowIndex:      totalRowIdxInt64 + 1,
					StartColumnIndex: 4,
					EndColumnIndex:   6,
				},
				Top:    &sheets.Border{Style: "SOLID"},
				Bottom: &sheets.Border{Style: "SOLID"},
				Left:   &sheets.Border{Style: "SOLID"},
				Right:  &sheets.Border{Style: "SOLID"},
			},
		},
		// Merge date cell vertically (column A = index 0)
		{
			MergeCells: &sheets.MergeCellsRequest{
				Range: &sheets.GridRange{
					SheetId:          sheetId,
					StartRowIndex:    startIdx,
					EndRowIndex:      totalRowIdxInt64,
					StartColumnIndex: 0,
					EndColumnIndex:   1,
				},
				MergeType: "MERGE_COLUMNS",
			},
		},
		// Merge store name cell vertically (column B = index 1)
		{
			MergeCells: &sheets.MergeCellsRequest{
				Range: &sheets.GridRange{
					SheetId:          sheetId,
					StartRowIndex:    startIdx,
					EndRowIndex:      totalRowIdxInt64,
					StartColumnIndex: 1,
					EndColumnIndex:   2,
				},
				MergeType: "MERGE_COLUMNS",
			},
		},
	}
}

// formatAllTicketBlocks builds formatting requests for every entry and sends
// them in a single BatchUpdate call.
func formatAllTicketBlocks(manager Manager, entries []Entry) error {
	if len(entries) == 0 {
		return nil
	}

	var requests []*sheets.Request
	for _, entry := range entries {
		totalRowIdx := entry.LastRow
		requests = append(requests, ticketFormatRequests(entry.FirstRow, totalRowIdx, manager.sheetId)...)
	}

	_, err := manager.service.BatchUpdate(manager.config.Sheets.MainID, &sheets.BatchUpdateSpreadsheetRequest{
		Requests: requests,
	})
	return err
}

func FormatTicketBlock(manager Manager, firstRow int, ticketItems int) error {
	totalRowIdx := firstRow + ticketItems - 1
	requests := ticketFormatRequests(firstRow, totalRowIdx, manager.sheetId)

	_, err := manager.service.BatchUpdate(manager.config.Sheets.MainID, &sheets.BatchUpdateSpreadsheetRequest{
		Requests: requests,
	})
	return err
}
