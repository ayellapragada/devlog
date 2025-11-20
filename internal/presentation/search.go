package presentation

import (
	"encoding/json"
	"fmt"
	"io"
	"unicode/utf8"

	"devlog/internal/events"
	"devlog/internal/storage"
)

type OutputFormat string

const (
	FormatTable OutputFormat = "table"
	FormatJSON  OutputFormat = "json"
)

type SearchPresenter struct {
	writer  io.Writer
	format  OutputFormat
	verbose bool
}

func NewSearchPresenter(writer io.Writer, format OutputFormat, verbose bool) *SearchPresenter {
	return &SearchPresenter{
		writer:  writer,
		format:  format,
		verbose: verbose,
	}
}

func (p *SearchPresenter) Present(results []*storage.SearchResult, query string) error {
	switch p.format {
	case FormatJSON:
		return p.presentJSON(results)
	default:
		return p.presentTable(results, query)
	}
}

func (p *SearchPresenter) presentTable(results []*storage.SearchResult, query string) error {
	if len(results) == 0 {
		fmt.Fprintf(p.writer, "No events found matching '%s'\n", query)
		return nil
	}

	fmt.Fprintf(p.writer, "Found %d event(s) matching '%s':\n\n", len(results), query)

	for _, result := range results {
		fmt.Fprintf(p.writer, "%s %s [%s:%s]\n",
			result.Event.Timestamp,
			result.Event.ID[:8],
			result.Event.Source,
			result.Event.Type,
		)

		if !p.verbose {
			content := p.extractContent(result.Event)
			if content != "" {
				fmt.Fprintf(p.writer, "  %s\n", content)
			}
		}

		if p.verbose {
			payloadJSON, _ := json.MarshalIndent(result.Event.Payload, "  ", "  ")
			fmt.Fprintf(p.writer, "  Payload:\n  %s\n", string(payloadJSON))
		}

		fmt.Fprintln(p.writer)
	}

	return nil
}

func (p *SearchPresenter) extractContent(event *events.Event) string {
	payload := event.Payload

	switch event.Type {
	case "commit":
		if msg, ok := payload["message"].(string); ok {
			return truncate(msg, 100)
		}
	case "command":
		if cmd, ok := payload["command"].(string); ok {
			return truncate(cmd, 100)
		}
	case "transcription":
		if text, ok := payload["text"].(string); ok {
			return truncate(text, 100)
		}
	case "conversation":
		if summary, ok := payload["summary"].(string); ok {
			return truncate(summary, 100)
		}
	case "copy":
		if content, ok := payload["content"].(string); ok {
			return truncate(content, 100)
		}
	case "push", "pull", "fetch":
		if ref, ok := payload["ref"].(string); ok {
			return "ref: " + ref
		}
	case "checkout":
		if branch, ok := payload["branch"].(string); ok {
			return "→ " + branch
		}
	case "merge":
		if source, ok := payload["source"].(string); ok {
			if target, ok := payload["target"].(string); ok {
				return source + " → " + target
			}
			return source
		}
	case "note":
		if note, ok := payload["note"].(string); ok {
			return truncate(note, 100)
		}
	case "file_edit":
		if file, ok := payload["file"].(string); ok {
			return file
		}
	}

	return ""
}

func truncate(s string, maxLen int) string {
	if s == "" || maxLen <= 0 {
		return ""
	}
	if maxLen <= 3 {
		return "..."
	}

	runeCount := utf8.RuneCountInString(s)
	if runeCount <= maxLen {
		return s
	}

	truncated := s
	runes := []rune(s)
	if len(runes) > maxLen-3 {
		truncated = string(runes[:maxLen-3])
	}
	return truncated + "..."
}

func (p *SearchPresenter) presentJSON(results []*storage.SearchResult) error {
	type output struct {
		Results []*storage.SearchResult `json:"results"`
		Count   int                     `json:"count"`
	}

	data := output{
		Results: results,
		Count:   len(results),
	}

	encoder := json.NewEncoder(p.writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}
