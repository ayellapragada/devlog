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
	if t, ok := event.Payload["text"].(string); ok {
		text = formatting.TruncateToFirstLine(t, 80)
	}

	if text != "" {
		return text
	}
	return fmt.Sprintf("clipboard/%s", event.Type)
}
