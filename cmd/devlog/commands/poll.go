package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"devlog/internal/config"
	"devlog/internal/events"
	"devlog/internal/storage"
	"devlog/modules/wisprflow"
)

func Poll() error {
	if len(os.Args) < 3 {
		fmt.Println("Usage: devlog poll <module>")
		fmt.Println()
		fmt.Println("Available modules:")
		fmt.Println("  wisprflow  - Poll Wispr Flow database for new transcriptions")
		return fmt.Errorf("missing module name")
	}

	moduleName := os.Args[2]

	switch moduleName {
	case "wisprflow":
		return pollWisprFlow()
	default:
		return fmt.Errorf("unknown module: %s (only 'wisprflow' is currently supported)", moduleName)
	}
}

func pollWisprFlow() error {
	fmt.Println("Polling Wispr Flow database...")

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if !cfg.IsModuleEnabled("wisprflow") {
		return fmt.Errorf("wisprflow module is not enabled (run 'devlog module install wisprflow' first)")
	}

	modCfg, ok := cfg.GetModuleConfig("wisprflow")
	if !ok {
		return fmt.Errorf("wisprflow module config not found")
	}

	dataDir, err := config.DataDir()
	if err != nil {
		return fmt.Errorf("get data directory: %w", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home directory: %w", err)
	}

	dbPathConfig, _ := modCfg["db_path"].(string)
	dbPath := wisprflow.GetDBPath(homeDir, dbPathConfig)

	fmt.Printf("Database: %s\n", dbPath)

	lastPoll, err := wisprflow.LoadLastPollTime(dataDir)
	if err != nil {
		return fmt.Errorf("load last poll time: %w", err)
	}

	fmt.Printf("Last poll: %s\n", lastPoll.Format(time.RFC3339Nano))
	fmt.Println()

	entries, err := wisprflow.PollDatabase(dbPath, lastPoll)
	if err != nil {
		return fmt.Errorf("poll database: %w", err)
	}

	if len(entries) == 0 {
		fmt.Println("No new entries found")
		fmt.Println()
		fmt.Println("To test:")
		fmt.Println("  1. Create a new transcription in Wispr Flow")
		fmt.Println("  2. Run 'devlog poll wisprflow' again")
		return nil
	}

	fmt.Printf("Found %d new entries:\n\n", len(entries))

	dbPath2 := filepath.Join(dataDir, "events.db")

	store, err := storage.New(dbPath2)
	if err != nil {
		return fmt.Errorf("open storage: %w", err)
	}
	defer store.Close()

	minWords := 0.0
	if mw, ok := modCfg["min_words"].(float64); ok {
		minWords = mw
	}

	storedCount := 0
	for i, entry := range entries {
		if minWords > 0 && float64(entry.NumWords) < minWords {
			continue
		}

		text := entry.EditedText
		if text == "" {
			text = entry.FormattedText
		}
		if text == "" {
			text = entry.ASRText
		}

		fmt.Printf("[%d] %s\n", i+1, entry.Timestamp.Format("2006-01-02 15:04:05"))
		displayText := text
		if len(displayText) > 100 {
			displayText = displayText[:100] + "..."
		}
		fmt.Printf("    Text: %s\n", displayText)
		if entry.App != "" {
			fmt.Printf("    App: %s\n", entry.App)
		}
		fmt.Printf("    Words: %d, Duration: %.1fs\n", entry.NumWords, entry.Duration)
		fmt.Println()

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

		if err := store.InsertEvent(event); err != nil {
			if !strings.Contains(err.Error(), "UNIQUE constraint failed") {
				fmt.Fprintf(os.Stderr, "Warning: failed to store event: %v\n", err)
			}
		} else {
			storedCount++
		}
	}

	if len(entries) > 0 {
		lastEntry := entries[len(entries)-1]
		nextPollTime := lastEntry.Timestamp.Add(1 * time.Millisecond)
		if err := wisprflow.SaveLastPollTime(dataDir, nextPollTime); err != nil {
			return fmt.Errorf("save last poll time: %w", err)
		}
		fmt.Printf("✓ Updated last poll timestamp to %s\n", nextPollTime.Format(time.RFC3339Nano))
	}

	if storedCount > 0 {
		fmt.Printf("✓ Stored %d new event(s) in database\n", storedCount)
	} else {
		fmt.Println("✓ No new events to store (all entries already captured)")
	}
	fmt.Println()
	fmt.Println("Run 'devlog status' to see the events")

	return nil
}
