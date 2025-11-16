package queue

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"devlog/internal/events"
)

type Queue struct {
	dir string
}

func New(queueDir string) (*Queue, error) {
	if err := os.MkdirAll(queueDir, 0755); err != nil {
		return nil, fmt.Errorf("create queue directory: %w", err)
	}

	q := &Queue{
		dir: queueDir,
	}

	q.cleanupOrphanedTmpFiles()

	return q, nil
}

func (q *Queue) Enqueue(event *events.Event) error {
	filename := fmt.Sprintf("%d-%s.json", time.Now().UnixNano(), event.ID)
	path := filepath.Join(q.dir, filename)

	data, err := event.ToJSON()
	if err != nil {
		return fmt.Errorf("serialize event: %w", err)
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("write queue file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename queue file: %w", err)
	}

	return nil
}

func (q *Queue) cleanupOrphanedTmpFiles() {
	entries, err := os.ReadDir(q.dir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".tmp" {
			path := filepath.Join(q.dir, entry.Name())
			_ = os.Remove(path)
		}
	}
}

func (q *Queue) List() ([]*events.Event, error) {
	entries, err := os.ReadDir(q.dir)
	if err != nil {
		return nil, fmt.Errorf("read queue directory: %w", err)
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)

	var queuedEvents []*events.Event
	for _, filename := range files {
		path := filepath.Join(q.dir, filename)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		event, err := events.FromJSON(data)
		if err != nil {
			continue
		}

		queuedEvents = append(queuedEvents, event)
	}

	return queuedEvents, nil
}

func (q *Queue) Remove(eventID string) error {
	entries, err := os.ReadDir(q.dir)
	if err != nil {
		return fmt.Errorf("read queue directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filename := entry.Name()
		if filepath.Ext(filename) != ".json" {
			continue
		}

		if strings.Contains(filename, eventID) {
			path := filepath.Join(q.dir, filename)
			if err := os.Remove(path); err != nil {
				return fmt.Errorf("remove queue file: %w", err)
			}
			return nil
		}
	}

	return fmt.Errorf("event not found in queue: %s", eventID)
}

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
