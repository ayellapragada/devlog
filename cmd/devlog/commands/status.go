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

	"github.com/urfave/cli/v2"
)

func StatusCommand() *cli.Command {
	return &cli.Command{
		Name:  "status",
		Usage: "Show recent events from the devlog",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "verbose",
				Aliases: []string{"v"},
				Usage:   "Show detailed event information",
			},
			&cli.IntFlag{
				Name:    "number",
				Aliases: []string{"n"},
				Value:   10,
				Usage:   "Number of events to display",
			},
			&cli.StringFlag{
				Name:    "source",
				Aliases: []string{"s"},
				Usage:   "Filter events by source module",
			},
			&cli.BoolFlag{
				Name:    "metrics",
				Aliases: []string{"m"},
				Usage:   "Show metrics instead of events",
			},
		},
		Action: func(c *cli.Context) error {
			if c.Bool("metrics") {
				return StatusMetrics()
			}
			return Status(c.Bool("verbose"), c.Int("number"), c.String("source"))
		},
	}
}

func FlushCommand() *cli.Command {
	return &cli.Command{
		Name:   "flush",
		Usage:  "Process all queued events (developer/debug command)",
		Hidden: true,
		Action: func(c *cli.Context) error {
			return Flush()
		},
	}
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

	recentEvents, err := store.QueryEvents(storage.QueryOptions{
		Limit:  limit,
		Source: source,
	})
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

	for i := len(recentEvents) - 1; i >= 0; i-- {
		event := recentEvents[i]
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

func StatusMetrics() error {
	if !daemon.IsRunning() {
		return fmt.Errorf("daemon is not running")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	url := fmt.Sprintf("http://127.0.0.1:%d/api/v1/metrics?summary=true", cfg.HTTP.Port)

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to connect to daemon: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("daemon returned error: %d", resp.StatusCode)
	}

	var body bytes.Buffer
	body.ReadFrom(resp.Body)

	fmt.Println("DevLog Metrics")
	fmt.Println("==============")
	fmt.Println()
	fmt.Println(body.String())

	return nil
}
