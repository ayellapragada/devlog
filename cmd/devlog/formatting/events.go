package formatting

import (
	"fmt"
	"path/filepath"
	"time"

	"devlog/internal/events"
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

	switch event.Type {
	case "commit":
		formatCommitContent(event)
	case "merge":
		formatMergeContent(event)
	case "command":
		formatCommandContent(event)
	case "note":
		formatNoteContent(event)
	case "pr_merged":
		formatPRContent(event)
	case "transcription":
		formatTranscriptionContent(event)
	default:
		fmt.Printf("%s/%s", event.Source, event.Type)
	}

	fmt.Println()
}

func getTypeTag(event *events.Event) string {
	switch event.Type {
	case "commit":
		return "git"
	case "merge":
		return "git"
	case "command":
		return "shell"
	case "note":
		return "note"
	case "pr_merged":
		return "github"
	case "transcription":
		return "voice"
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
