package main

import (
	"log"
	"os"

	"devlog/cmd/devlog/commands"

	"github.com/urfave/cli/v2"

	_ "devlog/modules/claude"
	_ "devlog/modules/git"
	_ "devlog/modules/shell"
	_ "devlog/modules/tmux"
	_ "devlog/modules/wisprflow"
)

func main() {
	app := &cli.App{
		Name:  "devlog",
		Usage: "Development journaling system",
		Commands: []*cli.Command{
			commands.InitCommand(),
			commands.ConfigCommand(),
			commands.DaemonCommand(),
			commands.StatusCommand(),
			commands.FlushCommand(),
			commands.SearchCommand(),
			commands.ModuleCommand(),
			commands.PluginCommand(),
			commands.IngestCommand(),
			commands.PollCommand(),
			commands.WebCommand(),
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
