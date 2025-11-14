package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	ObsidianPath string     `yaml:"obsidian_path"`
	HTTP         HTTPConfig `yaml:"http"`
}

// HTTPConfig contains HTTP server settings
type HTTPConfig struct {
	Port int `yaml:"port"`
}

// DefaultConfig returns a config with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		HTTP: HTTPConfig{
			Port: 8573,
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

	return nil
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
