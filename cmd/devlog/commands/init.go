package commands

import (
	"fmt"
	"path/filepath"

	"devlog/internal/config"
	"devlog/internal/modules"
	"devlog/internal/storage"
)

func init() {
	RegisterCommand("init", &CommandDefinition{
		Name:        "init",
		Description: "Initialize devlog configuration and database",
		Usage:       "devlog init",
		Examples: []string{
			"devlog init",
		},
	})
}

func Init() error {
	fmt.Println("Initializing devlog...")

	if err := config.InitConfig(); err != nil {
		return err
	}

	dataDir, err := config.DataDir()
	if err != nil {
		return err
	}

	dbPath := filepath.Join(dataDir, "events.db")
	if err := storage.InitDB(dbPath); err != nil {
		return err
	}

	fmt.Println("\nInitialization complete!")
	fmt.Println("\nNext steps:")
	fmt.Println("  1. Install modules to enable event capture:")

	allModules := modules.List()
	for _, mod := range allModules {
		fmt.Printf("     - %s: %s\n", mod.Name(), mod.Description())
	}

	fmt.Println()
	fmt.Println("     Install modules with: devlog module install <name>")
	fmt.Println()
	fmt.Println("  2. Start the daemon:")
	fmt.Println("     devlog daemon start")

	return nil
}
