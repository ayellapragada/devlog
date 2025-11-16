package wisprflow

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"devlog/internal/modules"
	"devlog/internal/state"

	_ "modernc.org/sqlite"
)

type Module struct{}

func (m *Module) Name() string {
	return "wisprflow"
}

func (m *Module) Description() string {
	return "Capture speech-to-text entries from Wispr Flow"
}

func (m *Module) Install(ctx *modules.InstallContext) error {
	ctx.Log("Installing Wispr Flow integration...")

	dbPath := filepath.Join(ctx.HomeDir, "Library", "Application Support", "Wispr Flow", "flow.sqlite")
	if _, err := os.Stat(dbPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("Wispr Flow database not found at %s. Is Wispr Flow installed?", dbPath)
		}
		return fmt.Errorf("error checking Wispr Flow database: %w", err)
	}

	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?mode=ro", dbPath))
	if err != nil {
		return fmt.Errorf("failed to open Wispr Flow database: %w", err)
	}
	defer db.Close()

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM History WHERE (formattedText IS NOT NULL OR asrText IS NOT NULL) AND numWords > 0").Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to query Wispr Flow database: %w", err)
	}

	ctx.Log("✓ Found Wispr Flow database at %s", dbPath)
	ctx.Log("✓ Database has %d valid history entries", count)
	ctx.Log("")

	// Initialize the last poll timestamp to now
	// This ensures we only capture new entries going forward
	timestampFile := getTimestampFilePath(ctx.DataDir)
	if err := saveLastPollTime(timestampFile, time.Now()); err != nil {
		return fmt.Errorf("failed to initialize timestamp: %w", err)
	}

	ctx.Log("✓ Initialized polling timestamp - will only capture new entries")
	ctx.Log("")
	ctx.Log("Wispr Flow integration installed successfully!")
	ctx.Log("")
	ctx.Log("The module will poll the database for new speech-to-text entries.")

	return nil
}

func (m *Module) Uninstall(ctx *modules.InstallContext) error {
	ctx.Log("Uninstalling Wispr Flow integration...")

	timestampFile := getTimestampFilePath(ctx.DataDir)
	if _, err := os.Stat(timestampFile); err == nil {
		if err := os.Remove(timestampFile); err != nil {
			ctx.Log("Warning: failed to remove old timestamp: %v", err)
		} else {
			ctx.Log("✓ Removed old timestamp")
		}
	}

	stateMgr, err := state.NewManager(ctx.DataDir)
	if err != nil {
		ctx.Log("Warning: failed to clean up state: %v", err)
	} else {
		if err := stateMgr.DeleteModule("wisprflow"); err != nil {
			ctx.Log("Warning: failed to clean up state: %v", err)
		} else {
			ctx.Log("✓ Cleaned up wisprflow state")
		}
	}

	ctx.Log("✓ Wispr Flow integration uninstalled")
	return nil
}

func (m *Module) DefaultConfig() interface{} {
	return map[string]interface{}{
		"poll_interval_seconds": 60,
		"db_path":               "~/Library/Application Support/Wispr Flow/flow.sqlite",
		"min_words":             1,
		"include_fields": []string{
			"transcriptEntityId",
			"asrText",
			"formattedText",
			"editedText",
			"timestamp",
			"status",
			"app",
			"url",
			"duration",
			"numWords",
		},
	}
}

func (m *Module) ValidateConfig(config interface{}) error {
	cfg, ok := config.(map[string]interface{})
	if !ok {
		return fmt.Errorf("config must be a map")
	}

	if interval, ok := cfg["poll_interval_seconds"].(float64); ok {
		if interval < 1 {
			return fmt.Errorf("poll_interval_seconds must be at least 1")
		}
	}

	if minWords, ok := cfg["min_words"].(float64); ok {
		if minWords < 0 {
			return fmt.Errorf("min_words must be non-negative")
		}
	}

	return nil
}

