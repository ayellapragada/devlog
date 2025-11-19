package git

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"devlog/internal/install"
	"devlog/internal/modules"
)

//go:embed hooks/git-wrapper.sh
var gitWrapperScript string

//go:embed hooks/devlog-git-common.sh
var gitCommonLib string

type Module struct{}

func (m *Module) Name() string {
	return "git"
}

func (m *Module) Description() string {
	return "Capture git operations (commits, pushes, pulls, merges, etc.) automatically"
}

func (m *Module) Install(ctx *install.Context) error {
	ctx.Log("Installing git command wrapper...")

	binDir := filepath.Join(ctx.HomeDir, ".local", "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return &modules.InstallError{
			Component: "git wrapper",
			File:      binDir,
			Err:       err,
			RecoverySteps: []string{
				fmt.Sprintf("Check directory permissions: ls -la %s", filepath.Dir(binDir)),
				fmt.Sprintf("Try creating manually: mkdir -p %s", binDir),
				"Check disk space: df -h",
			},
		}
	}

	commonLibPath := filepath.Join(binDir, "devlog-git-common.sh")
	if err := os.WriteFile(commonLibPath, []byte(gitCommonLib), 0644); err != nil {
		return &modules.InstallError{
			Component: "git wrapper",
			File:      commonLibPath,
			Err:       err,
			RecoverySteps: []string{
				fmt.Sprintf("Check file permissions: ls -la %s", filepath.Dir(commonLibPath)),
				fmt.Sprintf("Ensure directory exists: mkdir -p %s", filepath.Dir(commonLibPath)),
				"Check if file is write-protected",
			},
		}
	}

	wrapperPath := filepath.Join(binDir, "git")
	if err := os.WriteFile(wrapperPath, []byte(gitWrapperScript), 0755); err != nil {
		return &modules.InstallError{
			Component: "git wrapper",
			File:      wrapperPath,
			Err:       err,
			RecoverySteps: []string{
				fmt.Sprintf("Check file permissions: ls -la %s", filepath.Dir(wrapperPath)),
				"Ensure directory exists and is writable",
				fmt.Sprintf("Try manual install: Save the wrapper script to %s and chmod +x %s", wrapperPath, wrapperPath),
			},
		}
	}

	ctx.Log("✓ Installed shared library to %s", commonLibPath)
	ctx.Log("✓ Installed git wrapper to %s", wrapperPath)
	ctx.Log("")
	ctx.Log("All git operations (commits, pushes, pulls, merges, etc.) will now be tracked.")
	ctx.Log("")
	ctx.Log("IMPORTANT: Ensure %s is in your PATH and appears BEFORE /usr/bin", binDir)
	ctx.Log("Add this to your shell RC file:")
	ctx.Log("")
	ctx.Log("  export PATH=\"%s:$PATH\"", binDir)
	ctx.Log("")
	ctx.Log("Then restart your shell or run: source ~/.zshrc (or ~/.bashrc)")

	return nil
}

func (m *Module) Uninstall(ctx *install.Context) error {
	ctx.Log("Uninstalling git wrapper...")

	binDir := filepath.Join(ctx.HomeDir, ".local", "bin")

	commonLibPath := filepath.Join(binDir, "devlog-git-common.sh")
	if _, err := os.Stat(commonLibPath); err == nil {
		if err := os.Remove(commonLibPath); err != nil {
			return fmt.Errorf("remove common library: %w", err)
		}
		ctx.Log("✓ Removed shared library from %s", commonLibPath)
	}

	wrapperPath := filepath.Join(binDir, "git")
	if _, err := os.Stat(wrapperPath); err == nil {
		content, err := os.ReadFile(wrapperPath)
		if err == nil && string(content) == gitWrapperScript {
			if err := os.Remove(wrapperPath); err != nil {
				return fmt.Errorf("remove git wrapper: %w", err)
			}
			ctx.Log("✓ Removed git wrapper from %s", wrapperPath)
		} else {
			ctx.Log("Warning: git wrapper at %s doesn't match devlog's wrapper, skipping removal", wrapperPath)
		}
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
