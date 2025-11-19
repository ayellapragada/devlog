package kubectl

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"devlog/internal/install"
	"devlog/internal/modules"
)

//go:embed hooks/kubectl-wrapper.sh
var kubectlWrapperScript string

//go:embed hooks/devlog-kubectl-common.sh
var kubectlCommonLib string

type Module struct{}

func (m *Module) Name() string {
	return "kubectl"
}

func (m *Module) Description() string {
	return "Capture kubectl operations (apply, delete, get, logs, exec, debug, etc.) automatically"
}

func (m *Module) Install(ctx *install.Context) error {
	ctx.Log("Installing kubectl command wrapper...")

	binDir := filepath.Join(ctx.HomeDir, ".local", "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return &modules.InstallError{
			Component: "kubectl wrapper",
			File:      binDir,
			Err:       err,
			RecoverySteps: []string{
				fmt.Sprintf("Check directory permissions: ls -la %s", filepath.Dir(binDir)),
				fmt.Sprintf("Try creating manually: mkdir -p %s", binDir),
				"Check disk space: df -h",
			},
		}
	}

	commonLibPath := filepath.Join(binDir, "devlog-kubectl-common.sh")
	if err := os.WriteFile(commonLibPath, []byte(kubectlCommonLib), 0644); err != nil {
		return &modules.InstallError{
			Component: "kubectl wrapper",
			File:      commonLibPath,
			Err:       err,
			RecoverySteps: []string{
				fmt.Sprintf("Check file permissions: ls -la %s", filepath.Dir(commonLibPath)),
				fmt.Sprintf("Ensure directory exists: mkdir -p %s", filepath.Dir(commonLibPath)),
				"Check if file is write-protected",
			},
		}
	}

	wrapperPath := filepath.Join(binDir, "kubectl")
	if err := os.WriteFile(wrapperPath, []byte(kubectlWrapperScript), 0755); err != nil {
		return &modules.InstallError{
			Component: "kubectl wrapper",
			File:      wrapperPath,
			Err:       err,
			RecoverySteps: []string{
				fmt.Sprintf("Check file permissions: ls -la %s", filepath.Dir(wrapperPath)),
				"Ensure directory exists and is writable",
				fmt.Sprintf("Try manual install: Save the wrapper script to %s and chmod +x %s", wrapperPath, wrapperPath),
			},
		}
	}

	kWrapperPath := filepath.Join(binDir, "k")
	if err := os.WriteFile(kWrapperPath, []byte(kubectlWrapperScript), 0755); err != nil {
		return &modules.InstallError{
			Component: "k alias wrapper",
			File:      kWrapperPath,
			Err:       err,
			RecoverySteps: []string{
				fmt.Sprintf("Check file permissions: ls -la %s", filepath.Dir(kWrapperPath)),
				"Ensure directory exists and is writable",
				fmt.Sprintf("Try manual install: Save the wrapper script to %s and chmod +x %s", kWrapperPath, kWrapperPath),
			},
		}
	}

	ctx.Log("✓ Installed shared library to %s", commonLibPath)
	ctx.Log("✓ Installed kubectl wrapper to %s", wrapperPath)
	ctx.Log("✓ Installed k alias wrapper to %s", kWrapperPath)
	ctx.Log("")
	ctx.Log("All kubectl/k operations (apply, delete, get, logs, exec, debug, etc.) will now be tracked.")
	ctx.Log("")
	ctx.Log("IMPORTANT: Ensure %s is in your PATH and appears BEFORE /usr/local/bin", binDir)
	ctx.Log("Add this to your shell RC file:")
	ctx.Log("")
	ctx.Log("  export PATH=\"%s:$PATH\"", binDir)
	ctx.Log("")
	ctx.Log("Then restart your shell or run: source ~/.zshrc (or ~/.bashrc)")

	return nil
}

func (m *Module) Uninstall(ctx *install.Context) error {
	ctx.Log("Uninstalling kubectl wrapper...")

	binDir := filepath.Join(ctx.HomeDir, ".local", "bin")

	commonLibPath := filepath.Join(binDir, "devlog-kubectl-common.sh")
	if _, err := os.Stat(commonLibPath); err == nil {
		if err := os.Remove(commonLibPath); err != nil {
			return fmt.Errorf("remove common library: %w", err)
		}
		ctx.Log("✓ Removed shared library from %s", commonLibPath)
	}

	wrapperPath := filepath.Join(binDir, "kubectl")
	if _, err := os.Stat(wrapperPath); err == nil {
		content, err := os.ReadFile(wrapperPath)
		if err == nil && string(content) == kubectlWrapperScript {
			if err := os.Remove(wrapperPath); err != nil {
				return fmt.Errorf("remove kubectl wrapper: %w", err)
			}
			ctx.Log("✓ Removed kubectl wrapper from %s", wrapperPath)
		} else {
			ctx.Log("Warning: kubectl wrapper at %s doesn't match devlog's wrapper, skipping removal", wrapperPath)
		}
	}

	kWrapperPath := filepath.Join(binDir, "k")
	if _, err := os.Stat(kWrapperPath); err == nil {
		content, err := os.ReadFile(kWrapperPath)
		if err == nil && string(content) == kubectlWrapperScript {
			if err := os.Remove(kWrapperPath); err != nil {
				return fmt.Errorf("remove k wrapper: %w", err)
			}
			ctx.Log("✓ Removed k alias wrapper from %s", kWrapperPath)
		} else {
			ctx.Log("Warning: k wrapper at %s doesn't match devlog's wrapper, skipping removal", kWrapperPath)
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
