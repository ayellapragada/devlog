package shell

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"devlog/internal/configfile"
	"devlog/internal/install"
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

func (m *Module) Install(ctx *install.Context) error {
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
		return &modules.InstallError{
			Component: "shell integration",
			File:      hooksDir,
			Err:       err,
			RecoverySteps: []string{
				fmt.Sprintf("Check directory permissions: ls -la %s", filepath.Dir(hooksDir)),
				fmt.Sprintf("Try creating manually: mkdir -p %s", hooksDir),
				fmt.Sprintf("Check disk space: df -h %s", filepath.Dir(hooksDir)),
			},
		}
	}

	scriptPath := filepath.Join(hooksDir, "devlog.sh")
	if err := os.WriteFile(scriptPath, []byte(devlogShellScript), 0644); err != nil {
		return &modules.InstallError{
			Component: "shell integration",
			File:      scriptPath,
			Err:       err,
			RecoverySteps: []string{
				fmt.Sprintf("Check file permissions: ls -la %s", filepath.Dir(scriptPath)),
				fmt.Sprintf("Ensure directory exists: mkdir -p %s", filepath.Dir(scriptPath)),
				"Check if file is write-protected",
			},
		}
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

func (m *Module) installBash(ctx *install.Context, sourceLine string) error {
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

func (m *Module) installZsh(ctx *install.Context, sourceLine string) error {
	ctx.Log("Installing for Zsh...")
	rcFile := filepath.Join(ctx.HomeDir, ".zshrc")
	return m.addToRcFile(ctx, rcFile, sourceLine)
}

func (m *Module) addToRcFile(ctx *install.Context, rcFile string, sourceLine string) error {
	cfgMgr := configfile.NewFileSystemManager(".backup.devlog")

	hasSection, err := cfgMgr.HasSection(rcFile, "devlog shell integration")
	if err != nil {
		return &modules.InstallError{
			Component: "shell integration",
			File:      rcFile,
			Err:       err,
			RecoverySteps: []string{
				fmt.Sprintf("Check if file exists: ls -la %s", rcFile),
				fmt.Sprintf("Check file permissions: stat %s", rcFile),
				fmt.Sprintf("Try creating manually: touch %s", rcFile),
			},
		}
	}

	if hasSection {
		ctx.Log("Already installed in %s", rcFile)
		return nil
	}

	if err := cfgMgr.AddSection(rcFile, "devlog shell integration", sourceLine); err != nil {
		return &modules.InstallError{
			Component: "shell integration",
			File:      rcFile,
			Err:       err,
			RecoverySteps: []string{
				fmt.Sprintf("Check file permissions: ls -la %s", rcFile),
				"Ensure file is not read-only",
				fmt.Sprintf("Try manual install: echo '%s' >> %s", sourceLine, rcFile),
				"Restart shell after manual installation",
			},
		}
	}

	ctx.Log("✓ Added devlog hook to %s", rcFile)
	return nil
}

func (m *Module) Uninstall(ctx *install.Context) error {
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

func (m *Module) uninstallBash(ctx *install.Context) {
	ctx.Log("Checking Bash configuration...")

	bashProfile := filepath.Join(ctx.HomeDir, ".bash_profile")
	m.removeFromRcFile(ctx, bashProfile)

	bashrc := filepath.Join(ctx.HomeDir, ".bashrc")
	m.removeFromRcFile(ctx, bashrc)
}

func (m *Module) uninstallZsh(ctx *install.Context) {
	ctx.Log("Checking Zsh configuration...")
	zshrc := filepath.Join(ctx.HomeDir, ".zshrc")
	m.removeFromRcFile(ctx, zshrc)
}

func (m *Module) removeFromRcFile(ctx *install.Context, rcFile string) {
	cfgMgr := configfile.NewFileSystemManager(".backup.devlog.uninstall")

	hasSection, err := cfgMgr.HasSection(rcFile, "devlog shell integration")
	if err != nil {
		ctx.Log("Warning: couldn't check %s: %v", rcFile, err)
		return
	}

	if !hasSection {
		return
	}

	if err := cfgMgr.RemoveSection(rcFile, "devlog shell integration"); err != nil {
		ctx.Log("Warning: couldn't remove from %s: %v", rcFile, err)
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
	cfg, ok := config.(map[string]interface{})
	if !ok {
		return fmt.Errorf("config must be a map")
	}

	if ignoreList, ok := cfg["ignore_list"]; ok {
		ignoreSlice, ok := ignoreList.([]interface{})
		if !ok {
			return fmt.Errorf("ignore_list must be an array of strings")
		}
		for i, item := range ignoreSlice {
			if _, ok := item.(string); !ok {
				return fmt.Errorf("ignore_list[%d] must be a string", i)
			}
		}
	}

	return nil
}

func init() {
	modules.Register(&Module{})
}
