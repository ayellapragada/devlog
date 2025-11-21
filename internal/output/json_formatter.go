package output

import (
	"bytes"
	"context"
	"encoding/json"

	"devlog/internal/storage"
)

type jsonFormatter struct{}

func NewJSONFormatter() ResultFormatter {
	return jsonFormatter{}
}

func (jsonFormatter) Format(ctx context.Context, results []*storage.SearchResult, query string) (string, error) {
	type output struct {
		Results []*storage.SearchResult `json:"results"`
		Count   int                     `json:"count"`
	}

	data := output{
		Results: results,
		Count:   len(results),
	}

	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		return "", err
	}

	return buf.String(), nil
}
