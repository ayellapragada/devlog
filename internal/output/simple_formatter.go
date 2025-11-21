package output

import (
	"context"
	"fmt"
	"strings"

	"devlog/internal/storage"
)

type simpleFormatter struct{}

func NewSimpleFormatter() ResultFormatter {
	return simpleFormatter{}
}

func (simpleFormatter) Format(ctx context.Context, results []*storage.SearchResult, query string) (string, error) {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d events:\n\n", len(results)))

	for _, result := range results {
		sb.WriteString(FormatEventLine(result.Event, 200, 300, 300, 100))
		sb.WriteString("\n")
	}

	return sb.String(), nil
}
