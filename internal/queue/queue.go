package queue

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"devlog/internal/events"
)

// Queue manages local event queueing when daemon is unavailable
type Queue struct {
	dir string
}

// New creates a new Queue instance
func New(queueDir string) (*Queue, error) {
	if err := os.MkdirAll(queueDir, 0755); err != nil {
		return nil, fmt.Errorf("create queue directory: %w", err)
	}

	return &Queue{
		dir: queueDir,
	}, nil
}

// Enqueue adds an event to the queue
func (q *Queue) Enqueue(event *events.Event) error {
	// Generate filename: timestamp-id.json
	filename := fmt.Sprintf("%d-%s.json", time.Now().UnixNano(), event.ID)
	path := filepath.Join(q.dir, filename)

	// Serialize event
	data, err := event.ToJSON()
	if err != nil {
		return fmt.Errorf("serialize event: %w", err)
	}

	// Write to file atomically
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("write queue file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename queue file: %w", err)
	}

	return nil
}

// List returns all queued events sorted by timestamp
func (q *Queue) List() ([]*events.Event, error) {
	entries, err := os.ReadDir(q.dir)
	if err != nil {
		return nil, fmt.Errorf("read queue directory: %w", err)
	}

	// Filter and sort .json files
	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)

	// Load events
	var queuedEvents []*events.Event
	for _, filename := range files {
		path := filepath.Join(q.dir, filename)
		data, err := os.ReadFile(path)
		if err != nil {
			// Skip files that can't be read
			continue
		}

		event, err := events.FromJSON(data)
		if err != nil {
			// Skip invalid events
			continue
		}

		queuedEvents = append(queuedEvents, event)
	}

	return queuedEvents, nil
}

// Remove deletes a queued event by ID
func (q *Queue) Remove(eventID string) error {
	entries, err := os.ReadDir(q.dir)
	if err != nil {
		return fmt.Errorf("read queue directory: %w", err)
	}

	// Find file with matching ID
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filename := entry.Name()
		if filepath.Ext(filename) != ".json" {
			continue
		}

		// Check if filename contains the event ID
		if contains(filename, eventID) {
			path := filepath.Join(q.dir, filename)
			if err := os.Remove(path); err != nil {
				return fmt.Errorf("remove queue file: %w", err)
			}
			return nil
		}
	}

	return fmt.Errorf("event not found in queue: %s", eventID)
}

// Count returns the number of queued events
func (q *Queue) Count() (int, error) {
	entries, err := os.ReadDir(q.dir)
	if err != nil {
		return 0, fmt.Errorf("read queue directory: %w", err)
	}

	count := 0
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			count++
		}
	}

	return count, nil
}

// Clear removes all queued events
func (q *Queue) Clear() error {
	entries, err := os.ReadDir(q.dir)
	if err != nil {
		return fmt.Errorf("read queue directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		path := filepath.Join(q.dir, entry.Name())
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("remove queue file: %w", err)
		}
	}

	return nil
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(substr) > 0 && len(s) >= len(substr) &&
		(s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
