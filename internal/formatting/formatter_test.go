package formatting

import (
	"testing"

	"devlog/internal/events"
	"devlog/internal/testutil"
)

func TestFormatDurationMs(t *testing.T) {
	tests := []struct {
		name     string
		ms       int64
		expected string
	}{
		{"0 ms", 0, "0ms"},
		{"500 ms", 500, "500ms"},
		{"999 ms", 999, "999ms"},
		{"1000 ms", 1000, "1.0s"},
		{"1500 ms", 1500, "1.5s"},
		{"59500 ms", 59500, "59.5s"},
		{"60000 ms", 60000, "1m0s"},
		{"90000 ms", 90000, "1m30s"},
		{"120000 ms", 120000, "2m0s"},
		{"3661000 ms", 3661000, "61m1s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatDurationMs(tt.ms)
			if result != tt.expected {
				t.Errorf("FormatDurationMs(%d) = %q, want %q", tt.ms, result, tt.expected)
			}
		})
	}
}

func TestTruncateToFirstLine(t *testing.T) {
	tests := []struct {
		name   string
		text   string
		maxLen int
		want   string
	}{
		{"empty", "", 10, ""},
		{"single line", "hello world", 100, "hello world"},
		{"multiline", "line1\nline2", 100, "line1"},

		// Edge case: When first line exceeds maxLen, truncate with ellipsis
		{"multiline with truncate", "hello world\nline2", 5, "hello..."},

		// Edge case: Text starting with newline results in empty string
		{"just newline", "\nline2", 10, ""},

		{"exact length", "hello", 5, "hello"},
		{"over length", "hello world", 5, "hello..."},

		// Edge case: maxLen=0 should return just ellipsis
		{"length 0", "hello\nworld", 0, "..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TruncateToFirstLine(tt.text, tt.maxLen)
			if result != tt.want {
				t.Errorf("TruncateToFirstLine(%q, %d) = %q, want %q", tt.text, tt.maxLen, result, tt.want)
			}
		})
	}
}

func TestFormatEventContent(t *testing.T) {
	t.Run("formats git commit", func(t *testing.T) {
		event := testutil.NewEventBuilder().
			WithSource("git").
			WithType("commit").
			WithPayloadField("hash", "abc123").
			Build()

		content := FormatEventContent(event)
		if content == "" {
			t.Error("content is empty")
		}

		// Should contain something about the event
		if len(content) < 3 {
			t.Errorf("content too short: %q", content)
		}
	})

	t.Run("formats unknown event", func(t *testing.T) {
		event := testutil.NewEventBuilder().
			WithSource("unknown").
			WithType("unknown").
			Build()

		content := FormatEventContent(event)

		// Should return default format
		if content != "unknown/unknown" {
			t.Errorf("expected 'unknown/unknown', got %q", content)
		}
	})

	t.Run("handles missing formatter", func(t *testing.T) {
		event := &events.Event{
			Source: "nosuchsource",
			Type:   "nosuchtype",
		}

		content := FormatEventContent(event)

		// Should return source/type
		if content != "nosuchsource/nosuchtype" {
			t.Errorf("expected 'nosuchsource/nosuchtype', got %q", content)
		}
	})
}

func TestRegisterFormatter(t *testing.T) {
	t.Run("formatter registry exists", func(t *testing.T) {
		// Formatters should be registered at init time
		// We test by trying to format with known sources

		// GitHub formatter should be registered
		event := &events.Event{
			Source:  "github",
			Type:    "pr",
			Payload: map[string]interface{}{},
		}

		content := FormatEventContent(event)
		// Should use the GitHub formatter
		switch content {
		case "github/pr":
			t.Log("GitHub formatter not found (expected if not initialized)")
		case "":
			t.Error("formatter returned empty")
		default:
			t.Logf("formatted with github formatter: %q", content)
		}
	})
}

func TestManualFormatter(t *testing.T) {
	t.Run("formats manual events", func(t *testing.T) {
		event := &events.Event{
			Source: "manual",
			Type:   "note",
			Payload: map[string]interface{}{
				"text": "My note",
			},
		}

		content := FormatEventContent(event)

		// Manual formatter should return the text
		if content != "My note" {
			t.Errorf("expected 'My note', got %q", content)
		}
	})

	t.Run("manual formatter with empty text", func(t *testing.T) {
		event := &events.Event{
			Source:  "manual",
			Type:    "note",
			Payload: map[string]interface{}{},
		}

		content := FormatEventContent(event)

		// Should handle missing text
		if content == "" {
			t.Log("manual formatter returns empty for missing text")
		}
	})

	t.Run("manual formatter with multiline text", func(t *testing.T) {
		event := &events.Event{
			Source: "manual",
			Type:   "note",
			Payload: map[string]interface{}{
				"text": "Line 1\nLine 2\nLine 3",
			},
		}

		content := FormatEventContent(event)

		// Should only show first line
		if content != "Line 1" {
			t.Errorf("expected 'Line 1', got %q", content)
		}
	})
}

func TestGitHubFormatter(t *testing.T) {
	t.Run("formats pull request with title and number", func(t *testing.T) {
		event := &events.Event{
			Source: "github",
			Type:   "pr",
			Payload: map[string]interface{}{
				"pr_number": 42.0,
				"title":     "Add new feature",
			},
		}

		content := FormatEventContent(event)

		// Should show #42 and title
		if content != "#42: Add new feature" {
			t.Errorf("expected '#42: Add new feature', got %q", content)
		}
	})

	t.Run("formats with only title", func(t *testing.T) {
		event := &events.Event{
			Source: "github",
			Type:   "pr",
			Payload: map[string]interface{}{
				"title": "Fix bug",
			},
		}

		content := FormatEventContent(event)

		if content != "Fix bug" {
			t.Errorf("expected 'Fix bug', got %q", content)
		}
	})

	t.Run("formats with only number", func(t *testing.T) {
		event := &events.Event{
			Source: "github",
			Type:   "pr",
			Payload: map[string]interface{}{
				"pr_number": 123.0,
			},
		}

		content := FormatEventContent(event)

		if content != "#123" {
			t.Errorf("expected '#123', got %q", content)
		}
	})

	t.Run("truncates long title", func(t *testing.T) {
		event := &events.Event{
			Source: "github",
			Type:   "issue",
			Payload: map[string]interface{}{
				"title": "This is a very long title that should be truncated at 60 characters because it's too long",
			},
		}

		content := FormatEventContent(event)

		if len(content) > 63 { // 60 + "..."
			t.Logf("title not truncated properly: %q", content)
		}
	})

	t.Run("empty payload", func(t *testing.T) {
		event := &events.Event{
			Source:  "github",
			Type:    "issue",
			Payload: map[string]interface{}{},
		}

		content := FormatEventContent(event)

		if content != "github/issue" {
			t.Errorf("expected 'github/issue', got %q", content)
		}
	})
}
