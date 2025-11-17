package commands

import (
	"bytes"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"devlog/internal/config"
	"devlog/internal/daemon"
	"devlog/internal/events"
	"devlog/internal/queue"
)

func init() {
	RegisterCommand("ingest", &CommandDefinition{
		Name:        "ingest",
		Description: "Manually ingest an event (developer/debug command)",
		Usage:       "devlog ingest <subcommand> [flags]",
		Subcommands: map[string]*CommandDefinition{
			"git-commit": {
				Name:        "git-commit",
				Description: "Ingest a git commit event (used by git hooks)",
				Usage:       "devlog ingest git-commit [flags]",
				Examples: []string{
					"devlog ingest git-commit",
				},
			},
			"shell-command": {
				Name:        "shell-command",
				Description: "Ingest a shell command event (used by shell hooks)",
				Usage:       "devlog ingest shell-command [flags]",
				Examples: []string{
					"devlog ingest shell-command",
				},
			},
			"tmux-event": {
				Name:        "tmux-event",
				Description: "Ingest a tmux event (used by tmux hooks)",
				Usage:       "devlog ingest tmux-event [flags]",
				Examples: []string{
					"devlog ingest tmux-event --type=session --action=create --session=dev",
				},
			},
		},
	})
}

func Ingest() error {
	if len(os.Args) < 3 {
		ShowHelp([]string{"ingest"})
		return fmt.Errorf("missing ingest subcommand")
	}

	subcommand := os.Args[2]

	if subcommand == "help" {
		ShowHelp([]string{"ingest"})
		return nil
	}

	switch subcommand {
	case "git-event", "git-commit":
		return ingestGitEvent()
	case "shell-command":
		return ingestShellCommand()
	case "tmux-event":
		return ingestTmuxEvent()
	default:
		fmt.Fprintf(os.Stderr, "Unknown ingest subcommand: %s\n\n", subcommand)
		ShowHelp([]string{"ingest"})
		return fmt.Errorf("unknown ingest subcommand: %s", subcommand)
	}
}

func ingestGitEvent() error {
	fs := flag.NewFlagSet("git-event", flag.ExitOnError)
	repo := fs.String("repo", "", "Repository path")
	branch := fs.String("branch", "", "Branch name")
	eventType := fs.String("type", "", "Event type (commit, push, pull, fetch, merge, rebase, checkout, stash)")

	hash := fs.String("hash", "", "Commit hash (for commit)")
	message := fs.String("message", "", "Commit message (for commit)")
	author := fs.String("author", "", "Commit author (for commit)")

	remote := fs.String("remote", "", "Remote name (for push/pull/fetch)")
	ref := fs.String("ref", "", "Reference (for push)")
	mergedBranch := fs.String("merged-branch", "", "Merged branch name (for merge)")
	targetBranch := fs.String("target-branch", "", "Target branch (for rebase)")
	fromBranch := fs.String("from-branch", "", "Previous branch (for checkout)")
	stashAction := fs.String("stash-action", "", "Stash action (for stash)")

	fs.Parse(os.Args[3:])

	if *repo == "" || *branch == "" || *eventType == "" {
		return fmt.Errorf("--repo, --branch, and --type are required")
	}

	var typeConstant string
	switch *eventType {
	case "commit":
		typeConstant = events.TypeCommit
		if *hash == "" {
			return fmt.Errorf("--hash is required for commit events")
		}
	case "push":
		typeConstant = events.TypePush
	case "pull":
		typeConstant = events.TypePull
	case "fetch":
		typeConstant = events.TypeFetch
	case "merge":
		typeConstant = events.TypeMerge
	case "rebase":
		typeConstant = events.TypeRebase
	case "checkout":
		typeConstant = events.TypeCheckout
	case "stash":
		typeConstant = events.TypeStash
	default:
		return fmt.Errorf("unknown event type: %s", *eventType)
	}

	event := events.NewEvent(events.SourceGit, typeConstant)
	event.Repo = *repo
	event.Branch = *branch

	if *hash != "" {
		event.Payload["hash"] = *hash
	}
	if *message != "" {
		event.Payload["message"] = *message
	}
	if *author != "" {
		event.Payload["author"] = *author
	}
	if *remote != "" {
		event.Payload["remote"] = *remote
	}
	if *ref != "" {
		event.Payload["ref"] = *ref
	}
	if *mergedBranch != "" {
		event.Payload["merged_branch"] = *mergedBranch
	}
	if *targetBranch != "" {
		event.Payload["target_branch"] = *targetBranch
	}
	if *fromBranch != "" {
		event.Payload["from_branch"] = *fromBranch
	}
	if *stashAction != "" {
		event.Payload["stash_action"] = *stashAction
	}

	return sendEvent(event)
}

