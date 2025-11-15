package git

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"devlog/internal/modules"
)

var postCommitHook string

type Module struct{}

func (m *Module) Name() string {
	return "git"
}

func (m *Module) Description() string {
	return "Capture git commits automatically"
}

func (m *Module) Install(ctx *modules.InstallContext) error {
	ctx.Log("Installing git hooks...")

	hooksDir := filepath.Join(ctx.HomeDir, ".config", "git", "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return fmt.Errorf("create hooks directory: %w", err)
	}

	cmd := exec.Command("git", "config", "--global", "--get", "core.hooksPath")
	output, _ := cmd.Output()
	currentHooksPath := string(output)
	if len(currentHooksPath) > 0 {
		currentHooksPath = currentHooksPath[:len(currentHooksPath)-1]
	}

	if currentHooksPath != "" && currentHooksPath != hooksDir {
		if ctx.Interactive {
			ctx.Log("Warning: Git is already configured to use a different global hooks directory:")
			ctx.Log("  %s", currentHooksPath)
			ctx.Log("Devlog will use: %s", hooksDir)
			ctx.Log("")
			ctx.Log("You may need to manually copy hooks if you have existing ones.")
		}
	}

	hookPath := filepath.Join(hooksDir, "post-commit")
	if err := os.WriteFile(hookPath, []byte(postCommitHook), 0755); err != nil {
		return fmt.Errorf("write post-commit hook: %w", err)
	}

	cmd = exec.Command("git", "config", "--global", "core.hooksPath", hooksDir)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("configure git hooks path: %w", err)
	}

	ctx.Log("✓ Installed post-commit hook to %s", hookPath)
	ctx.Log("✓ Configured git to use global hooks directory: %s", hooksDir)
	ctx.Log("")
	ctx.Log("All git repositories on this system will now send commit events to devlog.")

	return nil
}

func (m *Module) Uninstall(ctx *modules.InstallContext) error {
	ctx.Log("Uninstalling git hooks...")

	cmd := exec.Command("git", "config", "--global", "--get", "core.hooksPath")
	output, _ := cmd.Output()
	hooksPath := string(output)
	if len(hooksPath) > 0 {
		hooksPath = hooksPath[:len(hooksPath)-1]
	}

	if hooksPath == "" {
		ctx.Log("No global hooks directory configured")
		return nil
	}

	hookPath := filepath.Join(hooksPath, "post-commit")
	if _, err := os.Stat(hookPath); err == nil {
		content, err := os.ReadFile(hookPath)
		if err == nil && string(content) == postCommitHook {
			if err := os.Remove(hookPath); err != nil {
				return fmt.Errorf("remove post-commit hook: %w", err)
			}
			ctx.Log("✓ Removed post-commit hook from %s", hookPath)
		} else {
			ctx.Log("Warning: post-commit hook at %s doesn't match devlog's hook, skipping removal", hookPath)
		}
	}

	entries, err := os.ReadDir(hooksPath)
	if err == nil && len(entries) == 0 {
		cmd := exec.Command("git", "config", "--global", "--unset", "core.hooksPath")
		_ = cmd.Run()
		ctx.Log("✓ Removed git global hooks directory configuration")
	}

	return nil
}

func (m *Module) DefaultConfig() interface{} {
	return map[string]interface{}{}
}

func (m *Module) ValidateConfig(config interface{}) error {
	return nil
}

func init() {
	modules.Register(&Module{})
}
