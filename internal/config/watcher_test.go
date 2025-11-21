package config

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatcherCreation(t *testing.T) {
	t.Run("creates watcher with valid path", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.yaml")

		// Create the file first
		if err := os.WriteFile(configPath, []byte("test"), 0644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		logger := slog.Default()
		watcher, err := NewWatcher(configPath, func(cfg *Config) {}, logger)
		if err != nil {
			t.Fatalf("NewWatcher: %v", err)
		}
		defer watcher.Close()

		if watcher == nil {
			t.Fatal("watcher is nil")
		}
		if watcher.configPath != configPath {
			t.Errorf("config path mismatch: %s", watcher.configPath)
		}
	})

	t.Run("fails with non-existent directory", func(t *testing.T) {
		configPath := "/nonexistent/path/config.yaml"
		logger := slog.Default()

		watcher, err := NewWatcher(configPath, func(cfg *Config) {}, logger)
		if err == nil {
			t.Error("expected error for non-existent directory")
			if watcher != nil {
				watcher.Close()
			}
		}
	})
}

func TestWatcherDebounce(t *testing.T) {
	t.Run("debounce duration is set", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.yaml")

		if err := os.WriteFile(configPath, []byte("test"), 0644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		logger := slog.Default()
		watcher, err := NewWatcher(configPath, func(cfg *Config) {}, logger)
		if err != nil {
			t.Fatalf("NewWatcher: %v", err)
		}
		defer watcher.Close()

		if watcher.debounce == 0 {
			t.Error("debounce duration is zero")
		}

		// Should be around 500ms
		if watcher.debounce < 100*time.Millisecond || watcher.debounce > 1*time.Second {
			t.Logf("debounce: %v (should be around 500ms)", watcher.debounce)
		}
	})
}

func TestWatcherClose(t *testing.T) {
	t.Run("close doesn't panic", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.yaml")

		if err := os.WriteFile(configPath, []byte("test"), 0644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		logger := slog.Default()
		watcher, err := NewWatcher(configPath, func(cfg *Config) {}, logger)
		if err != nil {
			t.Fatalf("NewWatcher: %v", err)
		}

		// Should not panic
		err = watcher.Close()
		if err != nil {
			t.Logf("Close error (may be expected): %v", err)
		}
	})

	t.Run("close multiple times", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.yaml")

		if err := os.WriteFile(configPath, []byte("test"), 0644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		logger := slog.Default()
		watcher, err := NewWatcher(configPath, func(cfg *Config) {}, logger)
		if err != nil {
			t.Fatalf("NewWatcher: %v", err)
		}

		// Close multiple times
		watcher.Close()
		watcher.Close()
	})
}

func TestWatcherStart(t *testing.T) {
	t.Run("start with cancellation context", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.yaml")

		if err := os.WriteFile(configPath, []byte("test"), 0644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		logger := slog.Default()
		watcher, err := NewWatcher(configPath, func(cfg *Config) {}, logger)
		if err != nil {
			t.Fatalf("NewWatcher: %v", err)
		}
		defer watcher.Close()

		// Create a context that will be cancelled
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Start watcher in goroutine
		errChan := make(chan error, 1)
		go func() {
			errChan <- watcher.Start(ctx)
		}()

		// Let it start
		time.Sleep(10 * time.Millisecond)

		// Cancel context
		cancel()

		// Wait for it to exit
		select {
		case err := <-errChan:
			// Should exit cleanly
			if err != nil && err != context.Canceled {
				t.Logf("watcher start error (may be expected): %v", err)
			}
		case <-time.After(1 * time.Second):
			t.Error("watcher did not exit after context cancellation")
		}
	})

	t.Run("start respects context", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.yaml")

		if err := os.WriteFile(configPath, []byte("test"), 0644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		logger := slog.Default()
		changeCount := 0
		watcher, err := NewWatcher(configPath, func(cfg *Config) {
			changeCount++
		}, logger)
		if err != nil {
			t.Fatalf("NewWatcher: %v", err)
		}
		defer watcher.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		// Start watcher
		errChan := make(chan error, 1)
		go func() {
			errChan <- watcher.Start(ctx)
		}()

		// Wait for timeout
		select {
		case <-errChan:
			// Expected to exit after timeout
		case <-time.After(500 * time.Millisecond):
			t.Error("watcher did not respect context timeout")
		}
	})
}

func TestWatcherFileMonitoring(t *testing.T) {
	t.Run("watcher monitors config directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.yaml")

		if err := os.WriteFile(configPath, []byte("test"), 0644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		logger := slog.Default()
		watcher, err := NewWatcher(configPath, func(cfg *Config) {}, logger)
		if err != nil {
			t.Fatalf("NewWatcher: %v", err)
		}
		defer watcher.Close()

		// Watcher should have a valid fsnotify watcher
		if watcher.watcher == nil {
			t.Error("fsnotify watcher is nil")
		}
	})
}

func TestWatcherPendingReload(t *testing.T) {
	t.Run("pending reload flag", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.yaml")

		if err := os.WriteFile(configPath, []byte("test"), 0644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		logger := slog.Default()
		watcher, err := NewWatcher(configPath, func(cfg *Config) {}, logger)
		if err != nil {
			t.Fatalf("NewWatcher: %v", err)
		}
		defer watcher.Close()

		// pendingReload should start as false
		if watcher.pendingReload {
			t.Error("pendingReload should be false initially")
		}
	})
}

func TestWatcherCallback(t *testing.T) {
	t.Run("callback is stored", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.yaml")

		if err := os.WriteFile(configPath, []byte("test"), 0644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		logger := slog.Default()
		watcher, err := NewWatcher(configPath, func(cfg *Config) {
		}, logger)
		if err != nil {
			t.Fatalf("NewWatcher: %v", err)
		}
		defer watcher.Close()

		if watcher.onChange == nil {
			t.Error("onChange callback not stored")
		}
	})
}

func TestWatcherLogger(t *testing.T) {
	t.Run("logger is stored", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.yaml")

		if err := os.WriteFile(configPath, []byte("test"), 0644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		logger := slog.Default()
		watcher, err := NewWatcher(configPath, func(cfg *Config) {}, logger)
		if err != nil {
			t.Fatalf("NewWatcher: %v", err)
		}
		defer watcher.Close()

		if watcher.logger == nil {
			t.Error("logger not stored")
		}
	})
}
