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
	ctx.Log("  • Poll interval: 3s (configurable)")
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
		"poll_interval":      "3s",
		"max_length":         10000,
		"min_length":         1,
		"dedup_history_size": 5,
	}
}

func (m *Module) ValidateConfig(config interface{}) error {
	cfg, ok := config.(map[string]interface{})
	if !ok {
		return fmt.Errorf("config must be a map")
	}

	if maxLength, ok := cfg["max_length"].(float64); ok {
		if maxLength < 1 || maxLength > 1000000 {
			return fmt.Errorf("max_length must be between 1 and 1000000")
		}
	}

	if minLength, ok := cfg["min_length"].(float64); ok {
		if minLength < 0 {
			return fmt.Errorf("min_length cannot be negative")
		}
	}

	if dedupSize, ok := cfg["dedup_history_size"].(float64); ok {
		if dedupSize < 1 || dedupSize > 100 {
			return fmt.Errorf("dedup_history_size must be between 1 and 100")
		}
	}

	return nil
}

func (m *Module) CreatePoller(config map[string]interface{}, dataDir string) (poller.Poller, error) {
	pollInterval := "3s"
	if interval, ok := config["poll_interval"].(string); ok {
		pollInterval = interval
	}

	maxLength := 10000
	if ml, ok := config["max_length"].(float64); ok {
		maxLength = int(ml)
	}

	minLength := 1
	if ml, ok := config["min_length"].(float64); ok {
		minLength = int(ml)
	}

	dedupHistorySize := 5
	if dhs, ok := config["dedup_history_size"].(float64); ok {
		dedupHistorySize = int(dhs)
	}

	duration, err := time.ParseDuration(pollInterval)
	if err != nil {
		duration = 3 * time.Second
	}

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
