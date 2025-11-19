package claude

import (
	"fmt"
	"path/filepath"

	"devlog/internal/events"
	"devlog/internal/formatting"
)

type ClaudeFormatter struct{}

func init() {
	formatting.Register("claude", &ClaudeFormatter{})
}

func (f *ClaudeFormatter) Format(event *events.Event) string {
	switch event.Type {
	case "conversation":
		return f.formatConversation(event)
	case "command":
		return f.formatCommand(event)
	case "file_edit":
		return f.formatFileEdit(event)
	default:
		return fmt.Sprintf("claude/%s", event.Type)
	}
}

func (f *ClaudeFormatter) formatConversation(event *events.Event) string {
	summary := ""
	if s, ok := event.Payload["summary"].(string); ok {
		summary = s
	} else if msg, ok := event.Payload["user_message"].(string); ok {
		summary = msg
		if len(summary) > 80 {
			summary = summary[:80] + "..."
		}
	}

	result := summary
	if result == "" {
		result = "claude/conversation"
	}

	var metadata []string
	if cmdCount, ok := event.Payload["command_count"].(float64); ok && cmdCount > 0 {
		metadata = append(metadata, fmt.Sprintf("%d cmds", int(cmdCount)))
	}
	if editCount, ok := event.Payload["edit_count"].(float64); ok && editCount > 0 {
		metadata = append(metadata, fmt.Sprintf("%d edits", int(editCount)))
	}
	if readCount, ok := event.Payload["read_count"].(float64); ok && readCount > 0 {
		metadata = append(metadata, fmt.Sprintf("%d reads", int(readCount)))
	}

	if len(metadata) > 0 {
		result += " ["
		for i, m := range metadata {
			if i > 0 {
				result += ", "
			}
			result += m
		}
		result += "]"
	}

	return result
}

func (f *ClaudeFormatter) formatCommand(event *events.Event) string {
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

func (f *ClaudeFormatter) formatFileEdit(event *events.Event) string {
	filePath := ""
	if fp, ok := event.Payload["file_path"].(string); ok {
		filePath = filepath.Base(fp)
	}

	if filePath != "" {
		return fmt.Sprintf("edited %s", filePath)
	}
	return "edited file"
}
