package internal

import (
	"fmt"
	"time"

	"google.golang.org/api/sheets/v4"
)

type Item struct {
	Name   string
	Amount float64
	Price  float64
}

type Ticket struct {
	Items []Item
	Id    string
	Total float64
	Date  time.Time
}

type UploadableTicket interface {
	TicketToStr(store string) string
	ToValueRange(store string) *sheets.ValueRange
	GetTicketList()
}

func (ticket Ticket) ToValueRange(store string) *sheets.ValueRange {
	var values [][]any

	for _, item := range ticket.Items {
		row := []any{"", "", item.Name, item.Amount, item.Price}
		values = append(values, row)
	}
	values[0][0] = ticket.Date.Format("02/01/2006")
	values[0][1] = store

	valRange := sheets.ValueRange{MajorDimension: "ROWS", Values: values}

	return &valRange
}

func (ticket Ticket) TicketToStr(store string) string {
	return fmt.Sprintf("%s Ticket: %s, total: %f, items: %s", store, ticket.Id, ticket.Total, fmt.Sprint(ticket.Items))
}
