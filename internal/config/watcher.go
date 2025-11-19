package config

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

type Watcher struct {
	configPath    string
	watcher       *fsnotify.Watcher
	onChange      func(*Config)
	logger        *slog.Logger
	debounce      time.Duration
	timerMu       sync.Mutex
	debounceTimer *time.Timer
	pendingReload bool
}

func NewWatcher(configPath string, onChange func(*Config), logger *slog.Logger) (*Watcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("create file watcher: %w", err)
	}

	configDir := filepath.Dir(configPath)
	if err := fsWatcher.Add(configDir); err != nil {
		fsWatcher.Close()
		return nil, fmt.Errorf("watch config directory: %w", err)
	}

	return &Watcher{
		configPath: configPath,
		watcher:    fsWatcher,
		onChange:   onChange,
		logger:     logger,
		debounce:   500 * time.Millisecond,
	}, nil
}

func (w *Watcher) Start(ctx context.Context) error {
	defer w.watcher.Close()
	defer w.stopTimer()

	for {
		select {
		case <-ctx.Done():
			return nil

		case event, ok := <-w.watcher.Events:
			if !ok {
				return nil
			}

			if event.Name != w.configPath {
				continue
			}

			if event.Op&(fsnotify.Write|fsnotify.Create) == 0 {
				continue
			}

			w.scheduleReload()

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return nil
			}
			w.logger.Error("config watcher error", "error", err)
		}
	}
}

func (w *Watcher) scheduleReload() {
	w.timerMu.Lock()
	defer w.timerMu.Unlock()

	if w.debounceTimer != nil {
		w.debounceTimer.Stop()
	}

	w.pendingReload = true
	w.debounceTimer = time.AfterFunc(w.debounce, func() {
		w.timerMu.Lock()
		shouldReload := w.pendingReload
		w.pendingReload = false
		w.timerMu.Unlock()

		if shouldReload {
			w.reloadConfig()
		}
	})
}

func (w *Watcher) stopTimer() {
	w.timerMu.Lock()
	defer w.timerMu.Unlock()

	if w.debounceTimer != nil {
		w.debounceTimer.Stop()
		w.debounceTimer = nil
	}
	w.pendingReload = false
}

func (w *Watcher) reloadConfig() {
	w.logger.Info("config file changed, reloading", "path", w.configPath)

	cfg, err := Load()
	if err != nil {
		w.logger.Error("failed to load config", "error", err, "path", w.configPath)
		return
	}

	if err := cfg.Validate(); err != nil {
		w.logger.Error("invalid config, not applying changes", "error", err, "path", w.configPath)
		return
	}

	w.logger.Info("config reloaded successfully")
	w.onChange(cfg)
}

func (w *Watcher) Close() error {
	return w.watcher.Close()
}
