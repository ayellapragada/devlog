package git

import (
	"flag"
	"fmt"
	"strings"

	"devlog/internal/events"
	"devlog/internal/ingest"

	"github.com/urfave/cli/v2"
)

type IngestHandler struct{}

func (h *IngestHandler) CLICommand() *cli.Command {
	return &cli.Command{
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
		Action: h.handle,
	}
}

func (h *IngestHandler) handle(c *cli.Context) error {
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
	return h.ingestEvent(args)
}

func (h *IngestHandler) ingestEvent(args []string) error {
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

	return ingest.SendEvent(event)
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

func init() {
	ingest.Register("git", &IngestHandler{})
}
