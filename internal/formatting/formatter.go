package formatting

import (
	"bytes"
	"fmt"
	"sync"

	"devlog/internal/events"
)

type Formatter interface {
	Format(event *events.Event) string
}

var (
	formatters = make(map[string]Formatter)
	mu         sync.RWMutex
)

func Register(source string, formatter Formatter) {
	mu.Lock()
	defer mu.Unlock()
	formatters[source] = formatter
}

func FormatEventContent(event *events.Event) string {
	mu.RLock()
	formatter, exists := formatters[event.Source]
	mu.RUnlock()

	if exists {
		return formatter.Format(event)
	}

	return fmt.Sprintf("%s/%s", event.Source, event.Type)
}

func FormatDurationMs(ms int64) string {
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	seconds := ms / 1000
	if seconds < 60 {
		return fmt.Sprintf("%.1fs", float64(ms)/1000)
	}
	minutes := seconds / 60
	seconds %= 60
	return fmt.Sprintf("%dm%ds", minutes, seconds)
}

func TruncateToFirstLine(text string, maxLen int) string {
	if idx := bytes.IndexByte([]byte(text), '\n'); idx != -1 {
		text = text[:idx]
	}
	if len(text) > maxLen {
		text = text[:maxLen] + "..."
	}
	return text
}
