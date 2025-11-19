package claude

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"devlog/internal/errors"
)

type ConversationEntry struct {
	Type          string          `json:"type"`
	Timestamp     string          `json:"timestamp"`
	SessionID     string          `json:"sessionId"`
	Message       json.RawMessage `json:"message"`
	UUID          string          `json:"uuid"`
	ParentUUID    *string         `json:"parentUuid"`
	CWD           string          `json:"cwd"`
	GitBranch     string          `json:"gitBranch"`
	ToolUseResult *ToolUseResult  `json:"toolUseResult"`
}

type Message struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type ContentItem struct {
	Type    string          `json:"type"`
	Text    string          `json:"text"`
	ToolUse *ToolUseContent `json:"tool_use,omitempty"`
	ID      string          `json:"tool_use_id,omitempty"`
}

type ToolUseContent struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

type ToolUseResult struct {
	Type   string       `json:"type"`
	File   *FileContent `json:"file,omitempty"`
	Stdout string       `json:"stdout,omitempty"`
	Stderr string       `json:"stderr,omitempty"`
}

type FileContent struct {
	FilePath string `json:"filePath"`
	Content  string `json:"content"`
}

type BashInput struct {
	Command     string `json:"command"`
	Description string `json:"description"`
}

type EditInput struct {
	FilePath  string `json:"file_path"`
	OldString string `json:"old_string"`
	NewString string `json:"new_string"`
}

type WriteInput struct {
	FilePath string `json:"file_path"`
	Content  string `json:"content"`
}

type ParsedConversation struct {
	SessionID     string
	Timestamp     time.Time
	UserMessage   string
	ClaudeMessage string
	Commands      []CommandExecution
	FileEdits     []FileEdit
	FileReads     []FileRead
	CWD           string
	GitBranch     string
}

type CommandExecution struct {
	Command     string
	Description string
	Stdout      string
	Stderr      string
	Timestamp   time.Time
}

type FileEdit struct {
	FilePath  string
	OldString string
	NewString string
	Timestamp time.Time
}

type FileRead struct {
	FilePath  string
	Timestamp time.Time
}

func ParseJSONLFile(filepath string, since time.Time) ([]ParsedConversation, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, errors.WrapModule("claude", "open conversation file", err)
	}
	defer file.Close()

	var entries []ConversationEntry
	scanner := bufio.NewScanner(file)

	const maxCapacity = 10 * 1024 * 1024
	buf := make([]byte, maxCapacity)
	scanner.Buffer(buf, maxCapacity)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var entry ConversationEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}

		if entry.Timestamp != "" {
			ts, err := time.Parse(time.RFC3339, entry.Timestamp)
			if err == nil && ts.After(since) {
				entries = append(entries, entry)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, errors.WrapModule("claude", "scan conversation file", err)
	}

	return aggregateConversations(entries)
}

func aggregateConversations(entries []ConversationEntry) ([]ParsedConversation, error) {
	conversationMap := make(map[string]*ParsedConversation)

	for _, entry := range entries {
		if entry.Type != "user" && entry.Type != "assistant" {
			continue
		}

		ts, err := time.Parse(time.RFC3339, entry.Timestamp)
		if err != nil {
			continue
		}

		conv, exists := conversationMap[entry.SessionID]
		if !exists {
			conv = &ParsedConversation{
				SessionID: entry.SessionID,
				Timestamp: ts,
				CWD:       entry.CWD,
				GitBranch: entry.GitBranch,
			}
			conversationMap[entry.SessionID] = conv
		}

		if entry.Type == "user" {
			text := extractTextFromMessage(entry.Message)

			if entry.ToolUseResult != nil && entry.ToolUseResult.File != nil {
				conv.FileReads = append(conv.FileReads, FileRead{
					FilePath:  entry.ToolUseResult.File.FilePath,
					Timestamp: ts,
				})
			}

			if text != "" {
				conv.UserMessage += text + "\n"
			}
		}

		if entry.Type == "assistant" {
			text, tools := extractContentFromMessage(entry.Message)

			for _, tool := range tools {
				switch tool.Name {
				case "Bash":
					var input BashInput
					if err := json.Unmarshal(tool.Input, &input); err == nil {
						conv.Commands = append(conv.Commands, CommandExecution{
							Command:     input.Command,
							Description: input.Description,
							Timestamp:   ts,
						})
					}
				case "Edit":
					var input EditInput
					if err := json.Unmarshal(tool.Input, &input); err == nil {
						conv.FileEdits = append(conv.FileEdits, FileEdit{
							FilePath:  input.FilePath,
							OldString: input.OldString,
							NewString: input.NewString,
							Timestamp: ts,
						})
					}
				case "Write":
					var input WriteInput
					if err := json.Unmarshal(tool.Input, &input); err == nil {
						conv.FileEdits = append(conv.FileEdits, FileEdit{
							FilePath:  input.FilePath,
							NewString: fmt.Sprintf("[New file: %d bytes]", len(input.Content)),
							Timestamp: ts,
						})
					}
				case "Read":
					var input map[string]interface{}
					if err := json.Unmarshal(tool.Input, &input); err == nil {
						if filePath, ok := input["file_path"].(string); ok {
							conv.FileReads = append(conv.FileReads, FileRead{
								FilePath:  filePath,
								Timestamp: ts,
							})
						}
					}
				}
			}

			if text != "" {
				conv.ClaudeMessage += text + "\n"
			}
		}
	}

	var result []ParsedConversation
	for _, conv := range conversationMap {
		result = append(result, *conv)
	}

	return result, nil
}

func extractTextFromMessage(msgRaw json.RawMessage) string {
	var msg Message
	if err := json.Unmarshal(msgRaw, &msg); err != nil {
		return ""
	}

	var content []ContentItem
	if err := json.Unmarshal(msg.Content, &content); err != nil {
		var textContent string
		if err := json.Unmarshal(msg.Content, &textContent); err == nil {
			return textContent
		}
		return ""
	}

	var texts []string
	for _, item := range content {
		if item.Type == "text" && item.Text != "" {
			text := item.Text
			if strings.Contains(text, "<ide_opened_file>") {
				startIdx := strings.Index(text, "<ide_opened_file>")
				endIdx := strings.Index(text, "</ide_opened_file>")
				if startIdx != -1 && endIdx != -1 && endIdx > startIdx {
					text = text[:startIdx] + text[endIdx+len("</ide_opened_file>"):]
					text = strings.TrimSpace(text)
				}
			}
			if text != "" {
				texts = append(texts, text)
			}
		}
	}

	return strings.Join(texts, "\n")
}

func extractContentFromMessage(msgRaw json.RawMessage) (string, []ToolUseContent) {
	var msg Message
	if err := json.Unmarshal(msgRaw, &msg); err != nil {
		return "", nil
	}

	var content []ContentItem
	if err := json.Unmarshal(msg.Content, &content); err != nil {
		return "", nil
	}

	var texts []string
	var tools []ToolUseContent

	for _, item := range content {
		if item.Type == "text" && item.Text != "" {
			texts = append(texts, item.Text)
		}
		if item.Type == "tool_use" && item.ToolUse != nil {
			tools = append(tools, *item.ToolUse)
		}
	}

	return strings.Join(texts, "\n"), tools
}
