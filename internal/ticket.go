package internal

import (
	"autoGrocery/internal/google/sh"
	"autoGrocery/pkg/constants"
	"autoGrocery/utils"
	"fmt"
	"slices"
	"strings"
	"time"

	"google.golang.org/api/sheets/v4"
)

type Item struct {
	Name      string
	Amount    float64
	Price     float64
	Discount  float64
	Cancelled bool
}

type Items []Item

type Ticket struct {
	Items Items
	Id    string
	Total float64
	Store string
	Date  time.Time
}

type UploadableTicket interface {
	TicketToStr() string
	ToValueRange() *sheets.ValueRange
	IsExcluded([]string) bool
}

type ValidatedItems interface {
	IsTotalValid() bool
	IsTotalValidWithPrecision(precision int) bool
}

type CastableItem interface {
	ToEntryItem() sh.Item
}

type CastableTicket interface {
	ToEntry() sh.Entry
}

func (ticket Ticket) ToValueRange() (*sheets.ValueRange, error) {
	var values [][]any

	for _, item := range ticket.Items {
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
	values[0][0] = ticket.Date.Format(constants.TicketDateFormat)
	values[0][1] = ticket.Store + "\n" + ticket.Id

	valRange := sheets.ValueRange{MajorDimension: "ROWS", Values: values}

	return &valRange, nil
}

func (ticket Ticket) TicketToStr() string {
	return fmt.Sprintf("%s Ticket: %s, total: €%.2f, items: %s", ticket.Store, ticket.Id, ticket.Total, ticket.Items.String())
}

func (items Items) String() string {
	if len(items) == 0 {
		return "[]"
	}

	var builder strings.Builder
	builder.WriteString("[")
	for i, item := range items {
		if i > 0 {
			builder.WriteString(", ")
		}
		amountStr := fmt.Sprintf("%.3f", item.Amount)
		fmt.Fprintf(&builder, "%s (x%s @ €%.3f)", item.Name, amountStr, item.Price)
	}
	builder.WriteString("]")
	return builder.String()
}

func (ticket Ticket) IsExcluded(ids []string) bool {
	return slices.Contains(ids, ticket.Id)
}

func (items Items) IsTotalValid(total float64) bool {
	itemsTotal := 0.0

	for _, item := range items {
		itemsTotal += utils.ToFixed(item.Amount*item.Price, 2)
	}

	return utils.FloatsEqual(total, utils.ToFixed(itemsTotal, 2))
}

func (items Items) IsTotalValidWithPrecision(total float64, precision float64) bool {
	itemsTotal := 0.0

	for _, item := range items {
		itemsTotal += utils.ToFixed(item.Amount*item.Price, 2)
	}

	return utils.FloatsEqualWithDelta(total, utils.ToFixed(itemsTotal, 2), precision)
}

func (items Items) ToEntryItem() sh.Items {
	entryItems := make(sh.Items, 0)

	for _, ticketItem := range items {
		entryItems = append(entryItems, sh.Item{
			Name:   ticketItem.Name,
			Amount: ticketItem.Amount,
			Price:  ticketItem.Price})
	}

	return entryItems
}

func (ticket Ticket) ToEntry(firstRow int) sh.Entry {
	return sh.Entry{
		Date:     ticket.Date,
		Id:       ticket.Id,
		Store:    ticket.Store,
		FirstRow: firstRow,
		LastRow:  firstRow + len(ticket.Items),
		Total:    ticket.Total,
		Items:    ticket.Items.ToEntryItem()}
}
