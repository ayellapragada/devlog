package queue

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"devlog/internal/errors"
	"devlog/internal/events"

	"github.com/google/uuid"
)

type Queue struct {
	dir string
}

func New(queueDir string) (*Queue, error) {
	if err := os.MkdirAll(queueDir, 0755); err != nil {
		return nil, errors.WrapQueue("create directory", err)
	}

	q := &Queue{
		dir: queueDir,
	}

	q.cleanupOrphanedTmpFiles()

	return q, nil
}

func (q *Queue) Enqueue(event *events.Event) error {
	filename := fmt.Sprintf("event_%s.json", uuid.New().String())
	path := filepath.Join(q.dir, filename)

	data, err := event.ToJSON()
	if err != nil {
		return errors.WrapQueue("serialize event", err)
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return errors.WrapQueue("write file", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return errors.WrapQueue("rename file", err)
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
		return nil, errors.WrapQueue("read directory", err)
	}

	type fileInfo struct {
		name    string
		modTime int64
	}
	var files []fileInfo

	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			files = append(files, fileInfo{
				name:    entry.Name(),
				modTime: info.ModTime().UnixNano(),
			})
		}
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime < files[j].modTime
	})

	var queuedEvents []*events.Event
	for _, file := range files {
		path := filepath.Join(q.dir, file.name)
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

		path := filepath.Join(q.dir, filename)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		event, err := events.FromJSON(data)
		if err != nil {
			continue
		}

		if event.ID == eventID {
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
