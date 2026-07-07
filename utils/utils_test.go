package utils

import (
	"fmt"
	"math"
	"testing"
	"time"
)

// TestFloatsEqual verifies that two floats are considered equal within epsilon
func TestFloatsEqual(t *testing.T) {
	tests := []struct {
		a      float64
		b      float64
		expect bool
	}{
		{0.0, 0.0, true},
		{1.5, 1.5, true},
		{1e-6, 0.0, true},  // Within epsilon (1e-5)
		{1e-4, 0.0, false}, // Outside epsilon
		{math.MaxFloat64, math.MaxFloat64 - 1e-5, true}, // Precision boundary
		{-1.5, -1.5, true},
		{-1.5, -1.5 + 1e-4, false},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("%v-%v", tc.a, tc.b), func(t *testing.T) {
			result := FloatsEqual(tc.a, tc.b)
			if result != tc.expect {
				t.Errorf("FloatsEqual(%v, %v) = %v; expected %v", tc.a, tc.b, result, tc.expect)
			}
		})
	}
}

// TestRound verifies integer rounding behavior for positive and negative numbers
func TestRound(t *testing.T) {
	tests := []struct {
		input  float64
		expect int
	}{
		{0.0, 0},
		{0.4, 0}, // Rounds down
		{0.5, 1}, // Rounds up (half up)
		{1.49, 1},
		{1.5, 2},
		{-0.4, 0},
		{-0.5, -1},
		{-1.5, -2}, // Negative half rounds down (away from zero)
		{10.0, 10},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("%v->%d", tc.input, tc.expect), func(t *testing.T) {
			result := round(tc.input)
			if result != tc.expect {
				t.Errorf("round(%v) = %d; expected %d", tc.input, result, tc.expect)
			}
		})
	}
}

// TestToFixed verifies rounding to specific decimal places
func TestToFixed(t *testing.T) {
	tests := []struct {
		num       float64
		precision int
		expected  float64
	}{
		{1.234, 2, 1.23},
		{1.235, 2, 1.24},
		{1.239, 2, 1.24},
		{-1.234, 2, -1.23},
		{-1.235, 2, -1.24},
		{1.0, 0, 1.0},
		{1.000, 3, 1.0},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("%v,p=%d", tc.num, tc.precision), func(t *testing.T) {
			result := ToFixed(tc.num, tc.precision)
			if !FloatsEqual(result, tc.expected) {
				t.Errorf("toFixed(%v, %d) = %v; expected %v", tc.num, tc.precision, result, tc.expected)
			}
		})
	}
}

// TestStringToFloat verifies string parsing and formatting
func TestStringToFloat(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
		isError  bool // Indicates if we expect a valid number or just the check that it runs
	}{
		{"10.5", 10.5, false},
		{"  10.5  ", 10.5, false}, // Whitespace trimming
		{"10,5", 10.5, false},     // Comma to dot conversion
		{"abc", 0.0, true},        // Invalid string returns 0
		{"", 0.0, true},           // Empty string returns 0
		{"12.345", 12.35, false},  // Note: StringToFloat calls ToFixed(2) internally, so 3rd decimal is rounded
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := StringToFloat(tc.input, 2)
			if result != tc.expected {
				t.Errorf("StringToFloat(%q) = %v; expected %v", tc.input, result, tc.expected)
			}
		})
	}
}

// TestParseQty verifies quantity parsing with and without units
func TestParseQty(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
	}{
		{"5", 5.0},
		{"5ud", 5.0},
		{"5 kg", 5.0},
		{"10kg", 10.0},
		{"abc", 0.0},   // Invalid number content
		{"-5kg", -5.0}, // Negative quantities (if supported by parser)
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := ParseQty(tc.input)
			if !FloatsEqual(result, tc.expected) {
				t.Errorf("ParseQty(%q) = %v; expected %v", tc.input, result, tc.expected)
			}
		})
	}
}

// TestParseQty verifies quantity parsing with and without units
func TestParseQtyWithPrecision(t *testing.T) {
	tests := []struct {
		input     string
		precision int
		expected  float64
	}{
		{"5.1234", 3, 5.123},
		{"5.123ud", 3, 5.123},
		{"5.123ud.", 3, 5.123},
		{"5.123UD.", 3, 5.123},
		{"5.123Ud.", 3, 5.123},
		{"5.123 uD.", 3, 5.123},
		{"0.5565 kg", 3, 0.557},
		{"10kg", 1, 10.0},
		{"abc", 2, 0.0},              // Invalid number content
		{"-5kg", 2, -5.0},            // Negative quantities (if supported by parser)
		{"1.23456789", 6, 1.2345679}, //High precision
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := ParseQtyWithPrecision(tc.input, tc.precision)
			if !FloatsEqual(result, tc.expected) {
				t.Errorf("ParseQty(%q) = %v; expected %v", tc.input, result, tc.expected)
			}
		})
	}
}

// TestParsePrice verifies price parsing with currency and units
func TestParsePrice(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
	}{
		{"10.50", 10.5},
		{"€10.50", 10.5},
		{"10€/kg", 10},
		{"€10.50/kg", 10.5},
		{"10.50€/kg", 10.5},
		{"10.50/kg", 10.5},
		{"10.50 € /kg", 10.5},
		{"abc/kg", 0.0}, // Invalid price
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := ParsePrice(tc.input)
			if !FloatsEqual(result, tc.expected) {
				t.Errorf("ParsePrice(%q) = %v; expected %v", tc.input, result, tc.expected)
			}
		})
	}
}

func TestExtractString(t *testing.T) {
	type Point struct{ X, Y int }

	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{"string value", "hello", "hello"},
		{"empty string", "", ""},
		{"nil", nil, ""},
		{"float64 integer", float64(42), "42"},
		{"float64 decimal", float64(3.14), "3.14"},
		{"float64 large", float64(1e9), "1e+09"},
		{"float64 zero", float64(0), "0"},
		{"float64 negative", float64(-7.5), "-7.5"},
		{"int default", int(99), "99"},
		{"bool default", true, "true"},
		{"struct default", Point{1, 2}, "{1 2}"},
		{"slice default", []int{1, 2, 3}, "[1 2 3]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExtractString(tt.input); got != tt.expected {
				t.Errorf("ExtractString(%v) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestStringToDate(t *testing.T) {
	d := func(day, month, year int) time.Time {
		return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	}

	tests := []struct {
		name     string
		input    string
		expected time.Time
	}{
		{"standard format", "15/04/2023", d(15, 4, 2023)},
		{"compressed month", "05/3/2022", d(5, 3, 2022)},
		{"first day of year", "01/01/2000", d(1, 1, 2000)},
		{"last day of year", "31/12/2023", d(31, 12, 2023)},
		{"leap day", "29/02/2024", d(29, 2, 2024)},
		{"invalid string", "not-a-date", time.Time{}},
		{"empty string", "", time.Time{}},
		{"wrong separator", "15-04-2023", time.Time{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StringToDate(tt.input)
			if !got.Equal(tt.expected) {
				t.Errorf("StringToDate(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}
