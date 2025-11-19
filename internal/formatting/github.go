package formatting

import (
	"fmt"

	"devlog/internal/events"
)

type GitHubFormatter struct{}

func init() {
	Register("github", &GitHubFormatter{})
}

func (f *GitHubFormatter) Format(event *events.Event) string {
	title := ""
	if t, ok := event.Payload["title"].(string); ok {
		title = t
		if len(title) > 60 {
			title = title[:60] + "..."
		}
	}

	prNum := ""
	if num, ok := event.Payload["pr_number"].(float64); ok {
		prNum = fmt.Sprintf("#%.0f", num)
	}

	switch {
	case prNum != "" && title != "":
		return fmt.Sprintf("%s: %s", prNum, title)
	case prNum != "":
		return prNum
	case title != "":
		return title
	default:
		return fmt.Sprintf("github/%s", event.Type)
	}
}
