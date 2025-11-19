package presentation

import (
	"encoding/json"
	"fmt"
	"io"

	"devlog/internal/storage"
)

type OutputFormat string

const (
	FormatTable OutputFormat = "table"
	FormatJSON  OutputFormat = "json"
	FormatCSV   OutputFormat = "csv"
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
	case FormatCSV:
		return p.presentCSV(results)
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

		if result.Snippet != "" {
			fmt.Fprintf(p.writer, "  Snippet: %s\n", result.Snippet)
		}

		if p.verbose {
			payloadJSON, _ := json.MarshalIndent(result.Event.Payload, "  ", "  ")
			fmt.Fprintf(p.writer, "  Payload:\n  %s\n", string(payloadJSON))
		}

		fmt.Fprintln(p.writer)
	}

	return nil
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

func (p *SearchPresenter) presentCSV(results []*storage.SearchResult) error {
	fmt.Fprintln(p.writer, "timestamp,id,source,type,repo,branch,snippet")

	for _, result := range results {
		snippet := ""
		if result.Snippet != "" {
			snippet = fmt.Sprintf("%q", result.Snippet)
		}

		fmt.Fprintf(p.writer, "%s,%s,%s,%s,%s,%s,%s\n",
			result.Event.Timestamp,
			result.Event.ID,
			result.Event.Source,
			result.Event.Type,
			result.Event.Repo,
			result.Event.Branch,
			snippet,
		)
	}

	return nil
}
