package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"devlog/internal/config"
	"devlog/internal/presentation"
	"devlog/internal/services"
	"devlog/internal/storage"

	"github.com/urfave/cli/v2"
)

func SearchCommand() *cli.Command {
	return &cli.Command{
		Name:      "search",
		Usage:     "Search events using full-text search",
		ArgsUsage: "<query>",
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
			&cli.BoolFlag{
				Name:    "snippets",
				Aliases: []string{"s"},
				Usage:   "Show matching snippets",
			},
		},
		Action: func(c *cli.Context) error {
			if c.NArg() == 0 {
				return fmt.Errorf("search query required")
			}
			query := strings.Join(c.Args().Slice(), " ")
			return Search(query, c.Bool("verbose"), c.Int("number"), c.Bool("snippets"))
		},
	}
}

func Search(query string, verbose bool, limit int, showSnippets bool) error {
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

	results, err := eventService.SearchEvents(ctx, storage.SearchOptions{
		Query:          query,
		Limit:          limit,
		IncludeSnippet: showSnippets,
	})
	if err != nil {
		return err
	}

	presenter := presentation.NewSearchPresenter(os.Stdout, presentation.FormatTable, verbose)
	return presenter.Present(results, query)
}
