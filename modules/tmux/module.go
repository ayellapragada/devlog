package tmux

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"devlog/internal/config"
	"devlog/internal/install"
	"devlog/internal/modules"
)

//go:embed hooks/tmux-hooks.conf
var tmuxHooksConfig string

type Module struct{}

func (m *Module) Name() string {
	return "tmux"
}

func (m *Module) Description() string {
	return "Capture tmux session, window, and pane events automatically"
}

func (m *Module) Install(ctx *install.Context) error {
	ctx.Log("Installing tmux hooks...")

	if err := checkTmuxInstalled(); err != nil {
		return err
	}

	configDir := filepath.Join(ctx.HomeDir, ".config", "devlog")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return &modules.InstallError{
			Component: "tmux integration",
			File:      configDir,
			Err:       err,
			RecoverySteps: []string{
				fmt.Sprintf("Check directory permissions: ls -la %s", filepath.Dir(configDir)),
				fmt.Sprintf("Try creating manually: mkdir -p %s", configDir),
				"Check disk space: df -h",
			},
		}
	}

	hooksPath := filepath.Join(configDir, "tmux-hooks.conf")
	if err := os.WriteFile(hooksPath, []byte(tmuxHooksConfig), 0644); err != nil {
		return &modules.InstallError{
			Component: "tmux integration",
			File:      hooksPath,
			Err:       err,
			RecoverySteps: []string{
				fmt.Sprintf("Check file permissions: ls -la %s", filepath.Dir(hooksPath)),
				fmt.Sprintf("Ensure directory exists: mkdir -p %s", filepath.Dir(hooksPath)),
				"Check if file is write-protected",
			},
		}
	}

	ctx.Log("✓ Created tmux hooks config at %s", hooksPath)

	tmuxConfPath := filepath.Join(ctx.HomeDir, ".tmux.conf")
	sourceLine := fmt.Sprintf("source-file %s", hooksPath)

	if err := addToTmuxConf(ctx, tmuxConfPath, sourceLine); err != nil {
		return err
	}

	cfg, err := config.Load()
	if err == nil && cfg.IsModuleEnabled("shell") {
		cfg.AddToShellIgnoreList("tmux")
		if err := cfg.Save(); err == nil {
			ctx.Log("✓ Added 'tmux' to shell module ignore list")
		}
	}

	if isTmuxRunning() {
		ctx.Log("")
		ctx.Log("Reloading tmux configuration...")
		if err := reloadTmuxConfig(tmuxConfPath); err != nil {
			ctx.Log("Warning: Could not reload tmux config automatically: %v", err)
			ctx.Log("")
			ctx.Log("Please manually reload your tmux config:")
			ctx.Log("  tmux source-file ~/.tmux.conf")
		} else {
			ctx.Log("✓ Tmux configuration reloaded")
		}
	}

	return nil
}

func (m *Module) Uninstall(ctx *install.Context) error {
	ctx.Log("Uninstalling tmux hooks...")

	configDir := filepath.Join(ctx.HomeDir, ".config", "devlog")
	hooksPath := filepath.Join(configDir, "tmux-hooks.conf")

	if _, err := os.Stat(hooksPath); err == nil {
		if err := os.Remove(hooksPath); err != nil {
			return fmt.Errorf("remove tmux hooks config: %w", err)
		}
		ctx.Log("✓ Removed tmux hooks config from %s", hooksPath)
	}

	tmuxConfPath := filepath.Join(ctx.HomeDir, ".tmux.conf")
	removeFromTmuxConf(ctx, tmuxConfPath, hooksPath)

	cfg, err := config.Load()
	if err == nil && cfg.IsModuleEnabled("shell") {
		cfg.RemoveFromShellIgnoreList("tmux")
		if err := cfg.Save(); err == nil {
			ctx.Log("✓ Removed 'tmux' from shell module ignore list")
		}
	}

	if isTmuxRunning() {
		ctx.Log("")
		ctx.Log("Reloading tmux configuration...")
		if err := reloadTmuxConfig(tmuxConfPath); err != nil {
			ctx.Log("Warning: Could not reload tmux config automatically: %v", err)
			ctx.Log("")
			ctx.Log("Please manually reload your tmux config:")
			ctx.Log("  tmux source-file ~/.tmux.conf")
		} else {
			ctx.Log("✓ Tmux configuration reloaded")
		}
	}

	return nil
}

func (m *Module) DefaultConfig() interface{} {
	return map[string]interface{}{}
}

