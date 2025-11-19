package git

import (
	"bytes"
	"fmt"

	"devlog/internal/events"
	"devlog/internal/formatting"
)

type GitFormatter struct{}

func init() {
	formatting.Register("git", &GitFormatter{})
}

func (f *GitFormatter) Format(event *events.Event) string {
	switch event.Type {
	case "commit":
		return f.formatCommit(event)
	case "merge":
		return f.formatMerge(event)
	case "push":
		return f.formatPush(event)
	case "pull":
		return f.formatPull(event)
	case "fetch":
		return f.formatFetch(event)
	case "checkout":
		return f.formatCheckout(event)
	case "rebase":
		return f.formatRebase(event)
	case "stash":
		return f.formatStash(event)
	default:
		return fmt.Sprintf("git/%s", event.Type)
	}
}

func (f *GitFormatter) formatCommit(event *events.Event) string {
	message := ""
	if msg, ok := event.Payload["message"].(string); ok {
		if idx := bytes.IndexByte([]byte(msg), '\n'); idx != -1 {
			message = msg[:idx]
		} else {
			message = msg
		}
		if len(message) > 60 {
			message = message[:60] + "..."
		}
	}

	result := message

	var metadata []string
	if event.Branch != "" {
		metadata = append(metadata, event.Branch)
	}
	if hash, ok := event.Payload["hash"].(string); ok {
		if len(hash) > 7 {
			hash = hash[:7]
		}
		if len(metadata) > 0 {
			result += fmt.Sprintf(" [%s@%s]", metadata[0], hash)
		} else {
			result += fmt.Sprintf(" [%s]", hash)
		}
	} else if len(metadata) > 0 {
		result += fmt.Sprintf(" [%s]", metadata[0])
	}

	return result
}

func (f *GitFormatter) formatMerge(event *events.Event) string {
	mergedBranch := ""
	if mb, ok := event.Payload["merged_branch"].(string); ok {
		mergedBranch = mb
	} else if src, ok := event.Payload["source_branch"].(string); ok {
		mergedBranch = src
	}

	target := event.Branch
	if target == "" {
		target = "?"
	}

	if mergedBranch != "" {
		return fmt.Sprintf("merged %s → %s", mergedBranch, target)
	}
	return fmt.Sprintf("merged → %s", target)
}

func (f *GitFormatter) formatPush(event *events.Event) string {
	remote := "origin"
	if r, ok := event.Payload["remote"].(string); ok && r != "" {
		remote = r
	}

	ref := event.Branch
	if r, ok := event.Payload["ref"].(string); ok && r != "" {
		ref = r
	}

	provider := ""
	if p, ok := event.Payload["hosting_provider"].(string); ok && p != "" && p != "other" {
		provider = fmt.Sprintf(" (%s)", p)
	}

	return fmt.Sprintf("pushed to %s/%s%s", remote, ref, provider)
}

func (f *GitFormatter) formatPull(event *events.Event) string {
	remote := "origin"
	if r, ok := event.Payload["remote"].(string); ok && r != "" {
		remote = r
	}

	provider := ""
	if p, ok := event.Payload["hosting_provider"].(string); ok && p != "" && p != "other" {
		provider = fmt.Sprintf(" (%s)", p)
	}

	result := fmt.Sprintf("pulled from %s%s", remote, provider)
	if event.Branch != "" {
		result += fmt.Sprintf(" [%s]", event.Branch)
	}
	return result
}

func (f *GitFormatter) formatFetch(event *events.Event) string {
	remote := "origin"
	if r, ok := event.Payload["remote"].(string); ok && r != "" {
		remote = r
	}

	provider := ""
	if p, ok := event.Payload["hosting_provider"].(string); ok && p != "" && p != "other" {
		provider = fmt.Sprintf(" (%s)", p)
	}

	return fmt.Sprintf("fetched from %s%s", remote, provider)
}

func (f *GitFormatter) formatCheckout(event *events.Event) string {
	fromBranch := ""
	if fb, ok := event.Payload["from_branch"].(string); ok {
		fromBranch = fb
	}

	toBranch := event.Branch
	if toBranch == "" {
		toBranch = "?"
	}

	if fromBranch != "" {
		return fmt.Sprintf("switched %s → %s", fromBranch, toBranch)
	}
	return fmt.Sprintf("switched to %s", toBranch)
}

func (f *GitFormatter) formatRebase(event *events.Event) string {
	targetBranch := ""
	if tb, ok := event.Payload["target_branch"].(string); ok {
		targetBranch = tb
	}

	result := ""
	if targetBranch != "" {
		result = fmt.Sprintf("rebased onto %s", targetBranch)
	} else {
		result = "rebased"
	}

	if event.Branch != "" {
		result += fmt.Sprintf(" [%s]", event.Branch)
	}
	return result
}

func (f *GitFormatter) formatStash(event *events.Event) string {
	action := "push"
	if a, ok := event.Payload["stash_action"].(string); ok && a != "" {
		action = a
	}

	result := fmt.Sprintf("stash %s", action)
	if event.Branch != "" {
		result += fmt.Sprintf(" [%s]", event.Branch)
	}
	return result
}
