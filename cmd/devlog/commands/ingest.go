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

func Ingest() error {
	if len(os.Args) < 3 {
		fmt.Println("Usage:")
		fmt.Println("  devlog ingest git-commit [flags]")
		fmt.Println("  devlog ingest shell-command [flags]")
		return fmt.Errorf("missing ingest subcommand")
	}

	subcommand := os.Args[2]

	switch subcommand {
	case "git-commit":
		return ingestGitCommit()
	case "shell-command":
		return ingestShellCommand()
	default:
		fmt.Fprintf(os.Stderr, "Unknown ingest subcommand: %s\n", subcommand)
		return fmt.Errorf("unknown ingest subcommand: %s", subcommand)
	}
}

func ingestGitCommit() error {
	fs := flag.NewFlagSet("git-commit", flag.ExitOnError)
	repo := fs.String("repo", "", "Repository path")
	branch := fs.String("branch", "", "Branch name")
	hash := fs.String("hash", "", "Commit hash")
	message := fs.String("message", "", "Commit message")
	author := fs.String("author", "", "Commit author")

	fs.Parse(os.Args[3:])

	if *repo == "" || *branch == "" || *hash == "" {
		return fmt.Errorf("--repo, --branch, and --hash are required")
	}

	event := events.NewEvent(events.SourceGit, events.TypeCommit)
	event.Repo = *repo
	event.Branch = *branch
	event.Payload["hash"] = *hash
	if *message != "" {
		event.Payload["message"] = *message
	}
	if *author != "" {
		event.Payload["author"] = *author
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