func (m *Module) ValidateConfig(config interface{}) error {
	_, ok := config.(map[string]interface{})
	if !ok {
		return fmt.Errorf("config must be a map")
	}

	return nil
}

func addToTmuxConf(ctx *install.Context, tmuxConfPath, sourceLine string) error {
	var content []byte

	if _, err := os.Stat(tmuxConfPath); err == nil {
		content, err = os.ReadFile(tmuxConfPath)
		if err != nil {
			return fmt.Errorf("read %s: %w", tmuxConfPath, err)
		}

		if strings.Contains(string(content), "devlog tmux integration") ||
			strings.Contains(string(content), "tmux-hooks.conf") {
			ctx.Log("Already installed in %s", tmuxConfPath)
			return nil
		}

		backupPath := fmt.Sprintf("%s.backup.devlog", tmuxConfPath)
		if err := os.WriteFile(backupPath, content, 0644); err != nil {
			return fmt.Errorf("create backup: %w", err)
		}
		ctx.Log("Created backup: %s", backupPath)
	}

	toAdd := fmt.Sprintf("# devlog tmux integration\n%s\n", sourceLine)

	contentStr := string(content)
	lines := strings.Split(contentStr, "\n")

	tpmLineIndex := -1
	for i, line := range lines {
		if strings.Contains(line, "run") && (strings.Contains(line, "tpm") || strings.Contains(line, ".tmux/plugins")) {
			tpmLineIndex = i
			break
		}
	}

	var newContent string
	if tpmLineIndex >= 0 {
		for tpmLineIndex > 0 && (strings.TrimSpace(lines[tpmLineIndex-1]) == "" || strings.Contains(lines[tpmLineIndex-1], "Initialize TMUX plugin manager")) {
			tpmLineIndex--
		}

		before := strings.Join(lines[:tpmLineIndex], "\n")
		after := strings.Join(lines[tpmLineIndex:], "\n")

		if before != "" && !strings.HasSuffix(before, "\n") {
			before += "\n"
		}
		newContent = before + "\n" + toAdd + "\n" + after
	} else {
		if contentStr != "" && !strings.HasSuffix(contentStr, "\n") {
			contentStr += "\n"
		}
		newContent = contentStr + "\n" + toAdd
	}

	if err := os.WriteFile(tmuxConfPath, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("write to %s: %w", tmuxConfPath, err)
	}

	ctx.Log("✓ Added devlog hooks to %s", tmuxConfPath)
	return nil
}

func removeFromTmuxConf(ctx *install.Context, tmuxConfPath, hooksPath string) {
	if _, err := os.Stat(tmuxConfPath); err != nil {
		return
	}

	content, err := os.ReadFile(tmuxConfPath)
	if err != nil {
		ctx.Log("Warning: couldn't read %s: %v", tmuxConfPath, err)
		return
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "devlog tmux integration") &&
		!strings.Contains(contentStr, "tmux-hooks.conf") {
		return
	}

	backupPath := fmt.Sprintf("%s.backup.devlog.uninstall", tmuxConfPath)
	if err := os.WriteFile(backupPath, content, 0644); err != nil {
		ctx.Log("Warning: couldn't create backup of %s: %v", tmuxConfPath, err)
		return
	}
	ctx.Log("Created backup: %s", backupPath)

	lines := strings.Split(contentStr, "\n")
	var newLines []string
	skipNext := false

	for _, line := range lines {
		if strings.Contains(line, "# devlog tmux integration") {
			skipNext = true
			continue
		}

		if skipNext && strings.Contains(line, "tmux-hooks.conf") {
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

	if err := os.WriteFile(tmuxConfPath, []byte(newContent), 0644); err != nil {
		ctx.Log("Warning: couldn't write to %s: %v", tmuxConfPath, err)
		return
	}

	ctx.Log("✓ Removed devlog hooks from %s", tmuxConfPath)
}

func isTmuxRunning() bool {
	cmd := exec.Command("tmux", "list-sessions")
	return cmd.Run() == nil
}

func reloadTmuxConfig(tmuxConfPath string) error {
	cmd := exec.Command("tmux", "source-file", tmuxConfPath)
	return cmd.Run()
}

func checkTmuxInstalled() error {
	if _, err := exec.LookPath("tmux"); err != nil {
		return &modules.InstallError{
			Component: "tmux integration",
			Err:       err,
			RecoverySteps: []string{
				"Install tmux: brew install tmux (macOS) or apt install tmux (Linux)",
				"Verify installation: tmux -V",
				"Ensure tmux is in your PATH: echo $PATH",
			},
		}
	}
	return nil
}

func init() {
	modules.Register(&Module{})
}
