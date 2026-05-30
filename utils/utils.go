package utils

import (
	"log"
	"math"
	"strconv"
	"strings"
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
		log.Println("Unable to parse float from input string: ", str)
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
