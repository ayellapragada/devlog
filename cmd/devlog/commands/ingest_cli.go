package commands

import (
	"devlog/internal/ingest"

	_ "devlog/modules/git"
	_ "devlog/modules/kubectl"
	_ "devlog/modules/shell"
	_ "devlog/modules/tmux"

	"github.com/urfave/cli/v2"
)

func IngestCommand() *cli.Command {
	return &cli.Command{
		Name:        "ingest",
		Usage:       "Manually ingest an event (developer/debug command)",
		Hidden:      true,
		Subcommands: ingest.GetCommands(),
	}
}
