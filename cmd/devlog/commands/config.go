package commands

import (
	"fmt"
	"os"

	"devlog/internal/config"
	"devlog/internal/modules"
)

func Config() error {
	if len(os.Args) < 3 {
		fmt.Println("Usage:")
		fmt.Println("  devlog config status")
		return fmt.Errorf("missing config subcommand")
	}

	subcommand := os.Args[2]

	switch subcommand {
	case "status":
		return configStatus()
	default:
		fmt.Fprintf(os.Stderr, "Unknown config subcommand: %s\n", subcommand)
		return fmt.Errorf("unknown config subcommand: %s", subcommand)
	}
}

func configStatus() error {
	cfg, err := config.Load()
	if err != nil {
		configPath, _ := config.ConfigPath()
		if _, statErr := os.Stat(configPath); os.IsNotExist(statErr) {
			fmt.Println("Configuration Status")
			fmt.Println("===================")
			fmt.Println()
			fmt.Println("Status: Not initialized")
			fmt.Println()
			fmt.Println("Run 'devlog init' to initialize devlog")
			return nil
		}
		return err
	}

	configPath, _ := config.ConfigPath()
	dataDir, _ := config.DataDir()

	fmt.Println("Configuration Status")
	fmt.Println("===================")
	fmt.Println()
	fmt.Printf("Config file: %s\n", configPath)
	fmt.Printf("Data directory: %s\n", dataDir)
	fmt.Printf("Obsidian path: %s\n", cfg.ObsidianPath)
	fmt.Printf("HTTP port: %d\n", cfg.HTTP.Port)
	fmt.Println()

	fmt.Println("Modules:")
	allModules := modules.List()
	if len(allModules) == 0 {
		fmt.Println("  No modules available")
	} else {
		for _, mod := range allModules {
			enabled := cfg.IsModuleEnabled(mod.Name())
			status := "disabled"
			if enabled {
				status = "enabled"
			}
			fmt.Printf("  [%s] %s - %s\n", status, mod.Name(), mod.Description())
		}
	}

	fmt.Println()
	fmt.Println("Use 'devlog module install <name>' to enable a module")

	return nil
}
