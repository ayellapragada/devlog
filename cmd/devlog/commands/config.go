package commands

import (
	"fmt"
	"os"
	"os/exec"

	"devlog/internal/config"
	"devlog/internal/modules"
	"devlog/internal/plugins"

	"github.com/urfave/cli/v2"
)

func ConfigCommand() *cli.Command {
	return &cli.Command{
		Name:  "config",
		Usage: "Manage devlog configuration",
		Subcommands: []*cli.Command{
			{
				Name:  "show",
				Usage: "Display configuration",
				Action: func(c *cli.Context) error {
					path, err := config.ConfigPath()
					if err != nil {
						return fmt.Errorf("get config path: %w", err)
					}

					data, err := os.ReadFile(path)
					if err != nil {
						if os.IsNotExist(err) {
							return fmt.Errorf("config file not found at %s (run 'devlog init' to create)", path)
						}
						return fmt.Errorf("read config file: %w", err)
					}

					fmt.Print(string(data))
					return nil
				},
			},
			{
				Name:  "path",
				Usage: "Show config file path",
				Action: func(c *cli.Context) error {
					path, err := config.ConfigPath()
					if err != nil {
						return fmt.Errorf("get config path: %w", err)
					}

					fmt.Println(path)
					return nil
				},
			},
			{
				Name:  "edit",
				Usage: "Edit config in $EDITOR",
				Action: func(c *cli.Context) error {
					path, err := config.ConfigPath()
					if err != nil {
						return fmt.Errorf("get config path: %w", err)
					}

					if _, err := os.Stat(path); os.IsNotExist(err) {
						return fmt.Errorf("config file not found at %s (run 'devlog init' to create)", path)
					}

					editor := os.Getenv("EDITOR")
					if editor == "" {
						editor = "vi"
					}

					cmd := exec.Command(editor, path)
					cmd.Stdin = os.Stdin
					cmd.Stdout = os.Stdout
					cmd.Stderr = os.Stderr

					if err := cmd.Run(); err != nil {
						return fmt.Errorf("failed to run editor: %w", err)
					}

					cfg, err := config.Load()
					if err != nil {
						return fmt.Errorf("validate config after edit: %w", err)
					}

					if err := cfg.Validate(); err != nil {
						return fmt.Errorf("config validation failed: %w", err)
					}

					fmt.Println("Configuration validated successfully")
					return nil
				},
			},
			{
				Name:  "status",
				Usage: "Show current configuration status and enabled modules",
				Action: func(c *cli.Context) error {
					return configStatus()
				},
			},
		},
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
	fmt.Println("Plugins:")
	allPlugins := plugins.List()
	if len(allPlugins) == 0 {
		fmt.Println("  No plugins installed")
	} else {
		for _, plugin := range allPlugins {
			enabled := cfg.IsPluginEnabled(plugin.Name())
			status := "disabled"
			if enabled {
				status = "enabled"
			}
			fmt.Printf("  [%s] %s - %s\n", status, plugin.Name(), plugin.Description())
		}
	}

	fmt.Println()
	fmt.Println("Use 'devlog module install <name>' to enable a module")
	fmt.Println("Use 'devlog plugin install <name>' to enable a plugin")

	return nil
}
