package output

import (
	"context"
	"fmt"
	"strings"

	"devlog/internal/storage"
)

type tableFormatter struct{}

func NewTableFormatter() ResultFormatter {
	return tableFormatter{}
}

func (tableFormatter) Format(ctx context.Context, results []*storage.SearchResult, query string) (string, error) {
	var sb strings.Builder

	if len(results) == 0 {
		return fmt.Sprintf("No events found matching '%s'\n", query), nil
	}

	sb.WriteString(fmt.Sprintf("Found %d event(s) matching '%s':\n\n", len(results), query))

	for _, result := range results {
		sb.WriteString(fmt.Sprintf("%s %s [%s:%s]\n",
			result.Event.Timestamp,
			result.Event.ID[:8],
			result.Event.Source,
			result.Event.Type,
		))

		content := ExtractContent(result.Event, 100)
		if content != "" {
			sb.WriteString(fmt.Sprintf("  %s\n", content))
		}

		sb.WriteString("\n")
	}

	return sb.String(), nil
}
