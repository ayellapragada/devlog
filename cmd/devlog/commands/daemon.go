package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"devlog/internal/config"
	"devlog/internal/daemon"
	"devlog/internal/errors"
	"devlog/internal/storage"

	"github.com/urfave/cli/v2"
)

func DaemonCommand() *cli.Command {
	return &cli.Command{
		Name:  "daemon",
		Usage: "Manage the devlog daemon process",
		Subcommands: []*cli.Command{
			{
				Name:  "start",
				Usage: "Start the devlog daemon",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "foreground",
						Aliases: []string{"f"},
						Usage:   "Run daemon in foreground",
					},
				},
				Action: func(c *cli.Context) error {
					return daemonStart(c.Bool("foreground"))
				},
			},
			{
				Name:  "stop",
				Usage: "Stop the devlog daemon",
				Action: func(c *cli.Context) error {
					return daemonStop()
				},
			},
			{
				Name:  "restart",
				Usage: "Restart the devlog daemon",
				Action: func(c *cli.Context) error {
					return daemonRestart()
				},
			},
			{
				Name:  "status",
				Usage: "Check the status of the devlog daemon",
				Action: func(c *cli.Context) error {
					return daemonStatus()
				},
			},
		},
	}
}

func daemonStart(foreground bool) error {
	if daemon.IsRunning() {
		return fmt.Errorf("daemon is already running (PID %d)", daemon.GetPID())
	}

	if os.Getenv("DEVLOG_DAEMON_SUBPROCESS") == "1" || foreground {
		return runDaemonForeground()
	}

	cmd := daemon.SpawnBackground()
	if err := cmd.Start(); err != nil {
		return errors.WrapDaemon("start process", err)
	}

	time.Sleep(500 * time.Millisecond)

	if !daemon.IsRunning() {
		return errors.WrapDaemon("verify startup", fmt.Errorf("daemon failed to start"))
	}

	fmt.Printf("Daemon started successfully (PID %d)\n", daemon.GetPID())
	return nil
}

func runDaemonForeground() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	dataDir, err := config.DataDir()
	if err != nil {
		return err
	}

	dbPath := filepath.Join(dataDir, "events.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return fmt.Errorf("database does not exist (run 'devlog init' first)")
	}

	store, err := storage.New(dbPath)
	if err != nil {
		return err
	}
	defer store.Close()

	d := daemon.New(cfg, store)
	return d.Start()
}

func daemonStop() error {
	if !daemon.IsRunning() {
		fmt.Println("Daemon is not running")
		return nil
	}

	fmt.Printf("Stopping daemon (PID %d)...\n", daemon.GetPID())
	return daemon.StopDaemon()
}

func daemonRestart() error {
	if daemon.IsRunning() {
		fmt.Println("Stopping daemon...")
		if err := daemonStop(); err != nil {
			return errors.WrapDaemon("stop for restart", err)
		}

		if daemon.IsRunning() {
			return errors.WrapDaemon("verify stop", fmt.Errorf("daemon is still running after stop"))
		}
	}

	fmt.Println("Starting daemon...")
	return daemonStart(false)
}

func daemonStatus() error {
	if daemon.IsRunning() {
		fmt.Printf("Daemon is running (PID %d)\n", daemon.GetPID())

		cfg, err := config.Load()
		if err == nil {
			url := fmt.Sprintf("http://127.0.0.1:%d/api/v1/status", cfg.HTTP.Port)
			resp, err := http.Get(url)
			if err == nil {
				defer resp.Body.Close()
				body, _ := io.ReadAll(resp.Body)

				var status map[string]interface{}
				if json.Unmarshal(body, &status) == nil {
					fmt.Printf("Event count: %v\n", status["event_count"])
					fmt.Printf("Uptime: %v seconds\n", status["uptime_seconds"])
				}
			}
		}
	} else {
		fmt.Println("Daemon is not running")
	}
	return nil
}
