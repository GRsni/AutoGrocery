package constants

import (
	"testing"
	"time"
)

func TestFromTimeMonth_AllMonths(t *testing.T) {
	tests := []struct {
		month  time.Month
		expect SpanishMonth
	}{
		{time.January, Enero},
		{time.February, Febrero},
		{time.March, Marzo},
		{time.April, Abril},
		{time.May, Mayo},
		{time.June, Junio},
		{time.July, Julio},
		{time.August, Agosto},
		{time.September, Septiembre},
		{time.October, Octubre},
		{time.November, Noviembre},
		{time.December, Diciembre},
	}

	for _, tt := range tests {
		t.Run(tt.month.String(), func(t *testing.T) {
			got := FromTimeMonth(tt.month)
			if got != tt.expect {
				t.Errorf("FromTimeMonth(%d) = %q, want %q", tt.month, got, tt.expect)
			}
		})
	}
}

func TestFromTimeMonth_InvalidMonth(t *testing.T) {
	// Month 0 is invalid
	if got := FromTimeMonth(0); got != "" {
		t.Errorf("FromTimeMonth(0) = %q, want empty string", got)
	}
	// Month 13 is invalid
	if got := FromTimeMonth(13); got != "" {
		t.Errorf("FromTimeMonth(13) = %q, want empty string", got)
	}
}

func TestFromTimeMonth_ZeroValue(t *testing.T) {
	var m time.Month // zero value is 0
	if got := FromTimeMonth(m); got != "" {
		t.Errorf("FromTimeMonth(0) = %q, want empty string", got)
	}
}
