package internal

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
)

type Excluded struct {
	DiaIDs       []string `json:"dia"`
	CarrefourIds []string `json:"carrefour"`
	MercadonaIds    []string `json:"mercadona"`
}

func LoadExcluded(excludedPath string) (Excluded, error) {
	b, err := os.ReadFile(excludedPath)
	if err != nil {
		return Excluded{}, fmt.Errorf("unable to read excluded Ids file: %w", err)
	}
	var excluded Excluded
	if err := json.Unmarshal(b, &excluded); err != nil {
		slog.Info("Failed to unmarshal excluded ids", "ERROR", err)
		return Excluded{}, err
	}

	return excluded, nil
}
