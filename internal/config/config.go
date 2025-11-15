package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	ObsidianPath      string                  `yaml:"obsidian_path"`
	HTTP              HTTPConfig              `yaml:"http"`
	SessionGapMinutes int                     `yaml:"session_gap_minutes,omitempty"`
	Modules           map[string]ModuleConfig `yaml:"modules,omitempty"`

	Shell ShellConfig `yaml:"shell,omitempty"`
}

type ModuleConfig struct {
	Enabled bool                   `yaml:"enabled"`
	Config  map[string]interface{} `yaml:",inline"`
}

type HTTPConfig struct {
	Port int `yaml:"port"`
}

type ShellConfig struct {
	Enabled     bool     `yaml:"enabled"`
	CaptureMode string   `yaml:"capture_mode,omitempty"`
	IgnoreList  []string `yaml:"ignore_list,omitempty"`
}

func DefaultConfig() *Config {
	return &Config{
		HTTP: HTTPConfig{
			Port: 8573,
		},
		Modules: make(map[string]ModuleConfig),
	}
}

func ConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}
	return filepath.Join(home, ".config", "devlog"), nil
}

func ConfigPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

func DataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}
	return filepath.Join(home, ".local", "share", "devlog"), nil
}

func QueueDir() (string, error) {
	dataDir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dataDir, "queue"), nil
}

func (c *Config) Save() error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}

	return nil
}

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

func (c *Config) Validate() error {
	if c.ObsidianPath == "" {
		return fmt.Errorf("obsidian_path is required")
	}

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

func (c *Config) IsModuleEnabled(moduleName string) bool {
	if c.Modules == nil {
		return false
	}
	modCfg, exists := c.Modules[moduleName]
	return exists && modCfg.Enabled
}

func (c *Config) GetModuleConfig(moduleName string) (map[string]interface{}, bool) {
	if c.Modules == nil {
		return nil, false
	}
	modCfg, exists := c.Modules[moduleName]
	if !exists || !modCfg.Enabled {
		return nil, false
	}
	return modCfg.Config, true
}

func (c *Config) SetModuleEnabled(moduleName string, enabled bool) {
	if c.Modules == nil {
		c.Modules = make(map[string]ModuleConfig)
	}
	modCfg := c.Modules[moduleName]
	modCfg.Enabled = enabled
	c.Modules[moduleName] = modCfg
}

func (c *Config) SetModuleConfig(moduleName string, config map[string]interface{}) {
	if c.Modules == nil {
		c.Modules = make(map[string]ModuleConfig)
	}
	modCfg := c.Modules[moduleName]
	modCfg.Config = config
	c.Modules[moduleName] = modCfg
}

func (c *Config) ShouldCaptureCommand(command string) bool {
	if c.IsModuleEnabled("shell") {
		shellCfg, ok := c.GetModuleConfig("shell")
		if ok {
			captureMode, _ := shellCfg["capture_mode"].(string)
			ignoreList, _ := shellCfg["ignore_list"].([]interface{})

			return shouldCaptureWithConfig(command, captureMode, ignoreList)
		}
	}

	if !c.Shell.Enabled {
		return false
	}

	baseCmd := command
	for i, ch := range command {
		if ch == ' ' || ch == '\t' {
			baseCmd = command[:i]
			break
		}
	}

	if c.Shell.CaptureMode == "all" {
		for _, ignored := range c.Shell.IgnoreList {
			if baseCmd == ignored {
				return false
			}
		}
		return true
	}

	for _, ignored := range c.Shell.IgnoreList {
		if baseCmd == ignored {
			return false
		}
	}

	return true
}

func shouldCaptureWithConfig(command string, captureMode string, ignoreList []interface{}) bool {
	baseCmd := command
	for i, ch := range command {
		if ch == ' ' || ch == '\t' {
			baseCmd = command[:i]
			break
		}
	}

	ignored := make([]string, 0, len(ignoreList))
	for _, item := range ignoreList {
		if s, ok := item.(string); ok {
			ignored = append(ignored, s)
		}
	}

	for _, ig := range ignored {
		if baseCmd == ig {
			return false
		}
	}

	if captureMode == "all" {
		return true
	}

	return true
}

func InitConfig() error {
	configDir, err := ConfigDir()
	if err != nil {
		return err
	}

	dataDir, err := DataDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}

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
