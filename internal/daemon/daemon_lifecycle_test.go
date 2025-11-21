package daemon

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"devlog/internal/config"
	"devlog/internal/storage"
	"devlog/internal/testutil"
)

func setupTestDaemonWithDir(t *testing.T) (*Daemon, string, func()) {
	tmpDir := t.TempDir()

	// Create config directory
	configDir := filepath.Join(tmpDir, ".config", "devlog")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	// Create data directory
	dataDir := filepath.Join(tmpDir, ".local", "share", "devlog")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("Failed to create data dir: %v", err)
	}

	// Create database
	dbPath := filepath.Join(dataDir, "events.db")
	if err := storage.InitDB(dbPath); err != nil {
		t.Fatalf("Failed to init DB: %v", err)
	}

	store, err := storage.New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	// Create minimal config
	cfg := config.DefaultConfig()
	cfg.HTTP.Port = 0 // Use random port

	daemon := New(cfg, store)

	cleanup := func() {
		store.Close()
		if daemon != nil {
			daemon.Shutdown()
		}
	}

	return daemon, tmpDir, cleanup
}

func TestDaemonCreation(t *testing.T) {
	t.Run("daemon created with config and storage", func(t *testing.T) {
		cfg := config.DefaultConfig()
		store := testutil.NewTestStorage(t)
		defer store.Close()

		daemon := New(cfg, store)
		if daemon == nil {
			t.Fatal("daemon is nil")
		}
		if daemon.config == nil {
			t.Error("daemon config is nil")
		}
		if daemon.storage == nil {
			t.Error("daemon storage is nil")
		}
	})

	t.Run("daemon has required fields", func(t *testing.T) {
		cfg := config.DefaultConfig()
		store := testutil.NewTestStorage(t)
		defer store.Close()

		daemon := New(cfg, store)

		// Should have initialized maps and channels
		daemon.modulesMu.RLock()
		modulesInitialized := daemon.modules != nil
		daemon.modulesMu.RUnlock()

		daemon.pluginsMu.RLock()
		pluginsInitialized := daemon.plugins != nil
		daemon.pluginsMu.RUnlock()

		if !modulesInitialized {
			t.Error("modules map not initialized")
		}
		if !pluginsInitialized {
			t.Error("plugins map not initialized")
		}
	})
}

func TestDaemonShutdown(t *testing.T) {
	t.Run("daemon shutdown doesn't panic", func(t *testing.T) {
		cfg := config.DefaultConfig()
		store := testutil.NewTestStorage(t)
		defer store.Close()

		daemon := New(cfg, store)

		// Should not panic
		daemon.Shutdown()
	})

	t.Run("multiple shutdowns don't panic", func(t *testing.T) {
		cfg := config.DefaultConfig()
		store := testutil.NewTestStorage(t)
		defer store.Close()

		daemon := New(cfg, store)

		// Call shutdown multiple times
		daemon.Shutdown()
		daemon.Shutdown()
		daemon.Shutdown()
	})
}

func TestDaemonConfiguration(t *testing.T) {
	t.Run("daemon stores configuration", func(t *testing.T) {
		cfg := config.DefaultConfig()
		cfg.HTTP.Port = 9999
		store := testutil.NewTestStorage(t)
		defer store.Close()

		daemon := New(cfg, store)

		if daemon.config.HTTP.Port != 9999 {
			t.Errorf("port not set correctly: %d", daemon.config.HTTP.Port)
		}
	})

	t.Run("daemon stores storage reference", func(t *testing.T) {
		cfg := config.DefaultConfig()
		store := testutil.NewTestStorage(t)
		defer store.Close()

		daemon := New(cfg, store)

		// Storage should be accessible
		count, err := daemon.storage.Count()
		if err != nil {
			t.Fatalf("accessing storage: %v", err)
		}
		if count != 0 {
			t.Errorf("expected 0 events in new storage, got %d", count)
		}
	})
}

