package shell

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"devlog/internal/modules"
)

//go:embed hooks/devlog.sh
var devlogShellScript string

//go:embed install.sh
var installScript string

// Module implements the shell integration module
type Module struct{}

// Name returns the module identifier
func (m *Module) Name() string {
	return "shell"
}

// Description returns a user-friendly description
func (m *Module) Description() string {
	return "Capture shell commands automatically"
}

// Install sets up shell hooks
func (m *Module) Install(ctx *modules.InstallContext) error {
	ctx.Log("Installing shell hooks...")

	// Detect current shell
	shellEnv := os.Getenv("SHELL")
	currentShell := "unknown"
	if shellEnv != "" {
		currentShell = filepath.Base(shellEnv)
	}

	ctx.Log("Current shell: %s", currentShell)
	ctx.Log("")

	// Create hooks directory in data dir to store the shell script
	hooksDir := filepath.Join(ctx.DataDir, "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return fmt.Errorf("create hooks directory: %w", err)
	}

	// Write devlog.sh script
	scriptPath := filepath.Join(hooksDir, "devlog.sh")
	if err := os.WriteFile(scriptPath, []byte(devlogShellScript), 0644); err != nil {
		return fmt.Errorf("write devlog.sh: %w", err)
	}

	ctx.Log("✓ Created shell integration script at %s", scriptPath)

	// Add source line to appropriate RC file
	sourceLine := fmt.Sprintf(`source "%s"`, scriptPath)

	switch currentShell {
	case "bash":
		if err := m.installBash(ctx, sourceLine); err != nil {
			return err
		}
	case "zsh":
		if err := m.installZsh(ctx, sourceLine); err != nil {
			return err
		}
	default:
		ctx.Log("")
		ctx.Log("Warning: Unsupported shell: %s", currentShell)
		ctx.Log("Please manually add the following line to your shell's RC file:")
		ctx.Log("")
		ctx.Log("  %s", sourceLine)
		ctx.Log("")
		return fmt.Errorf("unsupported shell: %s", currentShell)
	}

	ctx.Log("")
	ctx.Log("Installation complete!")
	ctx.Log("")
	ctx.Log("To activate the hooks:")
	ctx.Log("  1. Restart your shell, or")
	ctx.Log("  2. Run: source ~/.%src", currentShell)
	ctx.Log("")

	return nil
}

func (m *Module) installBash(ctx *modules.InstallContext, sourceLine string) error {
	ctx.Log("Installing for Bash...")

	var rcFile string

	// On macOS, prefer .bash_profile if it exists
	if _, err := os.Stat("/System/Library/CoreServices/SystemVersion.plist"); err == nil {
		bashProfile := filepath.Join(ctx.HomeDir, ".bash_profile")
		if _, err := os.Stat(bashProfile); err == nil {
			rcFile = bashProfile
		}
	}

	// Default to .bashrc
	if rcFile == "" {
		rcFile = filepath.Join(ctx.HomeDir, ".bashrc")
	}

	return m.addToRcFile(ctx, rcFile, sourceLine)
}

func (m *Module) installZsh(ctx *modules.InstallContext, sourceLine string) error {
	ctx.Log("Installing for Zsh...")
	rcFile := filepath.Join(ctx.HomeDir, ".zshrc")
	return m.addToRcFile(ctx, rcFile, sourceLine)
}

func (m *Module) addToRcFile(ctx *modules.InstallContext, rcFile string, sourceLine string) error {
	// Check if already installed
	if _, err := os.Stat(rcFile); err == nil {
		content, err := os.ReadFile(rcFile)
		if err != nil {
			return fmt.Errorf("read %s: %w", rcFile, err)
		}

		if strings.Contains(string(content), "devlog shell integration") ||
			strings.Contains(string(content), "devlog.sh") {
			ctx.Log("Already installed in %s", rcFile)
			return nil
		}

		// Backup RC file
		backupPath := fmt.Sprintf("%s.backup.devlog", rcFile)
		if err := os.WriteFile(backupPath, content, 0644); err != nil {
			return fmt.Errorf("create backup: %w", err)
		}
		ctx.Log("Created backup: %s", backupPath)
	}

	// Add source line
	f, err := os.OpenFile(rcFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open %s: %w", rcFile, err)
	}
	defer f.Close()

	toAdd := fmt.Sprintf("\n# devlog shell integration\n%s\n", sourceLine)
	if _, err := f.WriteString(toAdd); err != nil {
		return fmt.Errorf("write to %s: %w", rcFile, err)
	}

	ctx.Log("✓ Added devlog hook to %s", rcFile)
	return nil
}

// Uninstall removes shell hooks
func (m *Module) Uninstall(ctx *modules.InstallContext) error {
	ctx.Log("Uninstalling shell hooks...")

	// Remove the devlog.sh script
	scriptPath := filepath.Join(ctx.DataDir, "hooks", "devlog.sh")
	if _, err := os.Stat(scriptPath); err == nil {
		if err := os.Remove(scriptPath); err != nil {
			return fmt.Errorf("remove devlog.sh: %w", err)
		}
		ctx.Log("✓ Removed %s", scriptPath)
	}

	// Inform user to remove from RC files
	ctx.Log("")
	ctx.Log("Please manually remove the 'devlog shell integration' section from your shell RC files:")
	ctx.Log("  ~/.bashrc or ~/.bash_profile (for Bash)")
	ctx.Log("  ~/.zshrc (for Zsh)")

	return nil
}

// DefaultConfig returns default shell module configuration
func (m *Module) DefaultConfig() interface{} {
	return map[string]interface{}{
		"capture_mode": "important",
		"ignore_list": []string{
			"ls", "cd", "pwd", "echo", "cat", "clear",
			"exit", "history", "which", "type", "alias",
		},
	}
}

// ValidateConfig validates the shell module configuration
func (m *Module) ValidateConfig(config interface{}) error {
	cfg, ok := config.(map[string]interface{})
	if !ok {
		return fmt.Errorf("config must be a map")
	}

	// Validate capture_mode if present
	if mode, ok := cfg["capture_mode"].(string); ok {
		if mode != "all" && mode != "important" {
			return fmt.Errorf("capture_mode must be 'all' or 'important'")
		}
	}

	return nil
}

func init() {
	// Register the shell module
	modules.Register(&Module{})
}
