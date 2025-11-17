package commands

import (
	"fmt"
	"os"

	"devlog/internal/config"
	"devlog/internal/modules"
)

func init() {
	RegisterCommand("module", &CommandDefinition{
		Name:        "module",
		Description: "Manage devlog modules",
		Usage:       "devlog module <subcommand>",
		Subcommands: map[string]*CommandDefinition{
			"list": {
				Name:        "list",
				Description: "List all available modules and their status",
				Usage:       "devlog module list",
				Examples: []string{
					"devlog module list",
				},
			},
			"install": {
				Name:        "install",
				Description: "Install and enable a module",
				Usage:       "devlog module install <name>",
				Examples: []string{
					"devlog module install git",
					"devlog module install shell",
				},
			},
			"uninstall": {
				Name:        "uninstall",
				Description: "Uninstall and disable a module (preserves config)",
				Usage:       "devlog module uninstall [--purge] <name>",
				Examples: []string{
					"devlog module uninstall git",
					"devlog module uninstall --purge git",
				},
			},
		},
	})
}

func Module() error {
	if len(os.Args) < 3 {
		ShowHelp([]string{"module"})
		return fmt.Errorf("missing module subcommand")
	}

	subcommand := os.Args[2]

	if subcommand == "help" {
		ShowHelp([]string{"module"})
		return nil
	}

	switch subcommand {
	case "list":
		if len(os.Args) > 3 && os.Args[3] == "help" {
			ShowHelp([]string{"module", "list"})
			return nil
		}
		return moduleList()
	case "install", "init":
		return moduleInstall()
	case "uninstall":
		return moduleUninstall()
	default:
		fmt.Fprintf(os.Stderr, "Unknown module subcommand: %s\n\n", subcommand)
		ShowHelp([]string{"module"})
		return fmt.Errorf("unknown module subcommand: %s", subcommand)
	}
}

func moduleList() error {
	fmt.Println("Available Modules")
	fmt.Println("================")
	fmt.Println()

	allModules := modules.List()
	if len(allModules) == 0 {
		fmt.Println("No modules available")
		return nil
	}

	cfg, err := config.Load()
	var showStatus bool
	if err == nil {
		showStatus = true
	}

	for _, mod := range allModules {
		status := ""
		if showStatus {
			if cfg.IsModuleEnabled(mod.Name()) {
				status = " [enabled]"
			} else {
				status = " [disabled]"
			}
		}
		fmt.Printf("  %s%s\n", mod.Name(), status)
		fmt.Printf("    %s\n", mod.Description())
		fmt.Println()
	}

	if !showStatus {
		fmt.Println("Run 'devlog init' to initialize configuration")
	}

	return nil
}

func moduleInstall() error {
	if len(os.Args) < 4 {
		return fmt.Errorf("usage: devlog module install <name>")
	}

	moduleName := os.Args[3]

	if moduleName == "help" {
		ShowHelp([]string{"module", "install"})
		return nil
	}

	mod, err := modules.Get(moduleName)
	if err != nil {
		return fmt.Errorf("module not found: %s", moduleName)
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w (run 'devlog init' first)", err)
	}

	if cfg.IsModuleEnabled(moduleName) {
		fmt.Printf("Module '%s' is already enabled\n", moduleName)
		return nil
	}

	fmt.Printf("Installing module: %s\n", moduleName)
	fmt.Printf("Description: %s\n", mod.Description())
	fmt.Println()

	ctx := createModuleContext()

	if err := mod.Install(ctx); err != nil {
		return fmt.Errorf("install module: %w", err)
	}

	cfg.SetModuleEnabled(moduleName, true)

	defaultCfg := mod.DefaultConfig()
	if cfgMap, ok := defaultCfg.(map[string]interface{}); ok {
		cfg.SetModuleConfig(moduleName, cfgMap)
	}

	if err := cfg.Save(); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Println()
	fmt.Printf("✓ Module '%s' installed and enabled\n", moduleName)

	return nil
}

func moduleUninstall() error {
	if len(os.Args) < 4 {
		return fmt.Errorf("usage: devlog module uninstall [--purge] <name>")
	}

	var purge bool
	var moduleName string

	if os.Args[3] == "--purge" {
		if len(os.Args) < 5 {
			return fmt.Errorf("usage: devlog module uninstall --purge <name>")
		}
		purge = true
		moduleName = os.Args[4]
	} else {
		moduleName = os.Args[3]
	}

	if moduleName == "help" {
		ShowHelp([]string{"module", "uninstall"})
		return nil
	}

	mod, err := modules.Get(moduleName)
	if err != nil {
		return fmt.Errorf("module not found: %s", moduleName)
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if !cfg.IsModuleEnabled(moduleName) {
		fmt.Printf("Module '%s' is not enabled\n", moduleName)
		return nil
	}

	fmt.Printf("Uninstalling module: %s\n", moduleName)
	if purge {
		fmt.Println("Mode: purge (will remove all configuration)")
	} else {
		fmt.Println("Mode: standard (configuration will be preserved)")
	}
	fmt.Println()

	ctx := createModuleContext()

	if err := mod.Uninstall(ctx); err != nil {
		return fmt.Errorf("uninstall module: %w", err)
	}

	if purge {
		cfg.ClearModuleConfig(moduleName)
	} else {
		cfg.SetModuleEnabled(moduleName, false)
	}

	if err := cfg.Save(); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Println()
	if purge {
		fmt.Printf("✓ Module '%s' uninstalled and configuration removed\n", moduleName)
	} else {
		fmt.Printf("✓ Module '%s' uninstalled\n", moduleName)
	}

	return nil
}

func createModuleContext() *modules.InstallContext {
	homeDir, _ := os.UserHomeDir()
	configDir, _ := config.ConfigDir()
	dataDir, _ := config.DataDir()

	return &modules.InstallContext{
		Interactive: true,
		ConfigDir:   configDir,
		DataDir:     dataDir,
		HomeDir:     homeDir,
		Log: func(format string, args ...interface{}) {
			fmt.Printf(format+"\n", args...)
		},
	}
}
