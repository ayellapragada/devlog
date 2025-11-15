package formatting

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"devlog/internal/events"
)

func captureOutput(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func TestFormatDurationMs(t *testing.T) {
	tests := []struct {
		name     string
		ms       int64
		expected string
	}{
		{
			name:     "milliseconds",
			ms:       500,
			expected: "500ms",
		},
		{
			name:     "seconds",
			ms:       2500,
			expected: "2.5s",
		},
		{
			name:     "minutes and seconds",
			ms:       125000,
			expected: "2m5s",
		},
		{
			name:     "exactly one minute",
			ms:       60000,
			expected: "1m0s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatDurationMs(tt.ms)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestFormatCommitContent(t *testing.T) {
	event := events.NewEvent("git", "commit")
	event.Branch = "main"
	event.Payload["hash"] = "abc123def456"
	event.Payload["message"] = "Add new feature"

	output := captureOutput(func() {
		formatCommitContent(event)
	})

	if !strings.Contains(output, "Add new feature") {
		t.Error("Output should contain commit message")
	}

	if !strings.Contains(output, "main@abc123d") {
		t.Error("Output should contain branch and short hash")
	}
}

func TestFormatCommitContentLongMessage(t *testing.T) {
	event := events.NewEvent("git", "commit")
	event.Payload["message"] = strings.Repeat("a", 100)

	output := captureOutput(func() {
		formatCommitContent(event)
	})

	if !strings.Contains(output, "...") {
		t.Error("Long message should be truncated with ...")
	}

	if len(output) > 80 {
		t.Errorf("Output too long: %d characters", len(output))
	}
}

func TestFormatCommitContentMultilineMessage(t *testing.T) {
	event := events.NewEvent("git", "commit")
	event.Payload["message"] = "First line\nSecond line\nThird line"

	output := captureOutput(func() {
		formatCommitContent(event)
	})

	if !strings.Contains(output, "First line") {
		t.Error("Output should contain first line")
	}

	if strings.Contains(output, "Second line") {
		t.Error("Output should not contain second line")
	}
}

func TestFormatMergeContent(t *testing.T) {
	event := events.NewEvent("git", "merge")
	event.Branch = "main"
	event.Payload["source_branch"] = "feature/new-stuff"

	output := captureOutput(func() {
		formatMergeContent(event)
	})

	if !strings.Contains(output, "feature/new-stuff → main") {
		t.Error("Output should show merge from source to target")
	}
}

func TestFormatCommandContent(t *testing.T) {
	event := events.NewEvent("shell", "command")
	event.Payload["command"] = "npm install"
	event.Payload["exit_code"] = 0
	event.Payload["duration_ms"] = int64(1500)

	output := captureOutput(func() {
		formatCommandContent(event)
	})

	if !strings.Contains(output, "npm install") {
		t.Error("Output should contain command")
	}

	if !strings.Contains(output, "1.5s") {
		t.Error("Output should contain duration")
	}
}

func TestFormatCommandContentWithError(t *testing.T) {
	event := events.NewEvent("shell", "command")
	event.Payload["command"] = "make build"
	event.Payload["exit_code"] = 1

	output := captureOutput(func() {
		formatCommandContent(event)
	})

	if !strings.Contains(output, "[exit:1]") {
		t.Error("Output should show non-zero exit code")
	}
}

func TestFormatCommandContentLongCommand(t *testing.T) {
	event := events.NewEvent("shell", "command")
	event.Payload["command"] = strings.Repeat("a", 100)

	output := captureOutput(func() {
		formatCommandContent(event)
	})

	if !strings.Contains(output, "...") {
		t.Error("Long command should be truncated")
	}
}

func TestFormatNoteContent(t *testing.T) {
	event := events.NewEvent("manual", "note")
	event.Payload["text"] = "This is a note"

	output := captureOutput(func() {
		formatNoteContent(event)
	})

	if !strings.Contains(output, "This is a note") {
		t.Error("Output should contain note text")
	}
}

func TestFormatNoteContentEmpty(t *testing.T) {
	event := events.NewEvent("manual", "note")
	event.Payload["text"] = ""

	output := captureOutput(func() {
		formatNoteContent(event)
	})

	if !strings.Contains(output, "(empty)") {
		t.Error("Output should show (empty) for empty note")
	}
}

func TestFormatPRContent(t *testing.T) {
	event := events.NewEvent("github", "pr_merged")
	event.Payload["pr_number"] = float64(123)
	event.Payload["title"] = "Add new feature"

	output := captureOutput(func() {
		formatPRContent(event)
	})

	if !strings.Contains(output, "#123") {
		t.Error("Output should contain PR number")
	}

	if !strings.Contains(output, "Add new feature") {
		t.Error("Output should contain PR title")
	}
}

func TestFormatTranscriptionContent(t *testing.T) {
	event := events.NewEvent("wisprflow", "transcription")
	event.Payload["text"] = "This is a test transcription"
	event.Payload["app"] = "/Applications/Slack.app"
	event.Payload["num_words"] = 5

	output := captureOutput(func() {
		formatTranscriptionContent(event)
	})

	if !strings.Contains(output, "This is a test transcription") {
		t.Error("Output should contain transcription text")
	}

	if !strings.Contains(output, "[Slack.app]") {
		t.Error("Output should contain app name")
	}

	if !strings.Contains(output, "(5 words)") {
		t.Error("Output should contain word count")
	}
}

func TestFormatTranscriptionContentLongText(t *testing.T) {
	event := events.NewEvent("wisprflow", "transcription")
	event.Payload["text"] = strings.Repeat("word ", 30)

	output := captureOutput(func() {
		formatTranscriptionContent(event)
	})

	if !strings.Contains(output, "...") {
		t.Error("Long transcription should be truncated")
	}
}
