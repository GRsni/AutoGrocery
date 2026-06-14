package internal

import (
	"autoGrocery/pkg/constants"
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
	Store string
	Date  time.Time
}

type UploadableTicket interface {
	TicketToStr() string
	ToValueRange() *sheets.ValueRange
}

func (ticket Ticket) ToValueRange() (*sheets.ValueRange, error) {
	var values [][]any

	for _, item := range ticket.Items {
		if item.Name[0] == '+' {
			item.Name = "'" + item.Name
		}
		row := []any{"", "", item.Name, item.Amount, item.Price}
		values = append(values, row)
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("ticket contains no items to upload")
	}
	values[0][0] = ticket.Date.Format(constants.TicketDateFormat)
	values[0][1] = ticket.Store

	valRange := sheets.ValueRange{MajorDimension: "ROWS", Values: values}

	return &valRange, nil
}

func (ticket Ticket) TicketToStr() string {
	return fmt.Sprintf("%s Ticket: %s, total: %f€, items: %s", ticket.Store, ticket.Id, ticket.Total, fmt.Sprint(ticket.Items))
}
