package clipboard

import (
	"fmt"
	"time"

	"devlog/internal/install"
	"devlog/internal/modules"
	"devlog/internal/poller"
	"devlog/internal/state"

	"golang.design/x/clipboard"
)

type Module struct{}

func (m *Module) Name() string {
	return "clipboard"
}

func (m *Module) Description() string {
	return "Track clipboard changes and maintain history"
}

func (m *Module) Install(ctx *install.Context) error {
	ctx.Log("Installing clipboard tracker...")
	ctx.Log("")

	if err := clipboard.Init(); err != nil {
		return fmt.Errorf("failed to initialize clipboard access: %w", err)
	}

	_ = clipboard.Read(clipboard.FmtText)
	ctx.Log("✓ Clipboard access verified")
	ctx.Log("")

	ctx.Log("PRIVACY NOTICE:")
	ctx.Log("  The clipboard tracker will monitor all text you copy to your clipboard.")
	ctx.Log("  This data will be stored locally in your devlog database.")
	ctx.Log("  Clipboard content is tracked automatically while the daemon is running.")
	ctx.Log("")
	ctx.Log("Configuration:")
	ctx.Log("  • Poll interval: 5 seconds (configurable)")
	ctx.Log("")
	ctx.Log("✓ Clipboard tracking will run in the background when daemon starts")

	return nil
}

func (m *Module) Uninstall(ctx *install.Context) error {
	ctx.Log("Uninstalling clipboard tracker...")

	stateMgr, err := state.NewManager(ctx.DataDir)
	if err != nil {
		ctx.Log("Warning: failed to clean up state: %v", err)
	} else {
		if err := stateMgr.DeleteModule("clipboard"); err != nil {
			ctx.Log("Warning: failed to clean up state: %v", err)
		} else {
			ctx.Log("✓ Cleaned up clipboard state")
		}
	}

	ctx.Log("✓ Clipboard tracking will be disabled")
	ctx.Log("")
	ctx.Log("Note: Historical clipboard data will be preserved in your devlog.")
	return nil
}

func (m *Module) DefaultConfig() interface{} {
	return map[string]interface{}{
		"poll_interval_seconds": 5,
		"max_length":            10000,
		"min_length":            3,
		"dedup_history_size":    5,
	}
}

func (m *Module) ValidateConfig(config interface{}) error {
	cfg, ok := config.(map[string]interface{})
	if !ok {
		return fmt.Errorf("config must be a map")
	}

	if val, ok := cfg["poll_interval_seconds"]; ok {
		var interval float64
		switch v := val.(type) {
		case float64:
			interval = v
		case int:
			interval = float64(v)
		default:
			return fmt.Errorf("poll_interval_seconds must be a number")
		}
		if interval < 1 || interval > 3600 {
			return fmt.Errorf("poll_interval_seconds must be between 1 and 3600")
		}
	}

	if val, ok := cfg["max_length"]; ok {
		var maxLength float64
		switch v := val.(type) {
		case float64:
			maxLength = v
		case int:
			maxLength = float64(v)
		default:
			return fmt.Errorf("max_length must be a number")
		}
		if maxLength < 1 || maxLength > 1000000 {
			return fmt.Errorf("max_length must be between 1 and 1000000")
		}
	}

	if val, ok := cfg["min_length"]; ok {
		var minLength float64
		switch v := val.(type) {
		case float64:
			minLength = v
		case int:
			minLength = float64(v)
		default:
			return fmt.Errorf("min_length must be a number")
		}
		if minLength < 0 {
			return fmt.Errorf("min_length cannot be negative")
		}
	}

	if val, ok := cfg["dedup_history_size"]; ok {
		var dedupSize float64
		switch v := val.(type) {
		case float64:
			dedupSize = v
		case int:
			dedupSize = float64(v)
		default:
			return fmt.Errorf("dedup_history_size must be a number")
		}
		if dedupSize < 1 || dedupSize > 100 {
			return fmt.Errorf("dedup_history_size must be between 1 and 100")
		}
	}

	return nil
}

func (m *Module) CreatePoller(config map[string]interface{}, dataDir string) (poller.Poller, error) {
	pollInterval := 5.0
	if val, exists := config["poll_interval_seconds"]; exists {
		switch v := val.(type) {
		case float64:
			pollInterval = v
		case int:
			pollInterval = float64(v)
		}
	}

	maxLength := 10000
	if val, exists := config["max_length"]; exists {
		switch v := val.(type) {
		case float64:
			maxLength = int(v)
		case int:
			maxLength = v
		}
	}

	minLength := 1
	if val, exists := config["min_length"]; exists {
		switch v := val.(type) {
		case float64:
			minLength = int(v)
		case int:
			minLength = v
		}
	}

	dedupHistorySize := 5
	if val, exists := config["dedup_history_size"]; exists {
		switch v := val.(type) {
		case float64:
			dedupHistorySize = int(v)
		case int:
			dedupHistorySize = v
		}
	}

	duration := time.Duration(pollInterval) * time.Second

	poller, err := NewPoller(dataDir, duration, maxLength, minLength, dedupHistorySize)
	if err != nil {
		return nil, err
	}

	if err := poller.Init(); err != nil {
		return nil, fmt.Errorf("init poller: %w", err)
	}

	return poller, nil
}

func init() {
	modules.Register(&Module{})
}
