package main

import (
	"log"
	"os"

	"devlog/cmd/devlog/commands"

	"github.com/urfave/cli/v2"

	_ "devlog/modules/claude"
	_ "devlog/modules/git"
	_ "devlog/modules/kubectl"
	_ "devlog/modules/shell"
	_ "devlog/modules/tmux"
	_ "devlog/modules/wisprflow"

	_ "devlog/plugins/llm"
	_ "devlog/plugins/query"
	_ "devlog/plugins/summarizer"
)

func main() {
	app := &cli.App{
		Name:  "devlog",
		Usage: "An automated development journaling system",
		Commands: []*cli.Command{
			commands.InitCommand(),
			commands.ConfigCommand(),
			commands.DaemonCommand(),
			commands.StatusCommand(),
			commands.SearchCommand(),
			commands.QueryCommand(),
			commands.ModuleCommand(),
			commands.PluginCommand(),
			commands.IngestCommand(),
			commands.PollCommand(),
			commands.WebCommand(),
			commands.VersionCommand(),
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
