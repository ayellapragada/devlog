package main

import (
	"fmt"
	"os"

	"devlog/cmd/devlog/commands"

	_ "devlog/modules/git"
	_ "devlog/modules/shell"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 {
		printUsage()
		return nil
	}

	command := os.Args[1]

	switch command {
	case "init":
		return commands.Init()
	case "daemon":
		return commands.Daemon()
	case "ingest":
		return commands.Ingest()
	case "poll":
		return commands.Poll()
	case "status":
		return commands.Status()
	case "flush":
		return commands.Flush()
	case "session":
		return commands.Session()
	case "config":
		return commands.Config()
	case "module":
		return commands.Module()
	case "help":
		printUsage()
		return nil
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", command)
		printUsage()
		return fmt.Errorf("unknown command: %s", command)
	}
}

func printUsage() {
	fmt.Println("DevLog - Development journaling system")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  devlog init                          Initialize config and database")
	fmt.Println("  devlog config status                 Show configuration status")
	fmt.Println("  devlog module list                   List available modules")
	fmt.Println("  devlog module install <name>         Install and enable a module")
	fmt.Println("  devlog module uninstall <name>       Uninstall and disable a module")
	fmt.Println("  devlog daemon start                  Start the daemon")
	fmt.Println("  devlog daemon stop                   Stop the daemon")
	fmt.Println("  devlog daemon status                 Check daemon status")
	fmt.Println("  devlog status                        Show recent events")
	fmt.Println("  devlog session create --events <ids> Create session from event IDs")
	fmt.Println("  devlog session list                  List all sessions")
	fmt.Println("  devlog help                          Show this help message")
	fmt.Println()
	fmt.Println("Developer/Debug Commands:")
	fmt.Println("  devlog ingest git-commit [flags]     Ingest a git commit event (used by hooks)")
	fmt.Println("  devlog ingest shell-command [flags]  Ingest a shell command event (used by hooks)")
	fmt.Println("  devlog poll <module>                 Manually poll a module (for testing)")
	fmt.Println("  devlog flush                         Process queued events")
}
