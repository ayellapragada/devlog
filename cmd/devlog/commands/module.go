package commands

import (
	"fmt"
	"os"

	"devlog/internal/config"
	"devlog/internal/modules"
)

func Module() error {
	if len(os.Args) < 3 {
		fmt.Println("Usage:")
		fmt.Println("  devlog module list")
		fmt.Println("  devlog module install <name>")
		fmt.Println("  devlog module uninstall <name>")
		return fmt.Errorf("missing module subcommand")
	}

	subcommand := os.Args[2]

	switch subcommand {
	case "list":
		return moduleList()
	case "install", "init":
		return moduleInstall()
	case "uninstall":
		return moduleUninstall()
	default:
		fmt.Fprintf(os.Stderr, "Unknown module subcommand: %s\n", subcommand)
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
		return fmt.Errorf("usage: devlog module uninstall <name>")
	}

	moduleName := os.Args[3]

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
	fmt.Println()

	ctx := createModuleContext()

	if err := mod.Uninstall(ctx); err != nil {
		return fmt.Errorf("uninstall module: %w", err)
	}

	cfg.SetModuleEnabled(moduleName, false)

	if err := cfg.Save(); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Println()
	fmt.Printf("✓ Module '%s' uninstalled and disabled\n", moduleName)

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
