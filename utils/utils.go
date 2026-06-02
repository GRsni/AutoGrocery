package utils

import (
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"time"
)

const epsilon = 1e-5

func FloatsEqual(a, b float64) bool {
	return math.Abs(a-b) < epsilon
}

func round(num float64) int {
	return int(num + math.Copysign(0.5, num))
}

func ToFixed(num float64, precision int) float64 {
	output := math.Pow(10, float64(precision))
	return float64(round(num*output)) / output
}

func StringToFloat(str string) float64 {
	parsed, err := strconv.ParseFloat(strings.TrimSpace(strings.Replace(str, ",", ".", -1)), 32)
	if err != nil {
		slog.Debug("Unable to parse float from input string: " + str)
		return 0
	}
	return ToFixed(parsed, 2)
}

func ParseQty(qty string) float64 {
	strippedQty := strings.Replace(qty, "ud", "", -1)
	strippedQty = strings.Replace(strippedQty, "kg", "", -1)
	return StringToFloat(strippedQty)
}

func ParsePrice(price string) float64 {
	strippedPrice := strings.Replace(price, "€", "", -1)
	strippedPrice = strings.Replace(strippedPrice, "/kg", "", -1)
	return StringToFloat(strippedPrice)
}

func ExtractString(val any) string {
	switch v := val.(type) {
	case string:
		return v
	case float64: // Google Sheets returns numbers as float64
		return fmt.Sprintf("%g", v)
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", v)
	}
}

func GetDateFromCell(dateCell string) time.Time {
	date, err := time.Parse("02/01/2006", dateCell)
	if err != nil {
		slog.Info("Failed to parse date cell with default format, trying compressed format", "ERROR", err)
		date, _ = time.Parse("02/1/2006", dateCell)
	}
	return date
}
