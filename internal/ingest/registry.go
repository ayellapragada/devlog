package ingest

import (
	"github.com/urfave/cli/v2"
)

type Handler interface {
	CLICommand() *cli.Command
}

var registry = make(map[string]Handler)

func Register(name string, handler Handler) {
	registry[name] = handler
}

func GetHandlers() map[string]Handler {
	return registry
}

func GetCommands() []*cli.Command {
	commands := make([]*cli.Command, 0, len(registry))
	for _, handler := range registry {
		commands = append(commands, handler.CLICommand())
	}
	return commands
}
