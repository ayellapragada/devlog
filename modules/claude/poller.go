package claude

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"devlog/internal/errors"
	"devlog/internal/events"
	"devlog/internal/state"
)

type Poller struct {
	projectsDir      string
	pollInterval     time.Duration
	extractCommands  bool
	extractFileEdits bool
	minMessageLength int
	stateMgr         *state.Manager
	logger           *slog.Logger
}

func NewPoller(
	projectsDir string,
	dataDir string,
	pollInterval time.Duration,
	extractCommands bool,
	extractFileEdits bool,
	minMessageLength int,
) (*Poller, error) {
	stateMgr, err := state.NewManager(dataDir)
	if err != nil {
		return nil, errors.WrapModule("claude", "create state manager", err)
	}

	return &Poller{
		projectsDir:      projectsDir,
		pollInterval:     pollInterval,
		extractCommands:  extractCommands,
		extractFileEdits: extractFileEdits,
		minMessageLength: minMessageLength,
		stateMgr:         stateMgr,
		logger:           slog.Default().With("module", "claude"),
	}, nil
}

func (p *Poller) Name() string {
	return "claude"
}

func (p *Poller) PollInterval() time.Duration {
	return p.pollInterval
}

func (p *Poller) Poll(ctx context.Context) ([]*events.Event, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	lastPoll := time.Now().Add(-24 * time.Hour)
	if lastPollStr, ok := p.stateMgr.GetString("claude", "last_poll_time"); ok {
		if t, err := time.Parse(time.RFC3339Nano, lastPollStr); err == nil {
			lastPoll = t
		}
	}

	projectDirs, err := p.getAllProjectDirs()
	if err != nil {
		return nil, errors.WrapModule("claude", "get project directories", err)
	}

	var allEvents []*events.Event
	var latestTimestamp time.Time

	for _, projectDir := range projectDirs {
		conversations, err := p.pollProjectDir(projectDir, lastPoll)
		if err != nil {
			p.logger.Debug("failed to poll project directory",
				slog.String("dir", projectDir),
				slog.String("error", err.Error()))
			continue
		}

		for _, conv := range conversations {
			evts := p.extractEvents(conv)
			allEvents = append(allEvents, evts...)

			if conv.Timestamp.After(latestTimestamp) {
				latestTimestamp = conv.Timestamp
			}
		}
	}

	if latestTimestamp.After(lastPoll) {
		nextPollTime := latestTimestamp.Add(1 * time.Millisecond)
		if err := p.stateMgr.Set("claude", "last_poll_time", nextPollTime.Format(time.RFC3339Nano)); err != nil {
			return nil, errors.WrapModule("claude", "save poll timestamp", err)
		}
	}

	return allEvents, nil
}

func (p *Poller) getAllProjectDirs() ([]string, error) {
	entries, err := os.ReadDir(p.projectsDir)
	if err != nil {
		return nil, errors.WrapModule("claude", "read projects directory", err)
	}

	var projectDirs []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		projectDirs = append(projectDirs, filepath.Join(p.projectsDir, entry.Name()))
	}

	return projectDirs, nil
}

func (p *Poller) pollProjectDir(projectDir string, since time.Time) ([]ParsedConversation, error) {
	entries, err := os.ReadDir(projectDir)
	if err != nil {
		return nil, errors.WrapModule("claude", "read project directory", err)
	}

	var allConversations []ParsedConversation

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}

		filePath := filepath.Join(projectDir, entry.Name())
		conversations, err := ParseJSONLFile(filePath, since)
		if err != nil {
			continue
		}

		allConversations = append(allConversations, conversations...)
	}

	return allConversations, nil
}

func (p *Poller) extractEvents(conv ParsedConversation) []*events.Event {
	var result []*events.Event

	if len(conv.UserMessage) >= p.minMessageLength {
		event := events.NewEvent("claude", "conversation")
		event.ID = conv.SessionID
		event.Timestamp = conv.Timestamp.Format(time.RFC3339)
		event.Repo = conv.CWD
		event.Branch = conv.GitBranch

		summary := conv.UserMessage
		if len(summary) > 200 {
			summary = summary[:200] + "..."
		}

		event.Payload = map[string]interface{}{
			"session_id":    conv.SessionID,
			"user_message":  conv.UserMessage,
			"claude_reply":  conv.ClaudeMessage,
			"summary":       summary,
			"command_count": len(conv.Commands),
			"edit_count":    len(conv.FileEdits),
			"read_count":    len(conv.FileReads),
		}

		result = append(result, event)
	}

	if p.extractCommands {
		for _, cmd := range conv.Commands {
			event := events.NewEvent("claude", "command")
			event.ID = generateID(conv.SessionID, cmd.Command, cmd.Timestamp.String())
			event.Timestamp = cmd.Timestamp.Format(time.RFC3339)
			event.Repo = conv.CWD
			event.Branch = conv.GitBranch

			event.Payload = map[string]interface{}{
				"session_id":  conv.SessionID,
				"command":     cmd.Command,
				"description": cmd.Description,
				"stdout":      cmd.Stdout,
				"stderr":      cmd.Stderr,
			}

			result = append(result, event)
		}
	}

	if p.extractFileEdits {
		for _, edit := range conv.FileEdits {
			event := events.NewEvent("claude", "file_edit")
			event.ID = generateID(conv.SessionID, edit.FilePath, edit.Timestamp.String())
			event.Timestamp = edit.Timestamp.Format(time.RFC3339)
			event.Repo = conv.CWD
			event.Branch = conv.GitBranch

			event.Payload = map[string]interface{}{
				"session_id": conv.SessionID,
				"file_path":  edit.FilePath,
				"old_string": truncateString(edit.OldString, 500),
				"new_string": truncateString(edit.NewString, 500),
			}

			result = append(result, event)
		}
	}

	return result
}

func generateID(parts ...string) string {
	combined := strings.Join(parts, "|")
	hash := md5.Sum([]byte(combined))
	return hex.EncodeToString(hash[:])
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
