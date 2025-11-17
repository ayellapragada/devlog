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

type Module struct{}

func (m *Module) Name() string {
	return "shell"
}

func (m *Module) Description() string {
	return "Capture shell commands automatically"
}

func (m *Module) Install(ctx *modules.InstallContext) error {
	ctx.Log("Installing shell hooks...")

	shellEnv := os.Getenv("SHELL")
	currentShell := "unknown"
	if shellEnv != "" {
		currentShell = filepath.Base(shellEnv)
	}

	ctx.Log("Current shell: %s", currentShell)
	ctx.Log("")

	hooksDir := filepath.Join(ctx.DataDir, "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return fmt.Errorf("create hooks directory: %w", err)
	}

	scriptPath := filepath.Join(hooksDir, "devlog.sh")
	if err := os.WriteFile(scriptPath, []byte(devlogShellScript), 0644); err != nil {
		return fmt.Errorf("write devlog.sh: %w", err)
	}

	ctx.Log("✓ Created shell integration script at %s", scriptPath)

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

	if _, err := os.Stat("/System/Library/CoreServices/SystemVersion.plist"); err == nil {
		bashProfile := filepath.Join(ctx.HomeDir, ".bash_profile")
		if _, err := os.Stat(bashProfile); err == nil {
			rcFile = bashProfile
		}
	}

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

		backupPath := fmt.Sprintf("%s.backup.devlog", rcFile)
		if err := os.WriteFile(backupPath, content, 0644); err != nil {
			return fmt.Errorf("create backup: %w", err)
		}
		ctx.Log("Created backup: %s", backupPath)
	}

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

func (m *Module) Uninstall(ctx *modules.InstallContext) error {
	ctx.Log("Uninstalling shell hooks...")

	scriptPath := filepath.Join(ctx.DataDir, "hooks", "devlog.sh")
	if _, err := os.Stat(scriptPath); err == nil {
		if err := os.Remove(scriptPath); err != nil {
			return fmt.Errorf("remove devlog.sh: %w", err)
		}
		ctx.Log("✓ Removed %s", scriptPath)
	}

	shellEnv := os.Getenv("SHELL")
	currentShell := "unknown"
	if shellEnv != "" {
		currentShell = filepath.Base(shellEnv)
	}

	ctx.Log("")

	switch currentShell {
	case "bash":
		m.uninstallBash(ctx)
	case "zsh":
		m.uninstallZsh(ctx)
	default:
		ctx.Log("Please manually remove the 'devlog shell integration' section from your shell RC files:")
		ctx.Log("  ~/.bashrc or ~/.bash_profile (for Bash)")
		ctx.Log("  ~/.zshrc (for Zsh)")
	}

	return nil
}

func (m *Module) uninstallBash(ctx *modules.InstallContext) {
	ctx.Log("Checking Bash configuration...")

	bashProfile := filepath.Join(ctx.HomeDir, ".bash_profile")
	m.removeFromRcFile(ctx, bashProfile)

	bashrc := filepath.Join(ctx.HomeDir, ".bashrc")
	m.removeFromRcFile(ctx, bashrc)
}

func (m *Module) uninstallZsh(ctx *modules.InstallContext) {
	ctx.Log("Checking Zsh configuration...")
	zshrc := filepath.Join(ctx.HomeDir, ".zshrc")
	m.removeFromRcFile(ctx, zshrc)
}

func (m *Module) removeFromRcFile(ctx *modules.InstallContext, rcFile string) {
	if _, err := os.Stat(rcFile); err != nil {
		return
	}

	content, err := os.ReadFile(rcFile)
	if err != nil {
		ctx.Log("Warning: couldn't read %s: %v", rcFile, err)
		return
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "devlog shell integration") &&
		!strings.Contains(contentStr, "devlog.sh") {
		return
	}

	backupPath := fmt.Sprintf("%s.backup.devlog.uninstall", rcFile)
	if err := os.WriteFile(backupPath, content, 0644); err != nil {
		ctx.Log("Warning: couldn't create backup of %s: %v", rcFile, err)
		return
	}
	ctx.Log("Created backup: %s", backupPath)

	lines := strings.Split(contentStr, "\n")
	var newLines []string
	skipNext := false

	for _, line := range lines {
		if strings.Contains(line, "# devlog shell integration") {
			skipNext = true
			continue
		}

		if skipNext && strings.Contains(line, "devlog.sh") {
			skipNext = false
			continue
		}

		skipNext = false
		newLines = append(newLines, line)
	}

	for len(newLines) > 0 && strings.TrimSpace(newLines[len(newLines)-1]) == "" {
		newLines = newLines[:len(newLines)-1]
	}

	newContent := strings.Join(newLines, "\n")
	if len(newLines) > 0 && !strings.HasSuffix(newContent, "\n") {
		newContent += "\n"
	}

	if err := os.WriteFile(rcFile, []byte(newContent), 0644); err != nil {
		ctx.Log("Warning: couldn't write to %s: %v", rcFile, err)
		return
	}

	ctx.Log("✓ Removed devlog hook from %s", rcFile)
}

func (m *Module) DefaultConfig() interface{} {
	return map[string]interface{}{
		"ignore_list": []string{
			"ls", "cd", "pwd", "echo", "cat", "clear",
			"exit", "history", "which", "type", "alias",
		},
	}
}

func (m *Module) ValidateConfig(config interface{}) error {
	_, ok := config.(map[string]interface{})
	if !ok {
		return fmt.Errorf("config must be a map")
	}

	return nil
}

func init() {
	modules.Register(&Module{})
}
