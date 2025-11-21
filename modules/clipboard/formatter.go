package clipboard

import (
	"fmt"

	"devlog/internal/events"
	"devlog/internal/formatting"
)

type ClipboardFormatter struct{}

func init() {
	formatting.Register("clipboard", &ClipboardFormatter{})
}

func (f *ClipboardFormatter) Format(event *events.Event) string {
	text := ""
	if t, ok := event.Payload["content"].(string); ok {
		text = formatting.TruncateToFirstLine(t, 60)
	}

	if text != "" {
		return fmt.Sprintf("content: %s", text)
	}
	return "clipboard event"
}
