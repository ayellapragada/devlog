package output

import (
	"context"
	"fmt"
	"io"

	"devlog/internal/storage"
)

type OutputFormat string

const (
	FormatTable  OutputFormat = "table"
	FormatJSON   OutputFormat = "json"
	FormatSimple OutputFormat = "simple"
)

type SearchPresenter struct {
	writer    io.Writer
	formatter ResultFormatter
}

func NewSearchPresenter(writer io.Writer, format OutputFormat) *SearchPresenter {
	var formatter ResultFormatter
	switch format {
	case FormatJSON:
		formatter = NewJSONFormatter()
	case FormatSimple:
		formatter = NewSimpleFormatter()
	default:
		formatter = NewTableFormatter()
	}

	return &SearchPresenter{
		writer:    writer,
		formatter: formatter,
	}
}

func NewSearchPresenterWithFormatter(writer io.Writer, formatter ResultFormatter) *SearchPresenter {
	return &SearchPresenter{
		writer:    writer,
		formatter: formatter,
	}
}

func (p *SearchPresenter) Present(ctx context.Context, results []*storage.SearchResult, query string) error {
	output, err := p.formatter.Format(ctx, results, query)
	if err != nil {
		return err
	}

	fmt.Fprint(p.writer, output)
	return nil
}
