package commands

import (
	"github.com/urfave/cli/v2"
)

func IngestCommand() *cli.Command {
	return &cli.Command{
		Name:   "ingest",
		Usage:  "Manually ingest an event (developer/debug command)",
		Hidden: true,
		Subcommands: []*cli.Command{
			{
				Name:  "git-commit",
				Usage: "Ingest a git commit event (used by git hooks)",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "repo", Usage: "Repository path", Required: true},
					&cli.StringFlag{Name: "branch", Usage: "Branch name", Required: true},
					&cli.StringFlag{Name: "type", Usage: "Event type (commit, push, pull, fetch, merge, rebase, checkout, stash)", Required: true},
					&cli.StringFlag{Name: "hash", Usage: "Commit hash (for commit)"},
					&cli.StringFlag{Name: "message", Usage: "Commit message (for commit)"},
					&cli.StringFlag{Name: "author", Usage: "Commit author (for commit)"},
					&cli.StringFlag{Name: "remote", Usage: "Remote name (for push/pull/fetch)"},
					&cli.StringFlag{Name: "remote-url", Usage: "Remote URL (for push/pull/fetch)"},
					&cli.StringFlag{Name: "ref", Usage: "Reference (for push)"},
					&cli.StringFlag{Name: "commits", Usage: "Number of commits (for push)"},
					&cli.StringFlag{Name: "refs-updated", Usage: "Number of refs updated (for fetch)"},
					&cli.StringFlag{Name: "changes", Usage: "Type of changes (for pull)"},
					&cli.StringFlag{Name: "files-changed", Usage: "Number of files changed (for pull)"},
					&cli.StringFlag{Name: "merged-branch", Usage: "Merged branch name (for merge)"},
					&cli.StringFlag{Name: "target-branch", Usage: "Target branch (for rebase)"},
					&cli.StringFlag{Name: "from-branch", Usage: "Previous branch (for checkout)"},
					&cli.StringFlag{Name: "stash-action", Usage: "Stash action (for stash)"},
				},
				Action: ingestGitEventCli,
			},
			{
				Name:  "shell-command",
				Usage: "Ingest a shell command event (used by shell hooks)",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "command", Usage: "The shell command", Required: true},
					&cli.IntFlag{Name: "exit-code", Usage: "Command exit code", Value: 0},
					&cli.StringFlag{Name: "workdir", Usage: "Working directory"},
					&cli.Int64Flag{Name: "duration", Usage: "Command duration in milliseconds"},
				},
				Action: ingestShellCommandCli,
			},
			{
				Name:  "tmux-event",
				Usage: "Ingest a tmux event (used by tmux hooks)",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "type", Usage: "Event type (session, attach, detach, session-switch, window, pane)", Required: true},
					&cli.StringFlag{Name: "action", Usage: "Action (create, close, rename, split)"},
					&cli.StringFlag{Name: "session", Usage: "Session name"},
					&cli.StringFlag{Name: "window", Usage: "Window name"},
					&cli.StringFlag{Name: "window-id", Usage: "Window ID"},
					&cli.StringFlag{Name: "pane", Usage: "Pane index"},
					&cli.StringFlag{Name: "pane-id", Usage: "Pane ID"},
					&cli.StringFlag{Name: "client", Usage: "Client name"},
				},
				Action: ingestTmuxEventCli,
			},
		},
	}
}

func ingestGitEventCli(c *cli.Context) error {
	args := []string{
		"--repo", c.String("repo"),
		"--branch", c.String("branch"),
		"--type", c.String("type"),
	}
	if v := c.String("hash"); v != "" {
		args = append(args, "--hash", v)
	}
	if v := c.String("message"); v != "" {
		args = append(args, "--message", v)
	}
	if v := c.String("author"); v != "" {
		args = append(args, "--author", v)
	}
	if v := c.String("remote"); v != "" {
		args = append(args, "--remote", v)
	}
	if v := c.String("remote-url"); v != "" {
		args = append(args, "--remote-url", v)
	}
	if v := c.String("ref"); v != "" {
		args = append(args, "--ref", v)
	}
	if v := c.String("commits"); v != "" {
		args = append(args, "--commits", v)
	}
	if v := c.String("refs-updated"); v != "" {
		args = append(args, "--refs-updated", v)
	}
	if v := c.String("changes"); v != "" {
		args = append(args, "--changes", v)
	}
	if v := c.String("files-changed"); v != "" {
		args = append(args, "--files-changed", v)
	}
	if v := c.String("merged-branch"); v != "" {
		args = append(args, "--merged-branch", v)
	}
	if v := c.String("target-branch"); v != "" {
		args = append(args, "--target-branch", v)
	}
	if v := c.String("from-branch"); v != "" {
		args = append(args, "--from-branch", v)
	}
	if v := c.String("stash-action"); v != "" {
		args = append(args, "--stash-action", v)
	}
	return ingestGitEvent(args)
}

func ingestShellCommandCli(c *cli.Context) error {
	args := []string{"--command", c.String("command")}
	if c.IsSet("exit-code") {
		args = append(args, "--exit-code", c.String("exit-code"))
	}
	if v := c.String("workdir"); v != "" {
		args = append(args, "--workdir", v)
	}
	if c.IsSet("duration") {
		args = append(args, "--duration", c.String("duration"))
	}
	return ingestShellCommand(args)
}

func ingestTmuxEventCli(c *cli.Context) error {
	args := []string{"--type", c.String("type")}
	if v := c.String("action"); v != "" {
		args = append(args, "--action", v)
	}
	if v := c.String("session"); v != "" {
		args = append(args, "--session", v)
	}
	if v := c.String("window"); v != "" {
		args = append(args, "--window", v)
	}
	if v := c.String("window-id"); v != "" {
		args = append(args, "--window-id", v)
	}
	if v := c.String("pane"); v != "" {
		args = append(args, "--pane", v)
	}
	if v := c.String("pane-id"); v != "" {
		args = append(args, "--pane-id", v)
	}
	if v := c.String("client"); v != "" {
		args = append(args, "--client", v)
	}
	return ingestTmuxEvent(args)
}
