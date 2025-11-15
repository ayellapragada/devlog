package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	ObsidianPath      string      `yaml:"obsidian_path"`
	HTTP              HTTPConfig  `yaml:"http"`
	SessionGapMinutes int         `yaml:"session_gap_minutes,omitempty"`
	Shell             ShellConfig `yaml:"shell,omitempty"`
}

// HTTPConfig contains HTTP server settings
type HTTPConfig struct {
	Port int `yaml:"port"`
}

// ShellConfig contains shell hook settings
type ShellConfig struct {
	Enabled     bool     `yaml:"enabled"`
	CaptureMode string   `yaml:"capture_mode,omitempty"` // "all" or "important" (default)
	IgnoreList  []string `yaml:"ignore_list,omitempty"`  // Commands to ignore (e.g., "ls", "cd")
}

// DefaultConfig returns a config with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		HTTP: HTTPConfig{
			Port: 8573,
		},
		Shell: ShellConfig{
			Enabled:     true,
			CaptureMode: "important",
			IgnoreList: []string{
				"ls", "cd", "pwd", "echo", "cat", "clear",
				"exit", "history", "which", "type", "alias",
			},
		},
	}
}

// ConfigDir returns the config directory path (~/.config/devlog)
func ConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}
	return filepath.Join(home, ".config", "devlog"), nil
}

// ConfigPath returns the full path to the config file
func ConfigPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

// DataDir returns the data directory path (~/.local/share/devlog)
func DataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}
	return filepath.Join(home, ".local", "share", "devlog"), nil
}

// QueueDir returns the queue directory path (~/.local/share/devlog/queue)
func QueueDir() (string, error) {
	dataDir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dataDir, "queue"), nil
}

// Load reads and parses the config file
func Load() (*Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config file not found at %s (run with --init to create)", path)
		}
		return nil, fmt.Errorf("read config file: %w", err)
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return cfg, nil
}

// Validate checks if the config is valid
func (c *Config) Validate() error {
	if c.ObsidianPath == "" {
		return fmt.Errorf("obsidian_path is required")
	}

	// Expand tilde in path
	if c.ObsidianPath[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("expand home directory: %w", err)
		}
		c.ObsidianPath = filepath.Join(home, c.ObsidianPath[1:])
	}

	if c.HTTP.Port < 1 || c.HTTP.Port > 65535 {
		return fmt.Errorf("http port must be between 1 and 65535")
	}

	// Validate shell config
	if c.Shell.CaptureMode != "" && c.Shell.CaptureMode != "all" && c.Shell.CaptureMode != "important" {
		return fmt.Errorf("shell.capture_mode must be 'all' or 'important'")
	}

	return nil
}

// ShouldCaptureCommand checks if a shell command should be captured
func (c *Config) ShouldCaptureCommand(command string) bool {
	// If shell hooks are disabled, don't capture
	if !c.Shell.Enabled {
		return false
	}

	// Extract the base command (first word)
	baseCmd := command
	for i, ch := range command {
		if ch == ' ' || ch == '\t' {
			baseCmd = command[:i]
			break
		}
	}

	// If capture mode is "all", capture everything except ignored commands
	if c.Shell.CaptureMode == "all" {
		// Check ignore list
		for _, ignored := range c.Shell.IgnoreList {
			if baseCmd == ignored {
				return false
			}
		}
		return true
	}

	// Default "important" mode - filter out common navigation/viewing commands
	// Always check ignore list first
	for _, ignored := range c.Shell.IgnoreList {
		if baseCmd == ignored {
			return false
		}
	}

	// In "important" mode, only capture commands that are likely meaningful
	// (build tools, git commands, package managers, etc.)
	return true // For now, let ignore list handle filtering
}

// InitConfig creates a default config file and necessary directories
func InitConfig() error {
	configDir, err := ConfigDir()
	if err != nil {
		return err
	}

	dataDir, err := DataDir()
	if err != nil {
		return err
	}

	// Create directories
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}

	// Create default config
	configPath, err := ConfigPath()
	if err != nil {
		return err
	}

	if _, err := os.Stat(configPath); err == nil {
		return fmt.Errorf("config file already exists at %s", configPath)
	}

	defaultCfg := DefaultConfig()
	defaultCfg.ObsidianPath = "~/Documents/Obsidian/DevLogs"

	data, err := yaml.Marshal(defaultCfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}

	fmt.Printf("Created config file at %s\n", configPath)
	fmt.Printf("Created data directory at %s\n", dataDir)
	fmt.Println("\nPlease edit the config file to set your Obsidian path.")

	return nil
}
