package shell

import (
	"flag"
	"fmt"

	"devlog/internal/config"
	"devlog/internal/events"
	"devlog/internal/ingest"

	"github.com/urfave/cli/v2"
)

type IngestHandler struct{}

func (h *IngestHandler) CLICommand() *cli.Command {
	return &cli.Command{
		Name:  "shell-command",
		Usage: "Ingest a shell command event (used by shell hooks)",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "command", Usage: "The shell command", Required: true},
			&cli.IntFlag{Name: "exit-code", Usage: "Command exit code", Value: 0},
			&cli.StringFlag{Name: "workdir", Usage: "Working directory"},
			&cli.StringFlag{Name: "branch", Usage: "Git branch"},
			&cli.Int64Flag{Name: "duration", Usage: "Command duration in milliseconds"},
		},
		Action: h.handle,
	}
}

func (h *IngestHandler) handle(c *cli.Context) error {
	args := []string{"--command", c.String("command")}
	if c.IsSet("exit-code") {
		args = append(args, "--exit-code", c.String("exit-code"))
	}
	if v := c.String("workdir"); v != "" {
		args = append(args, "--workdir", v)
	}
	if v := c.String("branch"); v != "" {
		args = append(args, "--branch", v)
	}
	if c.IsSet("duration") {
		args = append(args, "--duration", c.String("duration"))
	}
	return h.ingestEvent(args)
}

func (h *IngestHandler) ingestEvent(args []string) error {
	fs := flag.NewFlagSet("shell-command", flag.ExitOnError)
	command := fs.String("command", "", "The shell command")
	exitCode := fs.Int("exit-code", 0, "Command exit code")
	workdir := fs.String("workdir", "", "Working directory")
	branch := fs.String("branch", "", "Git branch")
	duration := fs.Int64("duration", 0, "Command duration in milliseconds")

	fs.Parse(args)

	if *command == "" {
		return fmt.Errorf("--command is required")
	}

	cfg, err := config.Load()
	if err == nil {
		if !cfg.ShouldCaptureCommand(*command) {
			return nil
		}
	}

	event := events.NewEvent(string(events.SourceShell), string(events.TypeCommand))
	event.Payload["command"] = *command
	event.Payload["exit_code"] = *exitCode

	if *workdir != "" {
		event.Payload["workdir"] = *workdir
		if repoPath, err := ingest.FindGitRepo(*workdir); err == nil {
			event.Repo = repoPath
		}
	}

	if *branch != "" {
		event.Branch = *branch
	}

	if *duration > 0 {
		event.Payload["duration_ms"] = *duration
	}

	return ingest.SendEvent(event)
}

func init() {
	ingest.Register("shell", &IngestHandler{})
}
