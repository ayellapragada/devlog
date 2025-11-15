package main

import (
	"fmt"
	"os"
	"path/filepath"

	"devlog/internal/config"
	"devlog/internal/daemon"
	"devlog/internal/storage"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
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

	d := daemon.New(cfg, store)
	return d.Start()
}
