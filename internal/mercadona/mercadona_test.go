package mercadona

import (
	"testing"
	"time"

	"google.golang.org/api/gmail/v1"
)

func Test_isTotalCorrect(t *testing.T) {
	type args struct {
		qty        float64
		pricePer   float64
		totalFound float64
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "exact match single unit",
			args: args{qty: 1, pricePer: 2.50, totalFound: 2.50},
			want: true,
		},
		{
			name: "exact match multiple units",
			args: args{qty: 3, pricePer: 1.20, totalFound: 3.60},
			want: true,
		},
		{
			name: "floating point that rounds correctly",
			args: args{qty: 2, pricePer: 0.55, totalFound: 1.10},
			want: true,
		},
		{
			name: "mismatch",
			args: args{qty: 2, pricePer: 1.50, totalFound: 4.00},
			want: false,
		},
		{
			name: "rounding edge case - rounds to match",
			args: args{qty: 3, pricePer: 0.333, totalFound: 1.00},
			want: true,
		},
		{
			name: "zero quantity",
			args: args{qty: 0, pricePer: 5.00, totalFound: 0.00},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isTotalCorrect(tt.args.qty, tt.args.pricePer, tt.args.totalFound); got != tt.want {
				t.Errorf("isTotalCorrect() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getTicketId(t *testing.T) {
	type args struct {
		message *gmail.Message
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "extracts last word from snippet",
			args: args{
				message: &gmail.Message{Snippet: "Tu compra en Mercadona - Ticket 123456789"},
			},
			want: "123456789",
		},
		{
			name: "single word snippet",
			args: args{
				message: &gmail.Message{Snippet: "987654321"},
			},
			want: "987654321",
		},
		{
			name: "snippet with extra spaces between words",
			args: args{
				message: &gmail.Message{Snippet: "Ref ticket ABC-001"},
			},
			want: "ABC-001",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getTicketId(tt.args.message); got != tt.want {
				t.Errorf("getTicketId() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getDateFromTicket(t *testing.T) {
	type args struct {
		message *gmail.Message
	}
	tests := []struct {
		name    string
		args    args
		want    time.Time
		wantErr bool
	}{
		{
			name: "parses date from filename",
			args: args{
				message: &gmail.Message{
					Payload: &gmail.MessagePart{
						Parts: []*gmail.MessagePart{
							{},
							{Filename: "20240115 Mercadona 45.30€.pdf"},
						},
					},
				},
			},
			want:    time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			wantErr: false,
		},
		{
			name: "parses date - end of year",
			args: args{
				message: &gmail.Message{
					Payload: &gmail.MessagePart{
						Parts: []*gmail.MessagePart{
							{},
							{Filename: "20231231 Mercadona 12.00.pdf"},
						},
					},
				},
			},
			want:    time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC),
			wantErr: false,
		},
		{
			name: "invalid date string returns error",
			args: args{
				message: &gmail.Message{
					Payload: &gmail.MessagePart{
						Parts: []*gmail.MessagePart{
							{},
							{Filename: "INVALID Mercadona 12.00.pdf"},
						},
					},
				},
			},
			want:    time.Time{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getDateFromTicket(tt.args.message)
			if (err != nil) != tt.wantErr {
				t.Errorf("getDateFromTicket() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !got.Equal(tt.want) {
				t.Errorf("getDateFromTicket() = %v, want %v", got, tt.want)
			}
		})
	}
}