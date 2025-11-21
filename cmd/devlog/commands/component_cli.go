package commands

import (
	"fmt"

	"github.com/urfave/cli/v2"
)

func createComponentCommandCli(
	componentType string,
	pluralName string,
	registry ComponentRegistry,
	configOpsFunc func() ComponentConfig,
) *cli.Command {
	return &cli.Command{
		Name:  componentType,
		Usage: fmt.Sprintf("Manage devlog %s", pluralName),
		Subcommands: []*cli.Command{
			{
				Name:  "list",
				Usage: fmt.Sprintf("List all available %s and their status", pluralName),
				Action: func(c *cli.Context) error {
					return componentList(pluralName, registry, configOpsFunc())
				},
			},
			{
				Name:      "install",
				Usage:     fmt.Sprintf("Install and enable one or more %s", pluralName),
				ArgsUsage: "<name> [name...]",
				Action: func(c *cli.Context) error {
					if c.NArg() < 1 {
						return fmt.Errorf("%s name required", componentType)
					}
					args := c.Args().Slice()
					return componentInstall(componentType, args, registry, configOpsFunc())
				},
			},
			{
				Name:      "uninstall",
				Usage:     fmt.Sprintf("Uninstall and disable one or more %s (preserves config)", pluralName),
				ArgsUsage: "<name> [name...]",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:  "purge",
						Usage: "Remove configuration completely",
					},
				},
				Action: func(c *cli.Context) error {
					if c.NArg() < 1 {
						return fmt.Errorf("%s name required", componentType)
					}
					args := c.Args().Slice()
					if c.Bool("purge") {
						args = append([]string{"--purge"}, args...)
					}
					return componentUninstall(componentType, args, registry, configOpsFunc())
				},
			},
		},
	}
}
