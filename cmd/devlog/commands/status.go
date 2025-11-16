package commands

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"devlog/cmd/devlog/formatting"
	"devlog/internal/config"
	"devlog/internal/daemon"
	"devlog/internal/queue"
	"devlog/internal/storage"
)

func init() {
	RegisterCommand("status", &CommandDefinition{
		Name:        "status",
		Description: "Show recent events from the devlog",
		Usage:       "devlog status [flags]",
		Flags: []FlagDefinition{
			{Short: "-v", Long: "--verbose", Description: "Show detailed event information"},
			{Short: "-n", Long: "--number", Description: "Number of events to display (default: 10)"},
			{Short: "-s", Long: "--source", Description: "Filter events by source module"},
		},
		Examples: []string{
			"devlog status",
			"devlog status -v",
			"devlog status -n 20",
			"devlog status -s git",
			"devlog status -v -n 5 -s shell",
		},
	})

	RegisterCommand("flush", &CommandDefinition{
		Name:        "flush",
		Description: "Process all queued events (developer/debug command)",
		Usage:       "devlog flush",
		Examples: []string{
			"devlog flush",
		},
	})
}

func Status(verbose bool, limit int, source string) error {
	_, err := config.Load()
	if err != nil {
		return err
	}

	dataDir, err := config.DataDir()
	if err != nil {
		return err
	}

	dbPath := filepath.Join(dataDir, "events.db")

	store, err := storage.New(dbPath)
	if err != nil {
		return err
	}
	defer store.Close()

	if limit <= 0 {
		limit = 10
	}

	recentEvents, err := store.ListEvents(limit, source)
	if err != nil {
		return err
	}

	if len(recentEvents) == 0 {
		if source != "" {
			fmt.Printf("No events from source '%s'\n", source)
		} else {
			fmt.Println("No events yet")
		}
		return nil
	}

	if source != "" {
		fmt.Printf("Recent events from source '%s' (showing last %d):\n\n", source, len(recentEvents))
	} else {
		fmt.Printf("Recent events (showing last %d):\n\n", len(recentEvents))
	}

	for _, event := range recentEvents {
		if verbose {
			formatting.FormatEventVerbose(event)
		} else {
			formatting.FormatEvent(event)
		}
	}

	count, _ := store.Count()
	fmt.Printf("\nTotal events: %d\n", count)

	return nil
}

func Flush() error {
	if !daemon.IsRunning() {
		return fmt.Errorf("daemon is not running (start it with 'devlog daemon start')")
	}

	queueDir, err := config.QueueDir()
	if err != nil {
		return fmt.Errorf("get queue directory: %w", err)
	}

	q, err := queue.New(queueDir)
	if err != nil {
		return fmt.Errorf("open queue: %w", err)
	}

	queuedEvents, err := q.List()
	if err != nil {
		return fmt.Errorf("list queue: %w", err)
	}

	if len(queuedEvents) == 0 {
		fmt.Println("No queued events")
		return nil
	}

	fmt.Printf("Processing %d queued event(s)...\n", len(queuedEvents))

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	successCount := 0
	for _, event := range queuedEvents {
		eventJSON, err := event.ToJSON()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to serialize event %s: %v\n", event.ID, err)
			continue
		}

		url := fmt.Sprintf("http://127.0.0.1:%d/api/v1/ingest", cfg.HTTP.Port)
		resp, err := http.Post(url, "application/json", bytes.NewReader(eventJSON))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to send event %s: %v\n", event.ID, err)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			fmt.Fprintf(os.Stderr, "Warning: daemon rejected event %s (status %d)\n", event.ID, resp.StatusCode)
			continue
		}

		if err := q.Remove(event.ID); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to remove event %s from queue: %v\n", event.ID, err)
		} else {
			successCount++
		}
	}

	fmt.Printf("Successfully processed %d/%d events\n", successCount, len(queuedEvents))

	return nil
}
