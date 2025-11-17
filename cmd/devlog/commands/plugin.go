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
			"install": {
				Name:        "install",
				Description: "Install and enable a plugin",
				Usage:       "devlog plugin install <name>",
				Examples: []string{
					"devlog plugin install llm_summarizer",
				},
			},
			"uninstall": {
				Name:        "uninstall",
				Description: "Uninstall and disable a plugin (preserves config)",
				Usage:       "devlog plugin uninstall [--purge] <name>",
				Examples: []string{
					"devlog plugin uninstall llm_summarizer",
					"devlog plugin uninstall --purge llm_summarizer",
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
	case "install", "init":
		return pluginInstall()
	case "uninstall":
		return pluginUninstall()
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

func pluginInstall() error {
	if len(os.Args) < 4 {
		return fmt.Errorf("usage: devlog plugin install <name>")
	}

	pluginName := os.Args[3]

	if pluginName == "help" {
		ShowHelp([]string{"plugin", "install"})
		return nil
	}

	plugin, err := plugins.Get(pluginName)
	if err != nil {
		return fmt.Errorf("plugin not found: %s", pluginName)
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w (run 'devlog init' first)", err)
	}

	if cfg.IsPluginEnabled(pluginName) {
		fmt.Printf("Plugin '%s' is already enabled\n", pluginName)
		return nil
	}

	fmt.Printf("Installing plugin: %s\n", pluginName)
	fmt.Printf("Description: %s\n", plugin.Description())
	fmt.Println()

	ctx := createPluginContext()

	if err := plugin.Install(ctx); err != nil {
		return fmt.Errorf("install plugin: %w", err)
	}

	cfg.SetPluginEnabled(pluginName, true)

	defaultCfg := plugin.DefaultConfig()
	if cfgMap, ok := defaultCfg.(map[string]interface{}); ok {
		cfg.SetPluginConfig(pluginName, cfgMap)
	}

	if err := cfg.Save(); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Println()
	fmt.Printf("✓ Plugin '%s' installed and enabled\n", pluginName)

	return nil
}

func pluginUninstall() error {
	if len(os.Args) < 4 {
		return fmt.Errorf("usage: devlog plugin uninstall [--purge] <name>")
	}

	var purge bool
	var pluginName string

	if os.Args[3] == "--purge" {
		if len(os.Args) < 5 {
			return fmt.Errorf("usage: devlog plugin uninstall --purge <name>")
		}
		purge = true
		pluginName = os.Args[4]
	} else {
		pluginName = os.Args[3]
	}

	if pluginName == "help" {
		ShowHelp([]string{"plugin", "uninstall"})
		return nil
	}

	plugin, err := plugins.Get(pluginName)
	if err != nil {
		return fmt.Errorf("plugin not found: %s", pluginName)
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if !cfg.IsPluginEnabled(pluginName) {
		fmt.Printf("Plugin '%s' is not enabled\n", pluginName)
		return nil
	}

	fmt.Printf("Uninstalling plugin: %s\n", pluginName)
	if purge {
		fmt.Println("Mode: purge (will remove all configuration)")
	} else {
		fmt.Println("Mode: standard (configuration will be preserved)")
	}
	fmt.Println()

	ctx := createPluginContext()

	if err := plugin.Uninstall(ctx); err != nil {
		return fmt.Errorf("uninstall plugin: %w", err)
	}

	if purge {
		cfg.ClearPluginConfig(pluginName)
	} else {
		cfg.SetPluginEnabled(pluginName, false)
	}

	if err := cfg.Save(); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Println()
	if purge {
		fmt.Printf("✓ Plugin '%s' uninstalled and configuration removed\n", pluginName)
	} else {
		fmt.Printf("✓ Plugin '%s' uninstalled\n", pluginName)
	}

	return nil
}

func createPluginContext() *plugins.InstallContext {
	homeDir, _ := os.UserHomeDir()
	configDir, _ := config.ConfigDir()
	dataDir, _ := config.DataDir()

	return &plugins.InstallContext{
		Interactive: true,
		ConfigDir:   configDir,
		DataDir:     dataDir,
		HomeDir:     homeDir,
		Log: func(format string, args ...interface{}) {
			fmt.Printf(format+"\n", args...)
		},
	}
}
