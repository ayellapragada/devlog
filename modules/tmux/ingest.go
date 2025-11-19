package tmux

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
		Action: h.handle,
	}
}

func (h *IngestHandler) handle(c *cli.Context) error {
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
	return h.ingestEvent(args)
}

func (h *IngestHandler) ingestEvent(args []string) error {
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

	return ingest.SendEvent(event)
}

func init() {
	ingest.Register("tmux", &IngestHandler{})
}
