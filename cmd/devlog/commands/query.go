package commands

import (
	"context"
	"fmt"
	"os"
	"strings"

	"devlog/internal/output"
	queryPlugin "devlog/plugins/query"

	"github.com/urfave/cli/v2"
)

func QueryCommand() *cli.Command {
	return &cli.Command{
		Name:        "query",
		Usage:       "Ask questions about your development history in natural language",
		UsageText:   "devlog query [options] [question]",
		Description: "Uses an LLM to understand your question and query your development history intelligently.\n\n   Examples:\n      devlog query \"What was I working on?\"\n      devlog query \"What files did I change today?\"\n      devlog query \"Show me all git commits from last week\"\n      devlog query \"What errors did I encounter yesterday?\"\n      devlog query \"When did I last work on the auth module?\"",
		ArgsUsage:   "[question]",
		Action: func(c *cli.Context) error {
			question := "What was I working on?"
			if c.NArg() > 0 {
				question = strings.Join(c.Args().Slice(), " ")
			}

			plugin, _, err := queryPlugin.LoadPlugin()
			if err != nil {
				return err
			}

			ctx := context.Background()
			result, err := plugin.Query(ctx, question)
			if err != nil {
				return err
			}

			if len(result.Results) == 0 {
				fmt.Println("No events found matching your query.")
				return nil
			}

			fmt.Printf("Generating summary of %d events...\n", len(result.Results))
			llmClient := plugin.LLMClient()
			formatter := output.NewLLMFormatter(llmClient, result.Plan.ResponseGoal)
			presenter := output.NewSearchPresenterWithFormatter(os.Stdout, formatter)
			return presenter.Present(ctx, result.Results, question)
		},
	}
}
