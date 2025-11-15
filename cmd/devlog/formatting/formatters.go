package formatting

import (
	"bytes"
	"fmt"
	"path/filepath"

	"devlog/internal/events"
)

func formatCommitContent(event *events.Event) {
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

	fmt.Printf("%s", message)

	var metadata []string
	if event.Branch != "" {
		metadata = append(metadata, event.Branch)
	}
	if hash, ok := event.Payload["hash"].(string); ok {
		if len(hash) > 7 {
			hash = hash[:7]
		}
		if len(metadata) > 0 {
			fmt.Printf(" [%s@%s]", metadata[0], hash)
		} else {
			fmt.Printf(" [%s]", hash)
		}
	} else if len(metadata) > 0 {
		fmt.Printf(" [%s]", metadata[0])
	}
}

func formatMergeContent(event *events.Event) {
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
		fmt.Printf("merged %s → %s", mergedBranch, target)
	} else {
		fmt.Printf("merged → %s", target)
	}
}

func formatPushContent(event *events.Event) {
	remote := "origin"
	if r, ok := event.Payload["remote"].(string); ok && r != "" {
		remote = r
	}

	ref := event.Branch
	if r, ok := event.Payload["ref"].(string); ok && r != "" {
		ref = r
	}

	fmt.Printf("pushed to %s/%s", remote, ref)
}

func formatPullContent(event *events.Event) {
	remote := "origin"
	if r, ok := event.Payload["remote"].(string); ok && r != "" {
		remote = r
	}

	fmt.Printf("pulled from %s", remote)
	if event.Branch != "" {
		fmt.Printf(" [%s]", event.Branch)
	}
}

func formatFetchContent(event *events.Event) {
	remote := "origin"
	if r, ok := event.Payload["remote"].(string); ok && r != "" {
		remote = r
	}

	fmt.Printf("fetched from %s", remote)
}

func formatCheckoutContent(event *events.Event) {
	fromBranch := ""
	if fb, ok := event.Payload["from_branch"].(string); ok {
		fromBranch = fb
	}

	toBranch := event.Branch
	if toBranch == "" {
		toBranch = "?"
	}

	if fromBranch != "" {
		fmt.Printf("switched %s → %s", fromBranch, toBranch)
	} else {
		fmt.Printf("switched to %s", toBranch)
	}
}

func formatRebaseContent(event *events.Event) {
	targetBranch := ""
	if tb, ok := event.Payload["target_branch"].(string); ok {
		targetBranch = tb
	}

	if targetBranch != "" {
		fmt.Printf("rebased onto %s", targetBranch)
	} else {
		fmt.Printf("rebased")
	}

	if event.Branch != "" {
		fmt.Printf(" [%s]", event.Branch)
	}
}

func formatStashContent(event *events.Event) {
	action := "push"
	if a, ok := event.Payload["stash_action"].(string); ok && a != "" {
		action = a
	}

	fmt.Printf("stash %s", action)
	if event.Branch != "" {
		fmt.Printf(" [%s]", event.Branch)
	}
}

func formatCommandContent(event *events.Event) {
	cmd := ""
	if c, ok := event.Payload["command"].(string); ok {
		cmd = c
		if len(cmd) > 70 {
			cmd = cmd[:70] + "..."
		}
	}

	fmt.Printf("%s", cmd)

	if exitCode, ok := event.Payload["exit_code"].(float64); ok && exitCode != 0 {
		fmt.Printf(" [exit:%d]", int(exitCode))
	} else if exitCode, ok := event.Payload["exit_code"].(int); ok && exitCode != 0 {
		fmt.Printf(" [exit:%d]", exitCode)
	}

	if duration, ok := event.Payload["duration_ms"].(float64); ok && duration > 0 {
		fmt.Printf(" [%s]", formatDurationMs(int64(duration)))
	} else if duration, ok := event.Payload["duration_ms"].(int64); ok && duration > 0 {
		fmt.Printf(" [%s]", formatDurationMs(duration))
	}
}

func formatNoteContent(event *events.Event) {
	text := ""
	if t, ok := event.Payload["text"].(string); ok {
		if idx := bytes.IndexByte([]byte(t), '\n'); idx != -1 {
			text = t[:idx]
		} else {
			text = t
		}
		if len(text) > 80 {
			text = text[:80] + "..."
		}
	}

	if text != "" {
		fmt.Printf("%s", text)
	} else {
		fmt.Printf("(empty)")
	}
}

func formatPRContent(event *events.Event) {
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

	if prNum != "" && title != "" {
		fmt.Printf("%s: %s", prNum, title)
	} else if prNum != "" {
		fmt.Printf("%s", prNum)
	} else if title != "" {
		fmt.Printf("%s", title)
	}
}

func formatTranscriptionContent(event *events.Event) {
	text := ""
	if t, ok := event.Payload["text"].(string); ok {
		text = t
		if len(text) > 80 {
			text = text[:80] + "..."
		}
	}

	if text != "" {
		fmt.Printf("%s", text)
	} else {
		fmt.Printf("(empty)")
	}

	if app, ok := event.Payload["app"].(string); ok {
		if app != "" {
			appName := filepath.Base(app)
			fmt.Printf(" [%s]", appName)
		}
	}

	if numWords, ok := event.Payload["num_words"].(float64); ok {
		fmt.Printf(" (%d words)", int(numWords))
	} else if numWords, ok := event.Payload["num_words"].(int); ok {
		fmt.Printf(" (%d words)", numWords)
	}
}

func formatDurationMs(ms int64) string {
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	seconds := ms / 1000
	if seconds < 60 {
		return fmt.Sprintf("%.1fs", float64(ms)/1000)
	}
	minutes := seconds / 60
	seconds = seconds % 60
	return fmt.Sprintf("%dm%ds", minutes, seconds)
}
