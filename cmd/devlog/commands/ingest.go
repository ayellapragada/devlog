package commands

import (
	"bytes"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"devlog/internal/config"
	"devlog/internal/daemon"
	"devlog/internal/events"
	"devlog/internal/queue"
)

func ingestGitEvent(args []string) error {
	fs := flag.NewFlagSet("git-event", flag.ExitOnError)
	repo := fs.String("repo", "", "Repository path")
	branch := fs.String("branch", "", "Branch name")
	eventType := fs.String("type", "", "Event type (commit, push, pull, fetch, merge, rebase, checkout, stash)")

	hash := fs.String("hash", "", "Commit hash (for commit)")
	message := fs.String("message", "", "Commit message (for commit)")
	author := fs.String("author", "", "Commit author (for commit)")

	remote := fs.String("remote", "", "Remote name (for push/pull/fetch)")
	remoteURL := fs.String("remote-url", "", "Remote URL (for push/pull/fetch)")
	ref := fs.String("ref", "", "Reference (for push)")
	commits := fs.String("commits", "", "Number of commits (for push)")
	refsUpdated := fs.String("refs-updated", "", "Number of refs updated (for fetch)")
	changes := fs.String("changes", "", "Type of changes (for pull)")
	filesChanged := fs.String("files-changed", "", "Number of files changed (for pull)")
	mergedBranch := fs.String("merged-branch", "", "Merged branch name (for merge)")
	targetBranch := fs.String("target-branch", "", "Target branch (for rebase)")
	fromBranch := fs.String("from-branch", "", "Previous branch (for checkout)")
	stashAction := fs.String("stash-action", "", "Stash action (for stash)")

	fs.Parse(args)

	if *repo == "" || *branch == "" || *eventType == "" {
		return fmt.Errorf("--repo, --branch, and --type are required")
	}

	var typeConstant string
	switch *eventType {
	case "commit":
		typeConstant = string(events.TypeCommit)
		if *hash == "" {
			return fmt.Errorf("--hash is required for commit events")
		}
	case "push":
		typeConstant = string(events.TypePush)
	case "pull":
		typeConstant = string(events.TypePull)
	case "fetch":
		typeConstant = string(events.TypeFetch)
	case "merge":
		typeConstant = string(events.TypeMerge)
	case "rebase":
		typeConstant = string(events.TypeRebase)
	case "checkout":
		typeConstant = string(events.TypeCheckout)
	case "stash":
		typeConstant = string(events.TypeStash)
	default:
		return fmt.Errorf("unknown event type: %s", *eventType)
	}

	event := events.NewEvent(string(events.SourceGit), typeConstant)
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
	if *remoteURL != "" {
		event.Payload["remote_url"] = *remoteURL
		event.Payload["hosting_provider"] = detectHostingProvider(*remoteURL)
	}
	if *ref != "" {
		event.Payload["ref"] = *ref
	}
	if *commits != "" {
		event.Payload["commits"] = *commits
	}
	if *refsUpdated != "" {
		event.Payload["refs_updated"] = *refsUpdated
	}
	if *changes != "" {
		event.Payload["changes"] = *changes
	}
	if *filesChanged != "" {
		event.Payload["files_changed"] = *filesChanged
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

func ingestShellCommand(args []string) error {
	fs := flag.NewFlagSet("shell-command", flag.ExitOnError)
	command := fs.String("command", "", "The shell command")
	exitCode := fs.Int("exit-code", 0, "Command exit code")
	workdir := fs.String("workdir", "", "Working directory")
	duration := fs.Int64("duration", 0, "Command duration in milliseconds")

	fs.Parse(args)

	if *command == "" {
		return fmt.Errorf("--command is required")
	}

	event := events.NewEvent(string(events.SourceShell), string(events.TypeCommand))
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

func ingestTmuxEvent(args []string) error {
	fs := flag.NewFlagSet("tmux-event", flag.ExitOnError)
	eventType := fs.String("type", "", "Event type (session, attach, detach, session-switch, window, pane)")
	action := fs.String("action", "", "Action (create, close, rename, split)")
	session := fs.String("session", "", "Session name")
	window := fs.String("window", "", "Window name")
	windowID := fs.String("window-id", "", "Window ID")
	pane := fs.String("pane", "", "Pane index")
	paneID := fs.String("pane-id", "", "Pane ID")
	client := fs.String("client", "", "Client name")

	fs.Parse(args)

	if *eventType == "" {
		return fmt.Errorf("--type is required")
	}

	var typeConstant string
	switch *eventType {
	case "session":
		typeConstant = string(events.TypeTmuxSession)
	case "attach":
		typeConstant = string(events.TypeTmuxAttach)
	case "detach":
		typeConstant = string(events.TypeTmuxDetach)
	case "session-switch":
		typeConstant = string(events.TypeContextSwitch)
	case "window":
		typeConstant = string(events.TypeTmuxWindow)
	case "pane":
		typeConstant = string(events.TypeTmuxPane)
	default:
		return fmt.Errorf("unknown event type: %s", *eventType)
	}

	event := events.NewEvent(string(events.SourceTmux), typeConstant)

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

func detectHostingProvider(remoteURL string) string {
	url := strings.ToLower(remoteURL)

	if strings.Contains(url, "github.com") {
		return "github"
	}
	if strings.Contains(url, "gitlab.com") || strings.Contains(url, "gitlab") {
		return "gitlab"
	}
	if strings.Contains(url, "bitbucket.org") || strings.Contains(url, "bitbucket") {
		return "bitbucket"
	}
	if strings.Contains(url, "azure.com") || strings.Contains(url, "visualstudio.com") || strings.Contains(url, "dev.azure.com") {
		return "azure"
	}
	if strings.Contains(url, "sourceforge") {
		return "sourceforge"
	}
	if strings.Contains(url, "gitea") {
		return "gitea"
	}
	if strings.Contains(url, "codeberg.org") {
		return "codeberg"
	}

	return "other"
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

func ingestKubectlEvent(args []string) error {
	fs := flag.NewFlagSet("kubectl-event", flag.ExitOnError)
	operation := fs.String("operation", "", "Operation type")
	context := fs.String("context", "", "Kubectl context")
	cluster := fs.String("cluster", "", "Cluster name")
	namespace := fs.String("namespace", "", "Namespace")
	resourceType := fs.String("resource-type", "", "Resource type")
	resourceNames := fs.String("resource-names", "", "Resource names")
	resourceCount := fs.String("resource-count", "", "Number of resources affected")
	exitCode := fs.Int("exit-code", 0, "Command exit code")

	fs.Parse(args)

	if *operation == "" || *context == "" || *namespace == "" {
		return fmt.Errorf("--operation, --context, and --namespace are required")
	}

	var typeConstant string
	switch *operation {
	case "apply":
		typeConstant = string(events.TypeKubectlApply)
	case "create":
		typeConstant = string(events.TypeKubectlCreate)
	case "delete":
		typeConstant = string(events.TypeKubectlDelete)
	case "get":
		typeConstant = string(events.TypeKubectlGet)
	case "describe":
		typeConstant = string(events.TypeKubectlDescribe)
	case "edit":
		typeConstant = string(events.TypeKubectlEdit)
	case "patch":
		typeConstant = string(events.TypeKubectlPatch)
	case "logs":
		typeConstant = string(events.TypeKubectlLogs)
	case "exec":
		typeConstant = string(events.TypeKubectlExec)
	case "debug":
		typeConstant = string(events.TypeKubectlDebug)
	default:
		return fmt.Errorf("unknown operation type: %s", *operation)
	}

	event := events.NewEvent(string(events.SourceKubectl), typeConstant)
	event.Payload["context"] = *context
	event.Payload["namespace"] = *namespace
	event.Payload["exit_code"] = *exitCode

	if *cluster != "" {
		event.Payload["cluster"] = *cluster
	}
	if *resourceType != "" {
		event.Payload["resource_type"] = *resourceType
	}
	if *resourceNames != "" {
		event.Payload["resource_names"] = *resourceNames
	}
	if *resourceCount != "" {
		event.Payload["resource_count"] = *resourceCount
	}

	return sendEvent(event)
}
