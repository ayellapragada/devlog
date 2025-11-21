package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"devlog/internal/config"
	"devlog/internal/install"
)

type Component interface {
	Name() string
	Description() string
	Install(ctx *install.Context) error
	Uninstall(ctx *install.Context) error
	DefaultConfig() interface{}
	ValidateConfig(config interface{}) error
}

type ComponentRegistry interface {
	Get(name string) (Component, error)
	List() []Component
}

type ComponentConfig interface {
	IsEnabled(name string) bool
	GetConfig(name string) (map[string]interface{}, bool)
	SetEnabled(name string, enabled bool)
	SetConfig(name string, config map[string]interface{})
	ClearConfig(name string)
	Save() error
}

func componentList(pluralName string, registry ComponentRegistry, configOps ComponentConfig) error {
	title := fmt.Sprintf("Available %s", pluralName)
	separator := ""
	for range title {
		separator += "="
	}

	fmt.Println(title)
	fmt.Println(separator)
	fmt.Println()

	allComponents := registry.List()
	if len(allComponents) == 0 {
		fmt.Printf("No %s available\n", pluralName)
		return nil
	}

	showStatus := configOps != nil

	for _, component := range allComponents {
		status := ""
		if showStatus {
			if configOps.IsEnabled(component.Name()) {
				status = " [✓] "
			} else {
				status = " [ ] "
			}
		}
		fmt.Printf("  %s%-12s %s\n", status, component.Name(), component.Description())
	}

	if !showStatus {
		fmt.Println()
		fmt.Println("Run 'devlog init' to initialize configuration")
	}

	return nil
}

func componentInstall(
	componentType string,
	args []string,
	registry ComponentRegistry,
	configOps ComponentConfig,
) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: devlog %s install <name> [name...]", componentType)
	}

	ctx := createInstallContext()
	var failed []string
	var succeeded []string

	for _, componentName := range args {
		component, err := registry.Get(componentName)
		if err != nil {
			fmt.Printf("✗ %s not found: %s\n", capitalize(componentType), componentName)
			failed = append(failed, componentName)
			continue
		}

		if configOps.IsEnabled(componentName) {
			fmt.Printf("  %s '%s' is already enabled\n", capitalize(componentType), componentName)
			continue
		}

		fmt.Printf("Installing %s: %s\n", componentType, componentName)

		if err := component.Install(ctx); err != nil {
			fmt.Printf("✗ Failed to install %s '%s': %v\n", componentType, componentName, err)
			failed = append(failed, componentName)
			continue
		}

		configOps.SetEnabled(componentName, true)

		if defaultCfg := component.DefaultConfig(); defaultCfg != nil {
			if cfgMap, ok := defaultCfg.(map[string]interface{}); ok {
				configOps.SetConfig(componentName, cfgMap)
			} else {
				data, _ := json.Marshal(defaultCfg)
				var cfgMap map[string]interface{}
				if json.Unmarshal(data, &cfgMap) == nil {
					configOps.SetConfig(componentName, cfgMap)
				}
			}
		}

		fmt.Printf("✓ %s '%s' installed and enabled\n", capitalize(componentType), componentName)
		succeeded = append(succeeded, componentName)
	}

	if len(succeeded) > 0 {
		if err := configOps.Save(); err != nil {
			return fmt.Errorf("save config: %w", err)
		}
	}

	fmt.Println()
	if len(succeeded) > 0 {
		fmt.Printf("Successfully installed %d %s(s)\n", len(succeeded), componentType)
	}
	if len(failed) > 0 {
		fmt.Printf("Failed to install %d %s(s)\n", len(failed), componentType)
		return fmt.Errorf("some %ss failed to install", componentType)
	}

	return nil
}

func componentUninstall(
	componentType string,
	args []string,
	registry ComponentRegistry,
	configOps ComponentConfig,
) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: devlog %s uninstall [--purge] <name> [name...]", componentType)
	}

	var purge bool
	var componentNames []string

	if args[0] == "--purge" {
		if len(args) < 2 {
			return fmt.Errorf("usage: devlog %s uninstall --purge <name> [name...]", componentType)
		}
		purge = true
		componentNames = args[1:]
	} else {
		componentNames = args
	}

	if purge {
		fmt.Println("Mode: purge (will remove all configuration)")
	} else {
		fmt.Println("Mode: standard (configuration will be preserved)")
	}
	fmt.Println()

	ctx := createInstallContext()
	var failed []string
	var succeeded []string

	for _, componentName := range componentNames {
		component, err := registry.Get(componentName)
		if err != nil {
			fmt.Printf("✗ %s not found: %s\n", capitalize(componentType), componentName)
			failed = append(failed, componentName)
			continue
		}

		if !configOps.IsEnabled(componentName) {
			fmt.Printf("  %s '%s' is not enabled\n", capitalize(componentType), componentName)
			continue
		}

		fmt.Printf("Uninstalling %s: %s\n", componentType, componentName)

		if err := component.Uninstall(ctx); err != nil {
			fmt.Printf("✗ Failed to uninstall %s '%s': %v\n", componentType, componentName, err)
			failed = append(failed, componentName)
			continue
		}

		if purge {
			configOps.ClearConfig(componentName)
		} else {
			configOps.SetEnabled(componentName, false)
		}

		if purge {
			fmt.Printf("✓ %s '%s' uninstalled and configuration removed\n", capitalize(componentType), componentName)
		} else {
			fmt.Printf("✓ %s '%s' uninstalled\n", capitalize(componentType), componentName)
		}
		succeeded = append(succeeded, componentName)
	}

	if len(succeeded) > 0 {
		if err := configOps.Save(); err != nil {
			return fmt.Errorf("save config: %w", err)
		}
	}

	fmt.Println()
	if len(succeeded) > 0 {
		fmt.Printf("Successfully uninstalled %d %s(s)\n", len(succeeded), componentType)
	}
	if len(failed) > 0 {
		fmt.Printf("Failed to uninstall %d %s(s)\n", len(failed), componentType)
		return fmt.Errorf("some %ss failed to uninstall", componentType)
	}

	return nil
}

func createInstallContext() *install.Context {
	homeDir, _ := os.UserHomeDir()
	configDir, _ := config.ConfigDir()
	dataDir, _ := config.DataDir()

	return &install.Context{
		Interactive: true,
		ConfigDir:   configDir,
		DataDir:     dataDir,
		HomeDir:     homeDir,
		Log: func(format string, args ...interface{}) {
			fmt.Printf(format+"\n", args...)
		},
	}
}

func capitalize(s string) string {
	if len(s) == 0 {
		return s
	}
	return string(s[0]-32) + s[1:]
}
