package commands

import (
	"fmt"
	"os/exec"
	"runtime"

	"devlog/internal/config"

	"github.com/urfave/cli/v2"
)

func WebCommand() *cli.Command {
	return &cli.Command{
		Name:  "web",
		Usage: "Manage devlog web interface",
		Subcommands: []*cli.Command{
			{
				Name:  "open",
				Usage: "Open the web interface in your browser",
				Action: func(c *cli.Context) error {
					cfg, err := config.Load()
					if err != nil {
						return err
					}

					url := fmt.Sprintf("http://127.0.0.1:%d", cfg.HTTP.Port)

					var cmd *exec.Cmd
					switch runtime.GOOS {
					case "darwin":
						cmd = exec.Command("open", url)
					case "linux":
						cmd = exec.Command("xdg-open", url)
					case "windows":
						cmd = exec.Command("cmd", "/c", "start", url)
					default:
						return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
					}

					if err := cmd.Start(); err != nil {
						return fmt.Errorf("failed to open browser: %w", err)
					}

					fmt.Printf("Opening %s in your browser...\n", url)
					return nil
				},
			},
			{
				Name:  "url",
				Usage: "Display the web interface URL",
				Action: func(c *cli.Context) error {
					cfg, err := config.Load()
					if err != nil {
						return err
					}

					url := fmt.Sprintf("http://127.0.0.1:%d", cfg.HTTP.Port)
					fmt.Println(url)
					return nil
				},
			},
		},
	}
}
