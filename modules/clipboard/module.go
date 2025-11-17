package clipboard

import (
	"fmt"

	"devlog/internal/modules"
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

func (m *Module) Install(ctx *modules.InstallContext) error {
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

func (m *Module) Uninstall(ctx *modules.InstallContext) error {
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
	return nil
}

func init() {
	modules.Register(&Module{})
}
