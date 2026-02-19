package pricing

import (
	"encoding/json"
	"fmt"
	"os"
)

type Store struct {
	prices map[string]float64
}

func LoadFromFile(path string) (*Store, error) {
	if path == "" {
		return nil, fmt.Errorf("PRICES_FILE is empty")
	}

	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read prices file: %w", err)
	}

	var m map[string]float64
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("parse prices json: %w", err)
	}

	for k, v := range m {
		if k == "" {
			return nil, fmt.Errorf("empty key in prices")
		}
		if v <= 0 {
			return nil, fmt.Errorf("invalid price for %q: %v", k, v)
		}
	}

	return &Store{prices: m}, nil
}

func Key(typ, itemID string) string {
	return typ + ":" + itemID
}

func (s *Store) Price(typ, itemID string) (float64, bool) {
	if s == nil {
		return 0, false
	}
	v, ok := s.prices[Key(typ, itemID)]
	return v, ok
}