func GetDBPath(homeDir string, configPath string) string {
	if configPath == "" {
		configPath = "~/Library/Application Support/Wispr Flow/flow.sqlite"
	}

	if len(configPath) > 0 && configPath[0] == '~' {
		configPath = filepath.Join(homeDir, configPath[1:])
	}

	return configPath
}

type HistoryEntry struct {
	TranscriptEntityID string
	ASRText            string
	FormattedText      string
	EditedText         string
	Timestamp          time.Time
	Status             string
	App                string
	URL                string
	Duration           float64
	NumWords           int
}

func PollDatabase(dbPath string, since time.Time) ([]HistoryEntry, error) {
	return PollDatabaseContext(context.Background(), dbPath, since)
}

func PollDatabaseContext(ctx context.Context, dbPath string, since time.Time) ([]HistoryEntry, error) {
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?mode=ro", dbPath))
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	query := `
		SELECT
			transcriptEntityId,
			COALESCE(asrText, ''),
			COALESCE(formattedText, ''),
			COALESCE(editedText, ''),
			COALESCE(timestamp, ''),
			COALESCE(status, ''),
			COALESCE(app, ''),
			COALESCE(url, ''),
			COALESCE(duration, 0),
			COALESCE(numWords, 0)
		FROM History
		WHERE timestamp > ?
			AND (formattedText IS NOT NULL OR asrText IS NOT NULL)
			AND numWords > 0
		ORDER BY timestamp ASC
	`

	rows, err := db.QueryContext(ctx, query, since.Format("2006-01-02 15:04:05.999 -07:00"))
	if err != nil {
		return nil, fmt.Errorf("failed to query database: %w", err)
	}
	defer rows.Close()

	var entries []HistoryEntry
	for rows.Next() {
		var entry HistoryEntry
		var timestampStr string

		err := rows.Scan(
			&entry.TranscriptEntityID,
			&entry.ASRText,
			&entry.FormattedText,
			&entry.EditedText,
			&timestampStr,
			&entry.Status,
			&entry.App,
			&entry.URL,
			&entry.Duration,
			&entry.NumWords,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		if timestampStr != "" {
			entry.Timestamp, err = time.Parse("2006-01-02 15:04:05.999 -07:00", timestampStr)
			if err != nil {
				entry.Timestamp, err = time.Parse("2006-01-02 15:04:05", timestampStr)
				if err != nil {
					continue
				}
			}
		} else {
			continue
		}

		entries = append(entries, entry)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return entries, nil
}

// getTimestampFilePath returns the path to the file storing the last poll timestamp
func getTimestampFilePath(dataDir string) string {
	return filepath.Join(dataDir, "wisprflow_last_poll.txt")
}

// saveLastPollTime saves the last poll timestamp to a file
func saveLastPollTime(path string, t time.Time) error {
	// Ensure the directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	// Use RFC3339Nano to preserve millisecond precision
	timestamp := t.Format(time.RFC3339Nano)
	if err := os.WriteFile(path, []byte(timestamp), 0644); err != nil {
		return fmt.Errorf("write timestamp file: %w", err)
	}
	return nil
}

// LoadLastPollTime loads the last poll timestamp from a file
// If the file doesn't exist, returns time.Now() (only capture new entries)
func LoadLastPollTime(dataDir string) (time.Time, error) {
	path := getTimestampFilePath(dataDir)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// If no timestamp file exists, start from now
			return time.Now(), nil
		}
		return time.Time{}, fmt.Errorf("read timestamp file: %w", err)
	}

	// Try RFC3339Nano first (with nanoseconds), fall back to RFC3339 (without)
	t, err := time.Parse(time.RFC3339Nano, string(data))
	if err != nil {
		t, err = time.Parse(time.RFC3339, string(data))
		if err != nil {
			return time.Time{}, fmt.Errorf("parse timestamp: %w", err)
		}
	}

	return t, nil
}

// SaveLastPollTime is the public version of saveLastPollTime
func SaveLastPollTime(dataDir string, t time.Time) error {
	return saveLastPollTime(getTimestampFilePath(dataDir), t)
}

func init() {
	modules.Register(&Module{})
}
