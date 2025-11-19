package wisprflow

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"devlog/internal/install"
	"devlog/internal/modules"
	"devlog/internal/poller"
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

func (m *Module) Install(ctx *install.Context) error {
	ctx.Log("Installing Wispr Flow integration...")

	dbPath := filepath.Join(ctx.HomeDir, "Library", "Application Support", "Wispr Flow", "flow.sqlite")
	if _, err := os.Stat(dbPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("database for WisprFlow not found at %s. Is WisprFlow installed?", dbPath)
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

	stateMgr, err := state.NewManager(ctx.DataDir)
	if err != nil {
		return fmt.Errorf("failed to initialize state manager: %w", err)
	}

	currentTime := time.Now().Format(time.RFC3339Nano)
	if err := stateMgr.Set("wisprflow", "last_poll_time", currentTime); err != nil {
		return fmt.Errorf("failed to initialize poll timestamp: %w", err)
	}

	ctx.Log("✓ Initialized polling timestamp - will only capture new entries")
	ctx.Log("")
	ctx.Log("Wispr Flow integration installed successfully!")
	ctx.Log("")
	ctx.Log("The module will poll the database for new speech-to-text entries.")

	return nil
}

func (m *Module) Uninstall(ctx *install.Context) error {
	ctx.Log("Uninstalling Wispr Flow integration...")

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
		if interval < 1 || interval > 3600 {
			return fmt.Errorf("poll_interval_seconds must be between 1 and 3600")
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

	rows, err := db.QueryContext(ctx, query, since.UTC().Format("2006-01-02 15:04:05.999 -07:00"))
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

func (m *Module) CreatePoller(config map[string]interface{}, dataDir string) (poller.Poller, error) {
	pollInterval := 60.0
	if interval, ok := config["poll_interval_seconds"].(float64); ok {
		pollInterval = interval
	}

	minWords := 0
	if mw, ok := config["min_words"].(float64); ok {
		minWords = int(mw)
	}

	dbPathConfig, _ := config["db_path"].(string)
	homeDir, _ := os.UserHomeDir()
	dbPath := GetDBPath(homeDir, dbPathConfig)

	return NewPoller(dbPath, dataDir, time.Duration(pollInterval)*time.Second, minWords)
}

func init() {
	modules.Register(&Module{})
}
