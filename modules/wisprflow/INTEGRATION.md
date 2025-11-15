# Daemon Integration Guide

This document explains how the Wispr Flow module should be integrated with the devlog daemon for polling.

## Overview

The daemon should periodically poll the Wispr Flow database for new entries and create events for each transcription. The module provides all the necessary functions to:

1. Load the last poll timestamp
2. Query for new entries
3. Save the updated timestamp

## Integration Pattern

Here's how the daemon should integrate with this module:

```go
package daemon

import (
    "time"
    "devlog/modules/wisprflow"
    "devlog/internal/events"
    "devlog/internal/config"
)

// PollWisprFlow should be called periodically by the daemon
func (d *Daemon) PollWisprFlow() error {
    // Check if module is enabled
    if !d.config.IsModuleEnabled("wisprflow") {
        return nil
    }

    // Get module config
    cfg, ok := d.config.GetModuleConfig("wisprflow")
    if !ok {
        return nil
    }

    // Get configuration values
    dbPath, _ := cfg["db_path"].(string)
    minWords, _ := cfg["min_words"].(float64)

    // Resolve DB path
    homeDir, _ := os.UserHomeDir()
    dbPath = wisprflow.GetDBPath(homeDir, dbPath)

    // Load last poll timestamp
    lastPoll, err := wisprflow.LoadLastPollTime(d.dataDir)
    if err != nil {
        return fmt.Errorf("load last poll time: %w", err)
    }

    // Poll for new entries
    entries, err := wisprflow.PollDatabase(dbPath, lastPoll)
    if err != nil {
        return fmt.Errorf("poll database: %w", err)
    }

    // Create events for each entry
    for _, entry := range entries {
        // Filter by minimum words if configured
        if minWords > 0 && float64(entry.NumWords) < minWords {
            continue
        }

        // Create event
        event := events.NewEvent("wisprflow", "transcription")
        event.Timestamp = entry.Timestamp.Format(time.RFC3339)

        // Prefer editedText > formattedText > asrText
        text := entry.EditedText
        if text == "" {
            text = entry.FormattedText
        }
        if text == "" {
            text = entry.ASRText
        }

        event.Payload = map[string]interface{}{
            "id":              entry.TranscriptEntityID,
            "text":            text,
            "asr_text":        entry.ASRText,
            "formatted_text":  entry.FormattedText,
            "edited_text":     entry.EditedText,
            "app":             entry.App,
            "url":             entry.URL,
            "duration":        entry.Duration,
            "num_words":       entry.NumWords,
            "status":          entry.Status,
        }

        // Store the event
        if err := d.store.SaveEvent(event); err != nil {
            return fmt.Errorf("save event: %w", err)
        }
    }

    // Update timestamp if we got new entries
    if len(entries) > 0 {
        lastEntry := entries[len(entries)-1]
        if err := wisprflow.SaveLastPollTime(d.dataDir, lastEntry.Timestamp); err != nil {
            return fmt.Errorf("save last poll time: %w", err)
        }
    }

    return nil
}
```

## Polling Interval

The daemon should respect the `poll_interval_seconds` configuration value:

```go
// In the daemon's main loop
func (d *Daemon) Run() error {
    // Get polling interval from config
    cfg, _ := d.config.GetModuleConfig("wisprflow")
    pollInterval := 60.0 // default
    if interval, ok := cfg["poll_interval_seconds"].(float64); ok {
        pollInterval = interval
    }

    ticker := time.NewTicker(time.Duration(pollInterval) * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            if err := d.PollWisprFlow(); err != nil {
                log.Printf("Error polling Wispr Flow: %v", err)
            }
        case <-d.stopCh:
            return nil
        }
    }
}
```

## Event Format

Events created by the Wispr Flow module will have:

- **Source**: `wisprflow`
- **Type**: `transcription`
- **Timestamp**: The timestamp from the Wispr Flow database
- **Payload**: Contains all transcription data including:
  - `text`: The best available text (edited > formatted > asr)
  - `asr_text`: Raw ASR output
  - `formatted_text`: Formatted text
  - `edited_text`: User-edited text (if any)
  - `app`: Application where transcription occurred
  - `url`: URL context (if applicable)
  - `duration`: Audio duration in seconds
  - `num_words`: Word count
  - `status`: Transcription status

## Error Handling

The daemon should handle errors gracefully:

1. If the database file is not found, log a warning but don't crash
2. If timestamp file is corrupted, it will default to `time.Now()` (safe fallback)
3. Database read errors should be logged and retried on next poll
4. Invalid entries (unparseable timestamps) are automatically skipped

## Testing

To test the integration without waiting for the poll interval:

1. Install the module: `devlog module install wisprflow`
2. Create a test transcription in Wispr Flow
3. Manually trigger a poll or wait for the interval
4. Check events: `devlog status`
