package commands

import (
	"fmt"
	"os"

	"devlog/internal/config"
	"devlog/internal/plugins"
)

func init() {
	RegisterCommand("plugin", &CommandDefinition{
		Name:        "plugin",
		Description: "Manage devlog plugins",
		Usage:       "devlog plugin <subcommand>",
		Subcommands: map[string]*CommandDefinition{
			"list": {
				Name:        "list",
				Description: "List all available plugins and their status",
				Usage:       "devlog plugin list",
				Examples: []string{
					"devlog plugin list",
				},
			},
		},
	})
}

func Plugin() error {
	if len(os.Args) < 3 {
		ShowHelp([]string{"plugin"})
		return fmt.Errorf("missing plugin subcommand")
	}

	subcommand := os.Args[2]

	if subcommand == "help" {
		ShowHelp([]string{"plugin"})
		return nil
	}

	switch subcommand {
	case "list":
		if len(os.Args) > 3 && os.Args[3] == "help" {
			ShowHelp([]string{"plugin", "list"})
			return nil
		}
		return pluginList()
	default:
		fmt.Fprintf(os.Stderr, "Unknown plugin subcommand: %s\n\n", subcommand)
		ShowHelp([]string{"plugin"})
		return fmt.Errorf("unknown plugin subcommand: %s", subcommand)
	}
}

func pluginList() error {
	fmt.Println("Available Plugins")
	fmt.Println("=================")
	fmt.Println()

	allPlugins := plugins.List()
	if len(allPlugins) == 0 {
		fmt.Println("No plugins available")
		return nil
	}

	cfg, err := config.Load()
	var showStatus bool
	if err == nil {
		showStatus = true
	}

	for _, plugin := range allPlugins {
		status := ""
		if showStatus {
			if cfg.IsPluginEnabled(plugin.Name()) {
				status = " [enabled]"
			} else {
				status = " [disabled]"
			}
		}
		fmt.Printf("  %s%s\n", plugin.Name(), status)
		fmt.Printf("    %s\n", plugin.Description())
		fmt.Println()
	}

	if !showStatus {
		fmt.Println("Run 'devlog init' to initialize configuration")
	}

	return nil
}
