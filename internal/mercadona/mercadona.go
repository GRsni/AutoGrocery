package mercadona

import (
	"autoGrocery/internal"
	"autoGrocery/internal/google/gm"
	"autoGrocery/pkg/constants"
	"autoGrocery/utils"
	"bytes"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/ledongthuc/pdf"
	"google.golang.org/api/gmail/v1"
)

const GmailLabelId = "Label_2031551581397134603"
const TicketHeaderRows = 7

func GetTicketList(manager gm.Manager, lastFound time.Time) []internal.Ticket {
	tickets := make([]internal.Ticket, 0)

	messages := gm.GetMessagesFromLabel(manager, GmailLabelId)

	for _, m := range messages {
		dateFromTicket, err := getDateFromTicket(m)
		if err != nil {
			slog.Warn("Unable to get date from ticket filename, skipping", "ERROR", err)
			continue
		}
		if dateFromTicket.After(lastFound) {
			ticket := getTicketDetails(manager, m, dateFromTicket)
			tickets = append(tickets, ticket)
			slog.Info(ticket.TicketToStr())
		}
	}
	return tickets
}
func getDateFromTicket(message *gmail.Message) (time.Time, error) {
	filenameParts := strings.Split(message.Payload.Parts[1].Filename, " ")
	date, err := time.Parse("20060102", filenameParts[0])
	if err != nil {
		return time.Now(), fmt.Errorf("unable to parse date from filename: %v", err)
	}
	return date, nil
}

func getTicketDetails(manager gm.Manager, message *gmail.Message, ticketDate time.Time) internal.Ticket {
	filenameParts := strings.Split(message.Payload.Parts[1].Filename, " ")
	id := getTicketId(message)

	ticketTotal := utils.StringToFloat(filenameParts[2], 2)
	items := getTicketItems(manager, message)

	if !items.IsTotalValid(ticketTotal) {
		slog.Warn("Ticket price does not match up, discarding")
		return internal.Ticket{}
	}
	return internal.Ticket{
		Items: items,
		Id:    id,
		Total: ticketTotal,
		Store: constants.MERCADONA,
		Date:  ticketDate,
	}
}

func getTicketId(message *gmail.Message) string {
	snippetParts := strings.Split(message.Snippet, " ")
	return snippetParts[len(snippetParts)-1]
}

func getTicketItems(manager gm.Manager, m *gmail.Message) internal.Items {
	items := make([]internal.Item, 0, 1)
	fileBytes := gm.GetAttachmentFromMessage(manager, m.Id, m.Payload.Parts[1].Body.AttachmentId)
	r, err := pdf.NewReader(bytes.NewReader(fileBytes), int64(len(fileBytes)))
	if err != nil {
		slog.Error("Unable to parse data as PDF file", "ERROR", err)
		return items
	}

	for i := 1; i <= r.NumPage(); i++ {
		page := r.Page(i)
		if page.V.IsNull() {
			continue
		}
		rows, err := page.GetTextByRow()
		if err != nil {
			continue
		}
		for rowIndex := TicketHeaderRows; rowIndex <= len(rows); rowIndex++ {
			row := rows[rowIndex]
			if row.Content[1].S == "PARKING" {
				slog.Debug("Parking item found, skipping")
				rowIndex++
				continue
			}
			if len(row.Content) < 2 {
				slog.Warn("Error while reading item line")
				continue
			}
			if row.Content[0].S == "TOTAL (€)" {
				slog.Debug("End of ticket reached, breaking out")
				break
			}

			itemName := row.Content[1].S
			var price = 0.0
			var qty = 0.0
			var total = 0.0
			if len(row.Content) == 2 {
				rowIndex++
				nextRow := rows[rowIndex]
				qty = utils.ParseQtyWithPrecision(nextRow.Content[0].S, 3)
				price = utils.ParsePrice(nextRow.Content[1].S)
				total = utils.StringToFloat(nextRow.Content[2].S, 2)
			} else {
				qty = utils.ParseQty(row.Content[0].S)
				price = utils.ParsePrice(row.Content[2].S)
				if qty > 1 {
					total = utils.StringToFloat(row.Content[3].S, 3)
				} else {
					total = price
				}
			}
			totalCorrect := isTotalCorrect(qty, price, total)
			if !totalCorrect {
				slog.Debug("Item total not correct, discarding item")
				continue
			}
			items = append(items, internal.Item{Name: itemName, Amount: qty, Price: price})
		}
	}
	return items
}

func isTotalCorrect(qty float64, pricePer float64, totalFound float64) bool {
	return utils.FloatsEqual(utils.ToFixed(qty*pricePer, 2), totalFound)
}