func TestDaemonModulesRegistry(t *testing.T) {
	t.Run("modules map is accessible", func(t *testing.T) {
		cfg := config.DefaultConfig()
		store := testutil.NewTestStorage(t)
		defer store.Close()

		daemon := New(cfg, store)

		// Should be able to lock and access modules
		daemon.modulesMu.RLock()
		defer daemon.modulesMu.RUnlock()

		if daemon.modules == nil {
			t.Error("modules map is nil")
		}

		// Should start empty
		if len(daemon.modules) != 0 {
			t.Errorf("expected no modules, got %d", len(daemon.modules))
		}
	})

	t.Run("can add module to registry", func(t *testing.T) {
		cfg := config.DefaultConfig()
		store := testutil.NewTestStorage(t)
		defer store.Close()

		daemon := New(cfg, store)

		// Add a module entry
		daemon.modulesMu.Lock()
		daemon.modules["test"] = "test-poller"
		daemon.modulesMu.Unlock()

		// Verify it's there
		daemon.modulesMu.RLock()
		poller, exists := daemon.modules["test"]
		daemon.modulesMu.RUnlock()

		if !exists {
			t.Error("module not added")
		}
		if poller != "test-poller" {
			t.Errorf("wrong poller name: %s", poller)
		}
	})
}

func TestDaemonPluginsRegistry(t *testing.T) {
	t.Run("plugins map is accessible", func(t *testing.T) {
		cfg := config.DefaultConfig()
		store := testutil.NewTestStorage(t)
		defer store.Close()

		daemon := New(cfg, store)

		// Should be able to lock and access plugins
		daemon.pluginsMu.RLock()
		defer daemon.pluginsMu.RUnlock()

		if daemon.plugins == nil {
			t.Error("plugins map is nil")
		}

		// Should start empty
		if len(daemon.plugins) != 0 {
			t.Errorf("expected no plugins, got %d", len(daemon.plugins))
		}
	})
}

func TestDaemonServices(t *testing.T) {
	t.Run("services map is initialized", func(t *testing.T) {
		cfg := config.DefaultConfig()
		store := testutil.NewTestStorage(t)
		defer store.Close()

		daemon := New(cfg, store)

		daemon.servicesMu.RLock()
		defer daemon.servicesMu.RUnlock()

		if daemon.services == nil {
			t.Error("services map is nil")
		}
	})

	t.Run("can register service", func(t *testing.T) {
		cfg := config.DefaultConfig()
		store := testutil.NewTestStorage(t)
		defer store.Close()

		daemon := New(cfg, store)

		// Register a service
		daemon.servicesMu.Lock()
		daemon.services["service1"] = "value1"
		daemon.servicesMu.Unlock()

		// Verify it's there
		daemon.servicesMu.RLock()
		service, exists := daemon.services["service1"]
		daemon.servicesMu.RUnlock()

		if !exists {
			t.Error("service not registered")
		}
		if service != "value1" {
			t.Errorf("wrong service value: %v", service)
		}
	})
}

func TestDaemonContext(t *testing.T) {
	t.Run("daemon has module context", func(t *testing.T) {
		cfg := config.DefaultConfig()
		store := testutil.NewTestStorage(t)
		defer store.Close()

		daemon := New(cfg, store)

		// Module context should be nil initially
		if daemon.moduleCtx != nil {
			t.Log("module context is initialized in New()")
		}
	})

	t.Run("daemon has plugin context", func(t *testing.T) {
		cfg := config.DefaultConfig()
		store := testutil.NewTestStorage(t)
		defer store.Close()

		daemon := New(cfg, store)

		// Plugin context should be nil initially
		if daemon.pluginCtx != nil {
			t.Log("plugin context is initialized in New()")
		}
	})
}

func TestDaemonTimeout(t *testing.T) {
	t.Run("plugin shutdown timeout is set", func(t *testing.T) {
		// PluginShutdownTimeout should be a constant
		if PluginShutdownTimeout == 0 {
			t.Error("PluginShutdownTimeout is zero")
		}

		// Should be reasonable (less than 30 seconds)
		if PluginShutdownTimeout > 30*time.Second {
			t.Logf("PluginShutdownTimeout is large: %v", PluginShutdownTimeout)
		}
	})
}

func TestDaemonIntegration(t *testing.T) {
	t.Run("daemon lifecycle without start", func(t *testing.T) {
		daemon, _, cleanup := setupTestDaemonWithDir(t)
		defer cleanup()

		// Should be able to create and shutdown without starting
		daemon.Shutdown()
	})
}
