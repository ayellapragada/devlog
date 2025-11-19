package formatting

import (
	"devlog/internal/events"
)

type ManualFormatter struct{}

func init() {
	Register("manual", &ManualFormatter{})
}

func (f *ManualFormatter) Format(event *events.Event) string {
	text := ""
	if t, ok := event.Payload["text"].(string); ok {
		text = TruncateToFirstLine(t, 80)
	}

	if text != "" {
		return text
	}
	return "(empty)"
}
