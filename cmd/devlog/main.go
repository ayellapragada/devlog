package main

import (
	"log"
	"os"
	"slices"

	"devlog/cmd/devlog/commands"
	"devlog/internal/config"

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
	coreCommands := []*cli.Command{
		commands.InitCommand(),
		commands.ConfigCommand(),
		commands.DaemonCommand(),
		commands.StatusCommand(),
		commands.SearchCommand(),
		commands.ModuleCommand(),
		commands.PluginCommand(),
		commands.WebCommand(),
		commands.VersionCommand(),
	}

	for _, cmd := range coreCommands {
		cmd.Category = "CORE"
	}

	cfg, err := config.Load()
	var pluginCommands []*cli.Command

	if err == nil && cfg.IsPluginEnabled("query") {
		pluginCommands = append(pluginCommands, commands.QueryCommand())
	}

	if err == nil && cfg.IsPluginEnabled("summarizer") {
		pluginCommands = append(pluginCommands, commands.SummarizerCommand())
	}

	for _, cmd := range pluginCommands {
		cmd.Category = "PLUGIN"
		cmd.Hidden = false
	}

	internalCommands := []*cli.Command{
		commands.IngestCommand(),
		commands.PollCommand(),
	}

	showInternal := slices.Contains(os.Args, "--show-internal")

	for _, cmd := range internalCommands {
		cmd.Category = "INTERNAL"
		cmd.Hidden = !showInternal
	}

	allCommands := make([]*cli.Command, 0, len(coreCommands)+len(pluginCommands)+len(internalCommands))
	allCommands = append(allCommands, coreCommands...)
	allCommands = append(allCommands, pluginCommands...)
	allCommands = append(allCommands, internalCommands...)

	app := &cli.App{
		Name:     "devlog",
		Usage:    "An automated development journaling system",
		Commands: allCommands,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:   "show-internal",
				Usage:  "Show internal/debug commands in help output",
				Hidden: true,
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
