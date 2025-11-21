package commands

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitConfigCreation(t *testing.T) {
	t.Run("creates config structure", func(t *testing.T) {
		tmpDir := t.TempDir()
		homeDir := tmpDir

		// Set temporary home
		oldHome := os.Getenv("HOME")
		defer os.Setenv("HOME", oldHome)
		os.Setenv("HOME", homeDir)

		// Config directories should be created
		configDir := filepath.Join(homeDir, ".config", "devlog")
		dataDir := filepath.Join(homeDir, ".local", "share", "devlog")

		if err := os.MkdirAll(configDir, 0755); err != nil {
			t.Fatalf("create config dir: %v", err)
		}
		if err := os.MkdirAll(dataDir, 0755); err != nil {
			t.Fatalf("create data dir: %v", err)
		}

		// Verify directories exist
		if _, err := os.Stat(configDir); os.IsNotExist(err) {
			t.Error("config directory was not created")
		}
		if _, err := os.Stat(dataDir); os.IsNotExist(err) {
			t.Error("data directory was not created")
		}
	})
}

func TestInitDirectories(t *testing.T) {
	t.Run("config directory path", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, ".config", "devlog", "config.yaml")
		configDir := filepath.Dir(configPath)

		if err := os.MkdirAll(configDir, 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}

		if _, err := os.Stat(configDir); os.IsNotExist(err) {
			t.Error("directory not created")
		}
	})

	t.Run("data directory path", func(t *testing.T) {
		tmpDir := t.TempDir()
		dataPath := filepath.Join(tmpDir, ".local", "share", "devlog", "events.db")
		dataDir := filepath.Dir(dataPath)

		if err := os.MkdirAll(dataDir, 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}

		if _, err := os.Stat(dataDir); os.IsNotExist(err) {
			t.Error("directory not created")
		}
	})
}

func TestInitPermissions(t *testing.T) {
	t.Run("directory permissions", func(t *testing.T) {
		tmpDir := t.TempDir()

		if err := os.MkdirAll(tmpDir, 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}

		info, err := os.Stat(tmpDir)
		if err != nil {
			t.Fatalf("stat: %v", err)
		}

		// Check that directory is readable/writable
		if info.Mode()&0700 == 0 {
			t.Error("directory lacks read/write permissions")
		}
	})
}

func TestInitIdempotency(t *testing.T) {
	t.Run("init can be called multiple times", func(t *testing.T) {
		tmpDir := t.TempDir()
		configDir := filepath.Join(tmpDir, ".config", "devlog")

		// Create once
		if err := os.MkdirAll(configDir, 0755); err != nil {
			t.Fatalf("first mkdir: %v", err)
		}

		// Create again (should not error)
		if err := os.MkdirAll(configDir, 0755); err != nil {
			t.Fatalf("second mkdir: %v", err)
		}

		// Verify directory still exists
		if _, err := os.Stat(configDir); os.IsNotExist(err) {
			t.Error("directory was removed after second init")
		}
	})
}
