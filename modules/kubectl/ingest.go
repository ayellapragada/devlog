package kubectl

import (
	"flag"
	"fmt"

	"devlog/internal/events"
	"devlog/internal/ingest"

	"github.com/urfave/cli/v2"
)

type IngestHandler struct{}

func (h *IngestHandler) CLICommand() *cli.Command {
	return &cli.Command{
		Name:  "kubectl",
		Usage: "Ingest a kubectl event (used by kubectl wrapper)",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "operation", Usage: "Operation type (apply, create, delete, get, describe, edit, patch, logs, exec, debug)", Required: true},
			&cli.StringFlag{Name: "context", Usage: "Kubectl context", Required: true},
			&cli.StringFlag{Name: "cluster", Usage: "Cluster name"},
			&cli.StringFlag{Name: "namespace", Usage: "Namespace", Required: true},
			&cli.StringFlag{Name: "resource-type", Usage: "Resource type (pod, deployment, service, etc.)"},
			&cli.StringFlag{Name: "resource-names", Usage: "Resource names"},
			&cli.StringFlag{Name: "resource-count", Usage: "Number of resources affected"},
			&cli.StringFlag{Name: "workdir", Usage: "Working directory"},
			&cli.IntFlag{Name: "exit-code", Usage: "Command exit code", Value: 0},
		},
		Action: h.handle,
	}
}

func (h *IngestHandler) handle(c *cli.Context) error {
	args := []string{
		"--operation", c.String("operation"),
		"--context", c.String("context"),
		"--namespace", c.String("namespace"),
	}
	if v := c.String("cluster"); v != "" {
		args = append(args, "--cluster", v)
	}
	if v := c.String("resource-type"); v != "" {
		args = append(args, "--resource-type", v)
	}
	if v := c.String("resource-names"); v != "" {
		args = append(args, "--resource-names", v)
	}
	if v := c.String("resource-count"); v != "" {
		args = append(args, "--resource-count", v)
	}
	if v := c.String("workdir"); v != "" {
		args = append(args, "--workdir", v)
	}
	if c.IsSet("exit-code") {
		args = append(args, "--exit-code", c.String("exit-code"))
	}
	return h.ingestEvent(args)
}

func (h *IngestHandler) ingestEvent(args []string) error {
	fs := flag.NewFlagSet("kubectl-event", flag.ExitOnError)
	operation := fs.String("operation", "", "Operation type")
	context := fs.String("context", "", "Kubectl context")
	cluster := fs.String("cluster", "", "Cluster name")
	namespace := fs.String("namespace", "", "Namespace")
	resourceType := fs.String("resource-type", "", "Resource type")
	resourceNames := fs.String("resource-names", "", "Resource names")
	resourceCount := fs.String("resource-count", "", "Number of resources affected")
	workdir := fs.String("workdir", "", "Working directory")
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

	if *workdir != "" {
		event.Payload["workdir"] = *workdir
		if repoPath, err := ingest.FindGitRepo(*workdir); err == nil {
			event.Repo = repoPath
			if branch, err := ingest.FindGitBranch(*workdir); err == nil {
				event.Branch = branch
			}
		}
	}

	return ingest.SendEvent(event)
}

func init() {
	ingest.Register("kubectl", &IngestHandler{})
}
