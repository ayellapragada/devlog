package commands

import (
	"fmt"
	"os"

	"devlog/internal/config"
	"devlog/internal/modules"
)

func init() {
	RegisterCommand("config", &CommandDefinition{
		Name:        "config",
		Description: "Manage devlog configuration",
		Usage:       "devlog config <subcommand>",
		Subcommands: map[string]*CommandDefinition{
			"status": {
				Name:        "status",
				Description: "Show current configuration status and enabled modules",
				Usage:       "devlog config status",
				Examples: []string{
					"devlog config status",
				},
			},
		},
	})
}

func Config() error {
	if len(os.Args) < 3 {
		ShowHelp([]string{"config"})
		return fmt.Errorf("missing config subcommand")
	}

	subcommand := os.Args[2]

	if subcommand == "help" {
		ShowHelp([]string{"config"})
		return nil
	}

	switch subcommand {
	case "status":
		if len(os.Args) > 3 && os.Args[3] == "help" {
			ShowHelp([]string{"config", "status"})
			return nil
		}
		return configStatus()
	default:
		fmt.Fprintf(os.Stderr, "Unknown config subcommand: %s\n\n", subcommand)
		ShowHelp([]string{"config"})
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
