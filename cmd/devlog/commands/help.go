package commands

import (
	"fmt"
	"strings"
)

type CommandDefinition struct {
	Name        string
	Description string
	Usage       string
	Subcommands map[string]*CommandDefinition
	Flags       []FlagDefinition
	Examples    []string
}

type FlagDefinition struct {
	Short       string
	Long        string
	Description string
}

var CommandRegistry = map[string]*CommandDefinition{}

func RegisterCommand(name string, cmd *CommandDefinition) {
	CommandRegistry[name] = cmd
}

func ShowHelp(args []string) {
	if len(args) == 0 {
		showMainHelp(false)
		return
	}

	showAll := false
	if args[0] == "--all" || args[0] == "-a" {
		showAll = true
		if len(args) == 1 {
			showMainHelp(showAll)
			return
		}
		args = args[1:]
	}

	commandName := args[0]
	cmd, exists := CommandRegistry[commandName]
	if !exists {
		fmt.Printf("Unknown command: %s\n\n", commandName)
		showMainHelp(false)
		return
	}

	if len(args) == 1 {
		showCommandHelp(cmd)
		return
	}

	subcommandName := args[1]
	if subcommandName == "help" {
		showCommandHelp(cmd)
		return
	}

	if cmd.Subcommands != nil {
		subcmd, exists := cmd.Subcommands[subcommandName]
		if exists {
			showSubcommandHelp(cmd, subcmd)
			return
		}
	}

	showCommandHelp(cmd)
}

func showMainHelp(showAll bool) {
	fmt.Println("DevLog - Development journaling system")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  devlog <command> [subcommand] [flags]")
	fmt.Println("  devlog <command> help")
	fmt.Println()
	fmt.Println("Commands:")
	printCommand("  init", "Initialize config and database")
	printCommand("  daemon", "Manage the daemon process")
	printCommand("  config", "Manage configuration")
	printCommand("  module", "Manage modules")
	printCommand("  plugin", "Manage plugins")
	printCommand("  session", "Manage development sessions")
	printCommand("  status", "Show recent events")

	if showAll {
		fmt.Println()
		fmt.Println("Developer/Debug Commands:")
		printCommand("  ingest", "Manually ingest events")
		printCommand("  poll", "Manually poll a module")
		printCommand("  flush", "Process queued events")
	}

	fmt.Println()
	if showAll {
		fmt.Println("For more information on a command, use: devlog <command> help")
	} else {
		fmt.Println("For more information on a command, use: devlog <command> help")
		fmt.Println("For developer/debug commands, use: devlog help --all")
	}
}

func showCommandHelp(cmd *CommandDefinition) {
	fmt.Printf("Name:\n  %s - %s\n\n", cmd.Name, cmd.Description)
	fmt.Printf("Usage:\n  %s\n\n", cmd.Usage)

	if len(cmd.Subcommands) > 0 {
		fmt.Println("Subcommands:")
		for name, subcmd := range cmd.Subcommands {
			printCommand("  "+name, subcmd.Description)
		}
		fmt.Println()
	}

	if len(cmd.Flags) > 0 {
		fmt.Println("Flags:")
		for _, flag := range cmd.Flags {
			if flag.Short != "" {
				fmt.Printf("  %s, %s\n", flag.Short, flag.Long)
			} else {
				fmt.Printf("  %s\n", flag.Long)
			}
			fmt.Printf("      %s\n", flag.Description)
		}
		fmt.Println()
	}

	if len(cmd.Examples) > 0 {
		fmt.Println("Examples:")
		for _, example := range cmd.Examples {
			fmt.Printf("  %s\n", example)
		}
		fmt.Println()
	}

	if len(cmd.Subcommands) > 0 {
		fmt.Printf("For more information on a subcommand, use: devlog %s <subcommand> help\n", cmd.Name)
	}
}

func showSubcommandHelp(parent *CommandDefinition, cmd *CommandDefinition) {
	fmt.Printf("Name:\n  %s %s - %s\n\n", parent.Name, cmd.Name, cmd.Description)
	fmt.Printf("Usage:\n  %s\n\n", cmd.Usage)

	if len(cmd.Flags) > 0 {
		fmt.Println("Flags:")
		for _, flag := range cmd.Flags {
			if flag.Short != "" {
				fmt.Printf("  %s, %s\n", flag.Short, flag.Long)
			} else {
				fmt.Printf("  %s\n", flag.Long)
			}
			fmt.Printf("      %s\n", flag.Description)
		}
		fmt.Println()
	}

	if len(cmd.Examples) > 0 {
		fmt.Println("Examples:")
		for _, example := range cmd.Examples {
			fmt.Printf("  %s\n", example)
		}
		fmt.Println()
	}
}

func printCommand(name, description string) {
	const maxNameWidth = 20
	padding := maxNameWidth - len(name)
	if padding < 2 {
		padding = 2
	}
	fmt.Printf("%s%s%s\n", name, strings.Repeat(" ", padding), description)
}
