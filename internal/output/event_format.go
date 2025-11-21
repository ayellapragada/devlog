package output

import (
	"fmt"
	"unicode/utf8"

	"devlog/internal/events"
)

func Truncate(s string, maxLen int) string {
	if s == "" || maxLen <= 0 {
		return ""
	}
	if maxLen <= 3 {
		return "..."
	}

	runeCount := utf8.RuneCountInString(s)
	if runeCount <= maxLen {
		return s
	}

	runes := []rune(s)
	if len(runes) > maxLen-3 {
		return string(runes[:maxLen-3]) + "..."
	}
	return s + "..."
}

func ExtractContent(evt *events.Event, maxLen int) string {
	payload := evt.Payload

	switch evt.Type {
	case "commit":
		if msg, ok := payload["message"].(string); ok {
			return Truncate(msg, maxLen)
		}
	case "command":
		if cmd, ok := payload["command"].(string); ok {
			return Truncate(cmd, maxLen)
		}
	case "transcription":
		if text, ok := payload["text"].(string); ok {
			return Truncate(text, maxLen)
		}
	case "conversation":
		if summary, ok := payload["summary"].(string); ok {
			return Truncate(summary, maxLen)
		}
	case "copy":
		if content, ok := payload["content"].(string); ok {
			return Truncate(content, maxLen)
		}
	case "push", "pull", "fetch":
		if ref, ok := payload["ref"].(string); ok {
			return "ref: " + ref
		}
	case "checkout":
		if branch, ok := payload["branch"].(string); ok {
			return "→ " + branch
		}
	case "merge":
		if source, ok := payload["source"].(string); ok {
			if target, ok := payload["target"].(string); ok {
				return source + " → " + target
			}
			return source
		}
	case "note":
		if note, ok := payload["note"].(string); ok {
			return Truncate(note, maxLen)
		}
	case "file_edit":
		if file, ok := payload["file"].(string); ok {
			return file
		}
	}

	return ""
}

func FormatEventLine(evt *events.Event, maxSummaryLen int, maxMessageLen int, maxCommandLen int, maxTextLen int) string {
	line := fmt.Sprintf("\n[%s] source=%s type=%s", evt.Timestamp, evt.Source, evt.Type)

	if evt.Repo != "" {
		line += fmt.Sprintf(" repo=%s", evt.Repo)
	}
	if evt.Branch != "" {
		line += fmt.Sprintf(" branch=%s", evt.Branch)
	}

	if workdir, ok := evt.Payload["workdir"].(string); ok && workdir != "" {
		line += fmt.Sprintf(" (workdir: %s)", workdir)
	}

	if summary, ok := evt.Payload["summary"].(string); ok && summary != "" {
		line += fmt.Sprintf(": %s", Truncate(summary, maxSummaryLen))
	} else if msg, ok := evt.Payload["message"].(string); ok && msg != "" {
		line += fmt.Sprintf(": %s", Truncate(msg, maxMessageLen))
	} else if cmd, ok := evt.Payload["command"].(string); ok && cmd != "" {
		line += fmt.Sprintf(": %s", Truncate(cmd, maxCommandLen))
	} else if text, ok := evt.Payload["text"].(string); ok && text != "" {
		line += fmt.Sprintf(": %s", Truncate(text, maxTextLen))
	}

	return line
}
