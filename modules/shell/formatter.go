package shell

import (
	"fmt"

	"devlog/internal/events"
	"devlog/internal/formatting"
)

type ShellFormatter struct{}

func init() {
	formatting.Register("shell", &ShellFormatter{})
}

func (f *ShellFormatter) Format(event *events.Event) string {
	cmd := ""
	if c, ok := event.Payload["command"].(string); ok {
		cmd = c
		if len(cmd) > 70 {
			cmd = cmd[:70] + "..."
		}
	}

	result := cmd

	if exitCode, ok := event.Payload["exit_code"].(float64); ok && exitCode != 0 {
		result += fmt.Sprintf(" [exit:%d]", int(exitCode))
	} else if exitCode, ok := event.Payload["exit_code"].(int); ok && exitCode != 0 {
		result += fmt.Sprintf(" [exit:%d]", exitCode)
	}

	if duration, ok := event.Payload["duration_ms"].(float64); ok && duration > 0 {
		result += fmt.Sprintf(" [%s]", formatting.FormatDurationMs(int64(duration)))
	} else if duration, ok := event.Payload["duration_ms"].(int64); ok && duration > 0 {
		result += fmt.Sprintf(" [%s]", formatting.FormatDurationMs(duration))
	}

	return result
}
