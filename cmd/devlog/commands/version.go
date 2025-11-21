package commands

import (
	"fmt"
	"runtime"

	"github.com/urfave/cli/v2"
)

var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildTime = "unknown"
	GitDirty  = "unknown"
)

func VersionCommand() *cli.Command {
	return &cli.Command{
		Name:  "version",
		Usage: "Show version information",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "short",
				Aliases: []string{"s"},
				Usage:   "Show only the git commit",
			},
		},
		Action: func(c *cli.Context) error {
			if c.Bool("short") {
				fmt.Println(GitCommit)
				return nil
			}

			fmt.Printf("devlog version:    %s\n", Version)
			fmt.Printf("git commit:        %s", GitCommit)
			if GitDirty == "true" {
				fmt.Printf(" (dirty)")
			}
			fmt.Println()
			fmt.Printf("build time:        %s\n", BuildTime)
			fmt.Printf("go version:        %s\n", runtime.Version())
			fmt.Printf("platform:          %s/%s\n", runtime.GOOS, runtime.GOARCH)

			return nil
		},
	}
}
