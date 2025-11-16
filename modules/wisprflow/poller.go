package wisprflow

import (
	"context"
	"fmt"
	"strings"
	"time"

	"devlog/internal/events"
	"devlog/internal/state"
)

type Poller struct {
	dbPath       string
	pollInterval time.Duration
	minWords     int
	stateMgr     *state.Manager
}

func NewPoller(dbPath, dataDir string, pollInterval time.Duration, minWords int) (*Poller, error) {
	stateMgr, err := state.NewManager(dataDir)
	if err != nil {
		return nil, fmt.Errorf("create state manager: %w", err)
	}

	return &Poller{
		dbPath:       dbPath,
		pollInterval: pollInterval,
		minWords:     minWords,
		stateMgr:     stateMgr,
	}, nil
}

func (p *Poller) Name() string {
	return "wisprflow"
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

	lastPoll := time.Now()
	if lastPollStr, ok := p.stateMgr.GetString("wisprflow", "last_poll_time"); ok {
		if t, err := time.Parse(time.RFC3339Nano, lastPollStr); err == nil {
			lastPoll = t
		}
	}

	entries, err := PollDatabaseContext(ctx, p.dbPath, lastPoll)
	if err != nil {
		return nil, fmt.Errorf("poll database: %w", err)
	}

	if len(entries) == 0 {
		return nil, nil
	}

	var result []*events.Event
	for _, entry := range entries {
		if p.minWords > 0 && entry.NumWords < p.minWords {
			continue
		}

		text := entry.EditedText
		if text == "" {
			text = entry.FormattedText
		}
		if text == "" {
			text = entry.ASRText
		}

		event := events.NewEvent("wisprflow", "transcription")
		event.ID = entry.TranscriptEntityID
		event.Timestamp = entry.Timestamp.Format(time.RFC3339)
		event.Payload = map[string]interface{}{
			"id":             entry.TranscriptEntityID,
			"text":           text,
			"asr_text":       entry.ASRText,
			"formatted_text": entry.FormattedText,
			"edited_text":    entry.EditedText,
			"app":            entry.App,
			"url":            entry.URL,
			"duration":       entry.Duration,
			"num_words":      entry.NumWords,
			"status":         entry.Status,
		}

		result = append(result, event)
	}

	if len(entries) > 0 {
		lastEntry := entries[len(entries)-1]
		nextPollTime := lastEntry.Timestamp.Add(1 * time.Millisecond)
		if err := p.stateMgr.Set("wisprflow", "last_poll_time", nextPollTime.Format(time.RFC3339Nano)); err != nil {
			return nil, fmt.Errorf("save poll timestamp: %w", err)
		}
	}

	return result, nil
}

func (p *Poller) ShouldRetryOnError(err error) bool {
	return !strings.Contains(err.Error(), "UNIQUE constraint failed")
}
