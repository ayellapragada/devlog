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
		commands.ShowHelp([]string{})
		return nil
	}

	command := os.Args[1]

	if command == "help" {
		if len(os.Args) > 2 {
			commands.ShowHelp(os.Args[2:])
		} else {
			commands.ShowHelp([]string{})
		}
		return nil
	}

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
		verbose := false
		limit := 10
		source := ""

		for i := 2; i < len(os.Args); i++ {
			if os.Args[i] == "help" {
				commands.ShowHelp([]string{"status"})
				return nil
			} else if os.Args[i] == "--verbose" || os.Args[i] == "-v" {
				verbose = true
			} else if os.Args[i] == "-n" || os.Args[i] == "--number" {
				if i+1 < len(os.Args) {
					if n, err := fmt.Sscanf(os.Args[i+1], "%d", &limit); err == nil && n == 1 {
						i++
					}
				}
			} else if os.Args[i] == "-s" || os.Args[i] == "--source" {
				if i+1 < len(os.Args) {
					source = os.Args[i+1]
					i++
				}
			}
		}

		return commands.Status(verbose, limit, source)
	case "flush":
		return commands.Flush()
	case "session":
		return commands.Session()
	case "config":
		return commands.Config()
	case "module":
		return commands.Module()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", command)
		commands.ShowHelp([]string{})
		return fmt.Errorf("unknown command: %s", command)
	}
}
