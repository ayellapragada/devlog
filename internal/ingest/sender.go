package ingest

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"devlog/internal/config"
	"devlog/internal/daemon"
	"devlog/internal/events"
	"devlog/internal/queue"
)

func SendEvent(event *events.Event) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if daemon.IsRunning() {
		eventJSON, err := event.ToJSON()
		if err != nil {
			return fmt.Errorf("serialize event: %w", err)
		}

		url := fmt.Sprintf("http://127.0.0.1:%d/api/v1/ingest", cfg.HTTP.Port)
		resp, err := http.Post(url, "application/json", bytes.NewReader(eventJSON))
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
	}

	queueDir, err := config.QueueDir()
	if err != nil {
		return fmt.Errorf("get queue directory: %w", err)
	}

	q, err := queue.New(queueDir)
	if err != nil {
		return fmt.Errorf("create queue: %w", err)
	}

	if err := q.Enqueue(event); err != nil {
		return fmt.Errorf("queue event: %w", err)
	}

	return nil
}

func FindGitRepo(path string) (string, error) {
	current := path
	for {
		gitPath := filepath.Join(current, ".git")
		if _, err := os.Stat(gitPath); err == nil {
			return current, nil
		}

		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("not a git repository")
		}
		current = parent
	}
}
