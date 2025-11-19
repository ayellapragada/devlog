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
				status = " [enabled]"
			} else {
				status = " [disabled]"
			}
		}
		fmt.Printf("  %s%s\n", component.Name(), status)
		fmt.Printf("    %s\n", component.Description())
		fmt.Println()
	}

	if !showStatus {
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
		return fmt.Errorf("usage: devlog %s install <name>", componentType)
	}

	componentName := args[0]

	component, err := registry.Get(componentName)
	if err != nil {
		return fmt.Errorf("%s not found: %s", componentType, componentName)
	}

	if configOps.IsEnabled(componentName) {
		fmt.Printf("%s '%s' is already enabled\n", capitalize(componentType), componentName)
		return nil
	}

	fmt.Printf("Installing %s: %s\n", componentType, componentName)
	fmt.Printf("Description: %s\n", component.Description())
	fmt.Println()

	ctx := createInstallContext()

	if err := component.Install(ctx); err != nil {
		return fmt.Errorf("install %s: %w", componentType, err)
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

	if err := configOps.Save(); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Println()
	fmt.Printf("✓ %s '%s' installed and enabled\n", capitalize(componentType), componentName)

	return nil
}

func componentUninstall(
	componentType string,
	args []string,
	registry ComponentRegistry,
	configOps ComponentConfig,
) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: devlog %s uninstall [--purge] <name>", componentType)
	}

	var purge bool
	var componentName string

	if args[0] == "--purge" {
		if len(args) < 2 {
			return fmt.Errorf("usage: devlog %s uninstall --purge <name>", componentType)
		}
		purge = true
		componentName = args[1]
	} else {
		componentName = args[0]
	}

	component, err := registry.Get(componentName)
	if err != nil {
		return fmt.Errorf("%s not found: %s", componentType, componentName)
	}

	if !configOps.IsEnabled(componentName) {
		fmt.Printf("%s '%s' is not enabled\n", capitalize(componentType), componentName)
		return nil
	}

	fmt.Printf("Uninstalling %s: %s\n", componentType, componentName)
	if purge {
		fmt.Println("Mode: purge (will remove all configuration)")
	} else {
		fmt.Println("Mode: standard (configuration will be preserved)")
	}
	fmt.Println()

	ctx := createInstallContext()

	if err := component.Uninstall(ctx); err != nil {
		return fmt.Errorf("uninstall %s: %w", componentType, err)
	}

	if purge {
		configOps.ClearConfig(componentName)
	} else {
		configOps.SetEnabled(componentName, false)
	}

	if err := configOps.Save(); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Println()
	if purge {
		fmt.Printf("✓ %s '%s' uninstalled and configuration removed\n", capitalize(componentType), componentName)
	} else {
		fmt.Printf("✓ %s '%s' uninstalled\n", capitalize(componentType), componentName)
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
