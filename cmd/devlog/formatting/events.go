package formatting

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"devlog/internal/events"
	internalFormatting "devlog/internal/formatting"

	_ "devlog/modules/claude"
	_ "devlog/modules/clipboard"
	_ "devlog/modules/git"
	_ "devlog/modules/shell"
	_ "devlog/modules/wisprflow"
)

func FormatEvent(event *events.Event) {
	ts, _ := time.Parse(time.RFC3339, event.Timestamp)
	fmt.Printf("[%s] ", ts.Format("2006-01-02 15:04:05"))

	typeTag := getTypeTag(event)
	fmt.Printf("(%s) ", typeTag)

	folder := getFolder(event)
	if folder != "" {
		fmt.Printf("%s: ", folder)
	}

	content := internalFormatting.FormatEventContent(event)
	fmt.Print(content)

	fmt.Println()
}

func getTypeTag(event *events.Event) string {
	switch event.Type {
	case "commit", "merge", "push", "pull", "fetch", "checkout", "rebase", "stash":
		return "git"
	case "command":
		if event.Source == "claude" {
			return "claude-cmd"
		}
		return "shell"
	case "note":
		return "note"
	case "pr_merged":
		return "github"
	case "transcription":
		return "voice"
	case "conversation":
		return "conversation"
	case "file_edit":
		return "edit"
	default:
		return event.Type
	}
}

func getFolder(event *events.Event) string {
	if event.Repo != "" {
		return filepath.Base(event.Repo)
	}
	if event.Type == "command" {
		if workdir, ok := event.Payload["workdir"].(string); ok {
			return filepath.Base(workdir)
		}
	}
	return ""
}

func FormatEventVerbose(event *events.Event) {
	ts, _ := time.Parse(time.RFC3339, event.Timestamp)

	fmt.Println("─────────────────────────────────────────────────────────")
	fmt.Printf("ID:        %s\n", event.ID)
	fmt.Printf("Timestamp: %s\n", ts.Format("2006-01-02 15:04:05 MST"))
	fmt.Printf("Source:    %s\n", event.Source)
	fmt.Printf("Type:      %s\n", event.Type)

	if event.Repo != "" {
		fmt.Printf("Repo:      %s\n", event.Repo)
	}

	if event.Branch != "" {
		fmt.Printf("Branch:    %s\n", event.Branch)
	}

	if len(event.Payload) > 0 {
		fmt.Println("Payload:")
		payloadJSON, err := json.MarshalIndent(event.Payload, "  ", "  ")
		if err == nil {
			fmt.Printf("  %s\n", string(payloadJSON))
		}
	}

	fmt.Println()
}