func ingestShellCommand() error {
	fs := flag.NewFlagSet("shell-command", flag.ExitOnError)
	command := fs.String("command", "", "The shell command")
	exitCode := fs.Int("exit-code", 0, "Command exit code")
	workdir := fs.String("workdir", "", "Working directory")
	duration := fs.Int64("duration", 0, "Command duration in milliseconds")

	fs.Parse(os.Args[3:])

	if *command == "" {
		return fmt.Errorf("--command is required")
	}

	event := events.NewEvent(events.SourceShell, events.TypeCommand)
	event.Payload["command"] = *command
	event.Payload["exit_code"] = *exitCode

	if *workdir != "" {
		event.Payload["workdir"] = *workdir
		if repoPath, err := FindGitRepo(*workdir); err == nil {
			event.Repo = repoPath
		}
	}

	if *duration > 0 {
		event.Payload["duration_ms"] = *duration
	}

	return sendEvent(event)
}

func sendEvent(event *events.Event) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if daemon.IsRunning() {
		eventJSON, err := event.ToJSON()
		if err != nil {
			return fmt.Errorf("serialize event: %w", err)
		}

		url := fmt.Sprintf("http://127.0.0.1:%d/api/v1/ingest", cfg.HTTP.Port)
		resp, err := http.Post(url, "application/json", bytes.NewReader(eventJSON))
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
	}

	queueDir, err := config.QueueDir()
	if err != nil {
		return fmt.Errorf("get queue directory: %w", err)
	}

	q, err := queue.New(queueDir)
	if err != nil {
		return fmt.Errorf("create queue: %w", err)
	}

	if err := q.Enqueue(event); err != nil {
		return fmt.Errorf("queue event: %w", err)
	}

	return nil
}

func ingestTmuxEvent() error {
	fs := flag.NewFlagSet("tmux-event", flag.ExitOnError)
	eventType := fs.String("type", "", "Event type (session, attach, detach, session-switch, window, pane)")
	action := fs.String("action", "", "Action (create, close, rename, split)")
	session := fs.String("session", "", "Session name")
	window := fs.String("window", "", "Window name")
	windowID := fs.String("window-id", "", "Window ID")
	pane := fs.String("pane", "", "Pane index")
	paneID := fs.String("pane-id", "", "Pane ID")
	client := fs.String("client", "", "Client name")

	fs.Parse(os.Args[3:])

	if *eventType == "" {
		return fmt.Errorf("--type is required")
	}

	var typeConstant string
	switch *eventType {
	case "session":
		typeConstant = events.TypeTmuxSession
	case "attach":
		typeConstant = events.TypeTmuxAttach
	case "detach":
		typeConstant = events.TypeTmuxDetach
	case "session-switch":
		typeConstant = events.TypeContextSwitch
	case "window":
		typeConstant = events.TypeTmuxWindow
	case "pane":
		typeConstant = events.TypeTmuxPane
	default:
		return fmt.Errorf("unknown event type: %s", *eventType)
	}

	event := events.NewEvent(events.SourceTmux, typeConstant)

	if *session != "" {
		event.Payload["session"] = *session
	}
	if *action != "" {
		event.Payload["action"] = *action
	}
	if *window != "" {
		event.Payload["window"] = *window
	}
	if *windowID != "" {
		event.Payload["window_id"] = *windowID
	}
	if *pane != "" {
		event.Payload["pane"] = *pane
	}
	if *paneID != "" {
		event.Payload["pane_id"] = *paneID
	}
	if *client != "" {
		event.Payload["client"] = *client
	}

	return sendEvent(event)
}

func FindGitRepo(path string) (string, error) {
	current := path
	for {
		gitPath := filepath.Join(current, ".git")
		if _, err := os.Stat(gitPath); err == nil {
			return current, nil
		}

		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("not a git repository")
		}
		current = parent
	}
}
