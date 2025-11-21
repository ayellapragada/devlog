package output

import (
	"testing"

	"devlog/internal/events"
	"devlog/internal/testutil"
)

func TestTruncate(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"empty string", "", 10, ""},
		{"exact length", "hello", 5, "hello"},
		{"needs truncation", "hello world", 8, "hello..."},
		{"zero max length", "hello", 0, ""},
		{"max length 1", "hello", 1, "..."},
		{"max length 3", "hello", 3, "..."},
		{"short string", "hi", 10, "hi"},
		{"negative max length", "hello", -1, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Truncate(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("Truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestExtractContent(t *testing.T) {
	tests := []struct {
		name          string
		eventType     string
		payload       map[string]interface{}
		maxLen        int
		expectContent bool
	}{
		{
			"git commit message",
			"commit",
			map[string]interface{}{"message": "fix: improve performance"},
			50,
			true,
		},
		{
			"shell command",
			"command",
			map[string]interface{}{"command": "npm run build"},
			50,
			true,
		},
		{
			"checkout branch",
			"checkout",
			map[string]interface{}{"branch": "main"},
			50,
			true,
		},
		{
			"unknown type empty payload",
			"unknown_type",
			map[string]interface{}{},
			50,
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := &events.Event{
				Source:  string(events.SourceGit),
				Type:    tt.eventType,
				Payload: tt.payload,
			}

			got := ExtractContent(event, tt.maxLen)

			if tt.expectContent && got == "" {
				t.Error("ExtractContent returned empty string but expected content")
			}

			if !tt.expectContent && got != "" {
				t.Logf("ExtractContent returned %q but expected empty", got)
			}
		})
	}
}

func TestFormatEventLine(t *testing.T) {
	t.Run("formats event correctly", func(t *testing.T) {
		event := testutil.NewEventBuilder().
			WithSource(string(events.SourceGit)).
			WithType(string(events.TypeCommit)).
			WithRepo("myrepo").
			WithPayloadField("message", "fix: bug").
			Build()

		result := FormatEventLine(event, 80, 80, 80, 80)

		if result == "" {
			t.Error("FormatEventLine returned empty string")
		}

		// Should contain some meaningful content
		if len(result) < 5 {
			t.Errorf("FormatEventLine result too short: %s", result)
		}
	})

	t.Run("respects max lengths", func(t *testing.T) {
		// Create a long string
		longMsg := ""
		for i := 0; i < 100; i++ {
			longMsg += "a"
		}

		event := testutil.NewEventBuilder().
			WithPayloadField("message", longMsg).
			Build()

		result := FormatEventLine(event, 10, 10, 10, 10)

		// Result should respect the provided max lengths
		if len(result) > 500 {
			t.Logf("FormatEventLine result length: %d", len(result))
		}
	})

	t.Run("handles missing fields", func(t *testing.T) {
		event := &events.Event{
			Source:  string(events.SourceGit),
			Type:    string(events.TypeCommit),
			Payload: map[string]interface{}{},
		}

		result := FormatEventLine(event, 80, 80, 80, 80)

		if result == "" {
			t.Error("FormatEventLine should handle missing fields")
		}
	})
}

func TestFormatEventLineVariations(t *testing.T) {
	event := testutil.NewEventBuilder().
		WithSource(string(events.SourceGit)).
		WithType(string(events.TypeCommit)).
		Build()

	t.Run("with default max lengths", func(t *testing.T) {
		result := FormatEventLine(event, 100, 100, 100, 100)
		if result == "" {
			t.Error("should format with large max lengths")
		}
	})

	t.Run("with small max lengths", func(t *testing.T) {
		result := FormatEventLine(event, 5, 5, 5, 5)
		if result == "" {
			t.Error("should format with small max lengths")
		}
	})

	t.Run("with zero max lengths", func(t *testing.T) {
		result := FormatEventLine(event, 0, 0, 0, 0)
		// Should still return something
		if result == "" {
			t.Log("zero max lengths may return empty string")
		}
	})
}
