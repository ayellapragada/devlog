package wisprflow

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"devlog/internal/events"
)

type Poller struct {
	dbPath       string
	dataDir      string
	pollInterval time.Duration
	minWords     int
}

func NewPoller(dbPath, dataDir string, pollInterval time.Duration, minWords int) *Poller {
	return &Poller{
		dbPath:       dbPath,
		dataDir:      dataDir,
		pollInterval: pollInterval,
		minWords:     minWords,
	}
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

	lastPoll, err := LoadLastPollTime(p.dataDir)
	if err != nil {
		return nil, fmt.Errorf("load last poll time: %w", err)
	}

	entries, err := PollDatabaseContext(ctx, p.dbPath, lastPoll)
	if err != nil {
		return nil, fmt.Errorf("poll database: %w", err)
	}

	if len(entries) == 0 {
		return nil, nil
	}

	fmt.Printf("Wispr Flow: found %d new transcription(s)\n", len(entries))

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
		if err := SaveLastPollTime(p.dataDir, nextPollTime); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to save poll timestamp: %v\n", err)
		}
	}

	storedCount := len(result)
	if storedCount > 0 {
		fmt.Printf("Wispr Flow: prepared %d event(s) for storage\n", storedCount)
	}

	return result, nil
}

func (p *Poller) ShouldRetryOnError(err error) bool {
	return !strings.Contains(err.Error(), "UNIQUE constraint failed")
}
