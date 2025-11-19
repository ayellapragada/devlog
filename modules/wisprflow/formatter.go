package wisprflow

import (
	"fmt"
	"path/filepath"

	"devlog/internal/events"
	"devlog/internal/formatting"
)

type WisprflowFormatter struct{}

func init() {
	formatting.Register("wisprflow", &WisprflowFormatter{})
}

func (f *WisprflowFormatter) Format(event *events.Event) string {
	text := ""
	if t, ok := event.Payload["text"].(string); ok {
		text = t
		if len(text) > 80 {
			text = text[:80] + "..."
		}
	}

	result := text
	if result == "" {
		result = "(empty)"
	}

	if app, ok := event.Payload["app"].(string); ok {
		if app != "" {
			appName := filepath.Base(app)
			result += fmt.Sprintf(" [%s]", appName)
		}
	}

	if numWords, ok := event.Payload["num_words"].(float64); ok {
		result += fmt.Sprintf(" (%d words)", int(numWords))
	} else if numWords, ok := event.Payload["num_words"].(int); ok {
		result += fmt.Sprintf(" (%d words)", numWords)
	}

	return result
}
