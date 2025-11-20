package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"devlog/internal/config"
	"devlog/internal/presentation"
	"devlog/internal/services"
	"devlog/internal/storage"

	"github.com/urfave/cli/v2"
)

func SearchCommand() *cli.Command {
	return &cli.Command{
		Name:        "search",
		Usage:       "Search events using full-text search with advanced filters",
		UsageText:   "devlog search [options] [query]",
		Description: "Search your development history. Note: options must come before the query.\n\n   Examples:\n      devlog search --since 2h \"error\"\n      devlog search --module git --type commit \"fix\"\n      devlog search --repo myproject \"auth\"",
		ArgsUsage:   "[query]",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "verbose",
				Aliases: []string{"v"},
				Usage:   "Show detailed event information",
			},
			&cli.IntFlag{
				Name:    "number",
				Aliases: []string{"n"},
				Value:   20,
				Usage:   "Number of results to display",
			},
			&cli.StringFlag{
				Name:  "since",
				Usage: "Show events since duration ago (e.g., '2h', '30m', '1d')",
			},
			&cli.StringSliceFlag{
				Name:    "module",
				Aliases: []string{"m"},
				Usage:   "Filter by source module (can be repeated)",
			},
			&cli.StringSliceFlag{
				Name:    "type",
				Aliases: []string{"t"},
				Usage:   "Filter by event type (can be repeated)",
			},
			&cli.StringFlag{
				Name:  "repo",
				Usage: "Filter by repository pattern",
			},
			&cli.StringFlag{
				Name:  "branch",
				Usage: "Filter by branch pattern",
			},
			&cli.StringFlag{
				Name:    "sort",
				Value:   "time_asc",
				Usage:   "Sort order: time_asc, time_desc, relevance",
				Aliases: []string{"o"},
			},
			&cli.StringFlag{
				Name:    "format",
				Value:   "table",
				Usage:   "Output format: table, json",
				Aliases: []string{"f"},
			},
		},
		Action: func(c *cli.Context) error {
			query := "*"
			if c.NArg() > 0 {
				query = c.Args().First()
			}

			return executeSearch(c, query)
		},
	}
}

func parseDuration(s string) (time.Duration, error) {
	if strings.HasSuffix(s, "d") {
		days := strings.TrimSuffix(s, "d")
		if days == "" {
			return 0, fmt.Errorf("invalid duration: missing number before 'd'")
		}
		var d int
		_, err := fmt.Sscanf(days, "%d", &d)
		if err != nil {
			return 0, fmt.Errorf("invalid duration: %w", err)
		}
		if d < 0 {
			return 0, fmt.Errorf("invalid duration: negative value not allowed")
		}
		return time.Duration(d) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

func executeSearch(c *cli.Context, query string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	dataDir, err := config.DataDir()
	if err != nil {
		return err
	}

	dbPath := filepath.Join(dataDir, "events.db")

	store, err := storage.New(dbPath)
	if err != nil {
		return err
	}
	defer store.Close()

	eventService := services.NewEventService(store, func() *config.Config { return cfg }, nil)

	ctx := context.Background()

	searchOpts := storage.SearchOptions{
		Query:         query,
		Limit:         c.Int("number"),
		Modules:       c.StringSlice("module"),
		Types:         c.StringSlice("type"),
		RepoPattern:   c.String("repo"),
		BranchPattern: c.String("branch"),
	}

	if since := c.String("since"); since != "" {
		duration, err := parseDuration(since)
		if err != nil {
			return fmt.Errorf("invalid since duration: %w", err)
		}
		afterTime := time.Now().Add(-duration)
		searchOpts.After = &afterTime
	}

	switch c.String("sort") {
	case "relevance":
		searchOpts.SortOrder = storage.SortByRelevance
	case "time_desc":
		searchOpts.SortOrder = storage.SortByTimeDesc
	case "time_asc":
		searchOpts.SortOrder = storage.SortByTimeAsc
	default:
		return fmt.Errorf("invalid sort order: %s (must be time_asc, time_desc, or relevance)", c.String("sort"))
	}

	results, err := eventService.SearchEvents(ctx, searchOpts)
	if err != nil {
		return err
	}

	var format presentation.OutputFormat
	switch c.String("format") {
	case "table":
		format = presentation.FormatTable
	case "json":
		format = presentation.FormatJSON
	default:
		return fmt.Errorf("invalid format: %s (must be table or json)", c.String("format"))
	}

	presenter := presentation.NewSearchPresenter(os.Stdout, format, c.Bool("verbose"))
	return presenter.Present(results, query)
}
