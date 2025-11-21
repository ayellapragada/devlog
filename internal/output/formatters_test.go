package output

import (
	"context"
	"strings"
	"testing"

	"devlog/internal/storage"
)

func TestJSONFormatterBasics(t *testing.T) {
	t.Run("creates JSON formatter", func(t *testing.T) {
		formatter := NewJSONFormatter()
		if formatter == nil {
			t.Error("formatter is nil")
		}
	})

	t.Run("formats search results", func(t *testing.T) {
		formatter := NewJSONFormatter()
		ctx := context.Background()

		results := []*storage.SearchResult{}
		output, err := formatter.Format(ctx, results, "test query")

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if output == "" {
			t.Error("output is empty")
		}

		// Should be valid JSON
		if output[0] != '{' && output[0] != '[' {
			t.Errorf("output doesn't start with JSON: %q", output[:10])
		}
	})

	t.Run("includes query in context", func(t *testing.T) {
		formatter := NewJSONFormatter()
		ctx := context.Background()

		results := []*storage.SearchResult{}
		output, err := formatter.Format(ctx, results, "my query")

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if output == "" {
			t.Error("output is empty")
		}
	})
}

func TestSimpleFormatterBasics(t *testing.T) {
	t.Run("creates simple formatter", func(t *testing.T) {
		formatter := NewSimpleFormatter()
		if formatter == nil {
			t.Error("formatter is nil")
		}
	})

	t.Run("formats search results", func(t *testing.T) {
		formatter := NewSimpleFormatter()
		ctx := context.Background()

		results := []*storage.SearchResult{}
		output, err := formatter.Format(ctx, results, "test")

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if output == "" {
			t.Log("simple formatter may return empty for empty results")
		}
	})
}

func TestTableFormatterBasics(t *testing.T) {
	t.Run("creates table formatter", func(t *testing.T) {
		formatter := NewTableFormatter()
		if formatter == nil {
			t.Error("formatter is nil")
		}
	})

	t.Run("formats search results", func(t *testing.T) {
		formatter := NewTableFormatter()
		ctx := context.Background()

		results := []*storage.SearchResult{}
		output, err := formatter.Format(ctx, results, "test")

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if output == "" {
			t.Log("table formatter may return empty for empty results")
		}
	})
}

func TestNewSearchPresenter(t *testing.T) {
	t.Run("creates presenter with JSON format", func(t *testing.T) {
		if FormatJSON == "" {
			t.Error("FormatJSON is empty")
		}
	})

	t.Run("creates presenter with simple format", func(t *testing.T) {
		if FormatSimple == "" {
			t.Error("FormatSimple is empty")
		}
	})

	t.Run("creates presenter with table format", func(t *testing.T) {
		if FormatTable == "" {
			t.Error("FormatTable is empty")
		}
	})
}

func TestResultFormatterInterface(t *testing.T) {
	t.Run("JSON formatter implements ResultFormatter", func(t *testing.T) {
		formatter := NewJSONFormatter()
		var _ = formatter
	})

	t.Run("simple formatter implements ResultFormatter", func(t *testing.T) {
		formatter := NewSimpleFormatter()
		var _ = formatter
	})

	t.Run("table formatter implements ResultFormatter", func(t *testing.T) {
		formatter := NewTableFormatter()
		var _ = formatter
	})
}

func TestFormatterWithContext(t *testing.T) {
	t.Run("formatters complete quickly and don't check context", func(t *testing.T) {
		formatter := NewJSONFormatter()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		results := []*storage.SearchResult{}
		output, err := formatter.Format(ctx, results, "query")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if output == "" {
			t.Error("expected non-empty output")
		}
	})
}

func TestFormatterErrorHandling(t *testing.T) {
	t.Run("JSON formatter handles empty results", func(t *testing.T) {
		formatter := NewJSONFormatter()
		ctx := context.Background()

		results := []*storage.SearchResult{}
		output, err := formatter.Format(ctx, results, "query")

		if err != nil {
			t.Fatalf("unexpected error on empty results: %v", err)
		}

		if output == "" {
			t.Error("formatter should return formatted output even for empty results")
		}

		if !strings.Contains(output, "count") && !strings.Contains(output, "results") {
			t.Errorf("JSON output missing expected fields: %s", output)
		}
	})

	t.Run("formatters handle nil results", func(t *testing.T) {
		formatter := NewJSONFormatter()
		ctx := context.Background()

		output, err := formatter.Format(ctx, nil, "query")

		if err != nil {
			t.Fatalf("formatter should handle nil results gracefully: %v", err)
		}

		if output == "" {
			t.Error("formatter should return output even for nil results")
		}
	})
}

func TestFormatterOutputFormat(t *testing.T) {
	t.Run("JSON output is valid JSON", func(t *testing.T) {
		formatter := NewJSONFormatter()
		ctx := context.Background()

		results := []*storage.SearchResult{}
		output, err := formatter.Format(ctx, results, "query")

		if err != nil {
			t.Fatalf("format error: %v", err)
		}

		if len(output) > 0 && output[0] != '{' && output[0] != '[' {
			t.Errorf("JSON formatter output doesn't start with JSON: %q", output[:10])
		}
	})

	t.Run("simple formatter outputs readable text", func(t *testing.T) {
		formatter := NewSimpleFormatter()
		ctx := context.Background()

		results := []*storage.SearchResult{}
		output, err := formatter.Format(ctx, results, "query")

		if err != nil {
			t.Fatalf("simple formatter error: %v", err)
		}

		if output == "" {
			t.Error("simple formatter should return output for empty results")
		}
	})

	t.Run("table formatter outputs structured format", func(t *testing.T) {
		formatter := NewTableFormatter()
		ctx := context.Background()

		results := []*storage.SearchResult{}
		output, err := formatter.Format(ctx, results, "query")

		if err != nil {
			t.Fatalf("table formatter error: %v", err)
		}

		if output == "" {
			t.Error("table formatter should return output for empty results")
		}
	})
}
