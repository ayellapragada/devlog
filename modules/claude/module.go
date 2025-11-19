package claude

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"devlog/internal/install"
	"devlog/internal/modules"
	"devlog/internal/poller"
	"devlog/internal/state"
)

type Module struct{}

func (m *Module) Name() string {
	return "claude"
}

func (m *Module) Description() string {
	return "Capture Claude Code conversation history and activity"
}

func (m *Module) Install(ctx *install.Context) error {
	ctx.Log("Installing Claude Code integration...")

	projectsDir := filepath.Join(ctx.HomeDir, ".claude", "projects")
	if _, err := os.Stat(projectsDir); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("claude projects directory not found at %s. Is Claude Code installed?", projectsDir)
		}
		return fmt.Errorf("error checking Claude Code directory: %w", err)
	}

	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return fmt.Errorf("failed to read Claude Code projects directory: %w", err)
	}

	projectCount := 0
	for _, entry := range entries {
		if entry.IsDir() {
			projectCount++
		}
	}

	ctx.Log("✓ Found Claude Code projects directory at %s", projectsDir)
	ctx.Log("✓ Found %d project directories", projectCount)
	ctx.Log("")
	ctx.Log("Claude Code integration installed successfully!")
	ctx.Log("")
	ctx.Log("The module will poll for new conversation entries and extract:")
	ctx.Log("  • Conversation summaries")
	ctx.Log("  • File operations (read, edit, write)")
	ctx.Log("  • Shell commands executed")
	ctx.Log("  • Work session boundaries")

	return nil
}

func (m *Module) Uninstall(ctx *install.Context) error {
	ctx.Log("Uninstalling Claude Code integration...")

	stateMgr, err := state.NewManager(ctx.DataDir)
	if err != nil {
		ctx.Log("Warning: failed to clean up state: %v", err)
	} else {
		if err := stateMgr.DeleteModule("claude"); err != nil {
			ctx.Log("Warning: failed to clean up state: %v", err)
		} else {
			ctx.Log("✓ Cleaned up claude state")
		}
	}

	ctx.Log("✓ Claude Code integration uninstalled")
	return nil
}

func (m *Module) DefaultConfig() interface{} {
	return map[string]interface{}{
		"poll_interval_seconds": 30,
		"projects_dir":          "~/.claude/projects",
		"extract_commands":      true,
		"extract_file_edits":    true,
		"min_message_length":    10,
	}
}

func (m *Module) ValidateConfig(config interface{}) error {
	cfg, ok := config.(map[string]interface{})
	if !ok {
		return fmt.Errorf("config must be a map")
	}

	if interval, ok := cfg["poll_interval_seconds"].(float64); ok {
		if interval < 5 || interval > 600 {
			return fmt.Errorf("poll_interval_seconds must be between 5 and 600")
		}
	}

	if minLen, ok := cfg["min_message_length"].(float64); ok {
		if minLen < 0 {
			return fmt.Errorf("min_message_length must be non-negative")
		}
	}

	return nil
}

func GetProjectsDir(homeDir string, configPath string) string {
	if configPath == "" {
		configPath = "~/.claude/projects"
	}

	if len(configPath) > 0 && configPath[0] == '~' {
		configPath = filepath.Join(homeDir, configPath[1:])
	}

	return configPath
}

func (m *Module) CreatePoller(config map[string]interface{}, dataDir string) (poller.Poller, error) {
	pollInterval := 30.0
	if interval, ok := config["poll_interval_seconds"].(float64); ok {
		pollInterval = interval
	}

	extractCommands := true
	if ec, ok := config["extract_commands"].(bool); ok {
		extractCommands = ec
	}

	extractFileEdits := true
	if efe, ok := config["extract_file_edits"].(bool); ok {
		extractFileEdits = efe
	}

	minMessageLength := 10
	if mml, ok := config["min_message_length"].(float64); ok {
		minMessageLength = int(mml)
	}

	projectsDirConfig, _ := config["projects_dir"].(string)
	homeDir, _ := os.UserHomeDir()
	projectsDir := GetProjectsDir(homeDir, projectsDirConfig)

	return NewPoller(
		projectsDir,
		dataDir,
		time.Duration(pollInterval)*time.Second,
		extractCommands,
		extractFileEdits,
		minMessageLength,
	)
}

func init() {
	modules.Register(&Module{})
}
