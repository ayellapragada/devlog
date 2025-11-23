package config

import (
	"fmt"
	"os"
	"path/filepath"

	"devlog/internal/modules"
	"devlog/internal/plugins"

	"gopkg.in/yaml.v3"
)

type Config struct {
	HTTP    HTTPConfig                 `yaml:"http"`
	Modules map[string]ComponentConfig `yaml:"modules,omitempty"`
	Plugins map[string]ComponentConfig `yaml:"plugins,omitempty"`
}

type ComponentConfig struct {
	Enabled bool                   `yaml:"enabled"`
	Config  map[string]interface{} `yaml:",inline"`
}

type HTTPConfig struct {
	Port int `yaml:"port"`
}

func DefaultConfig() *Config {
	return &Config{
		HTTP: HTTPConfig{
			Port: 8573,
		},
		Modules: make(map[string]ComponentConfig),
		Plugins: make(map[string]ComponentConfig),
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
		return "", fmt.Errorf("get config directory: %w", err)
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
		return "", fmt.Errorf("get data directory: %w", err)
	}
	return filepath.Join(dataDir, "queue"), nil
}

func (c *Config) Save() error {
	path, err := ConfigPath()
	if err != nil {
		return fmt.Errorf("get config path: %w", err)
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
		return nil, fmt.Errorf("get config path: %w", err)
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
	if c.HTTP.Port < 1024 || c.HTTP.Port > 65535 {
		return fmt.Errorf("http port must be between 1024 and 65535 (privileged ports not allowed)")
	}

	if err := c.validateModules(); err != nil {
		return fmt.Errorf("module validation failed: %w", err)
	}

	if err := c.validatePlugins(); err != nil {
		return fmt.Errorf("plugin validation failed: %w", err)
	}

	return nil
}

func (c *Config) validateModules() error {
	for name, modCfg := range c.Modules {
		if !modCfg.Enabled {
			continue
		}

		mod, err := modules.Get(name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: unknown module '%s' in config (module may not be installed)\n", name)
			continue
		}

		if err := mod.ValidateConfig(modCfg.Config); err != nil {
			return fmt.Errorf("module '%s' config validation failed: %w", name, err)
		}
	}
	return nil
}

func (c *Config) validatePlugins() error {
	for name, pluginCfg := range c.Plugins {
		if !pluginCfg.Enabled {
			continue
		}

		plugin, err := plugins.Get(name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: unknown plugin '%s' in config (plugin may not be installed)\n", name)
			continue
		}

		if err := plugin.ValidateConfig(pluginCfg.Config); err != nil {
			return fmt.Errorf("plugin '%s' config validation failed: %w", name, err)
		}
	}
	return nil
}

func isComponentEnabled(components map[string]ComponentConfig, name string) bool {
	if components == nil {
		return false
	}
	cfg, exists := components[name]
	return exists && cfg.Enabled
}

func getComponentConfig(components map[string]ComponentConfig, name string) (map[string]interface{}, bool) {
	if components == nil {
		return nil, false
	}
	cfg, exists := components[name]
	if !exists || !cfg.Enabled {
		return nil, false
	}
	return cfg.Config, true
}

func setComponentEnabled(components *map[string]ComponentConfig, name string, enabled bool) {
	if *components == nil {
		*components = make(map[string]ComponentConfig)
	}
	cfg := (*components)[name]
	cfg.Enabled = enabled
	(*components)[name] = cfg
}

func setComponentConfig(components *map[string]ComponentConfig, name string, config map[string]interface{}) {
	if *components == nil {
		*components = make(map[string]ComponentConfig)
	}
	cfg := (*components)[name]
	cfg.Config = config
	(*components)[name] = cfg
}

func clearComponent(components map[string]ComponentConfig, name string) {
	if components == nil {
		return
	}
	delete(components, name)
}

func (c *Config) IsModuleEnabled(moduleName string) bool {
	return isComponentEnabled(c.Modules, moduleName)
}

func (c *Config) GetModuleConfig(moduleName string) (map[string]interface{}, bool) {
	return getComponentConfig(c.Modules, moduleName)
}

func (c *Config) SetModuleEnabled(moduleName string, enabled bool) {
	setComponentEnabled(&c.Modules, moduleName, enabled)
}

func (c *Config) SetModuleConfig(moduleName string, config map[string]interface{}) {
	setComponentConfig(&c.Modules, moduleName, config)
}

func (c *Config) ClearModuleConfig(moduleName string) {
	clearComponent(c.Modules, moduleName)
}

func (c *Config) IsPluginEnabled(pluginName string) bool {
	return isComponentEnabled(c.Plugins, pluginName)
}

func (c *Config) GetPluginConfig(pluginName string) (map[string]interface{}, bool) {
	return getComponentConfig(c.Plugins, pluginName)
}

func (c *Config) SetPluginEnabled(pluginName string, enabled bool) {
	setComponentEnabled(&c.Plugins, pluginName, enabled)
}

func (c *Config) SetPluginConfig(pluginName string, config map[string]interface{}) {
	setComponentConfig(&c.Plugins, pluginName, config)
}

func (c *Config) ClearPluginConfig(pluginName string) {
	clearComponent(c.Plugins, pluginName)
}

func (c *Config) ShouldCaptureCommand(command string) bool {
	if !c.IsModuleEnabled("shell") {
		return false
	}

	shellCfg, ok := c.GetModuleConfig("shell")
	if !ok {
		return true
	}

	baseCmd := command
	for i, ch := range command {
		if ch == ' ' || ch == '\t' {
			baseCmd = command[:i]
			break
		}
	}

	ignoreListInterface, _ := shellCfg["ignore_list"].([]interface{})
	for _, item := range ignoreListInterface {
		if ignored, ok := item.(string); ok && baseCmd == ignored {
			return false
		}
	}

	return true
}

func (c *Config) AddToShellIgnoreList(commands ...string) {
	shellCfg, ok := c.GetModuleConfig("shell")
	if !ok {
		shellCfg = make(map[string]interface{})
	}

	ignoreList := []string{}
	if existingList, ok := shellCfg["ignore_list"].([]interface{}); ok {
		for _, item := range existingList {
			if cmd, ok := item.(string); ok {
				ignoreList = append(ignoreList, cmd)
			}
		}
	}

	for _, cmd := range commands {
		found := false
		for _, existing := range ignoreList {
			if existing == cmd {
				found = true
				break
			}
		}
		if !found {
			ignoreList = append(ignoreList, cmd)
		}
	}

	shellCfg["ignore_list"] = ignoreList
	c.SetModuleConfig("shell", shellCfg)
}

func (c *Config) RemoveFromShellIgnoreList(commands ...string) {
	shellCfg, ok := c.GetModuleConfig("shell")
	if !ok {
		return
	}

	ignoreList := []string{}
	if existingList, ok := shellCfg["ignore_list"].([]interface{}); ok {
		for _, item := range existingList {
			if cmd, ok := item.(string); ok {
				shouldRemove := false
				for _, toRemove := range commands {
					if cmd == toRemove {
						shouldRemove = true
						break
					}
				}
				if !shouldRemove {
					ignoreList = append(ignoreList, cmd)
				}
			}
		}
	}

	shellCfg["ignore_list"] = ignoreList
	c.SetModuleConfig("shell", shellCfg)
}

func InitConfig() error {
	configDir, err := ConfigDir()
	if err != nil {
		return fmt.Errorf("get config directory: %w", err)
	}

	dataDir, err := DataDir()
	if err != nil {
		return fmt.Errorf("get data directory: %w", err)
	}

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}

	configPath, err := ConfigPath()
	if err != nil {
		return fmt.Errorf("get config path: %w", err)
	}

	if _, err := os.Stat(configPath); err == nil {
		return fmt.Errorf("config file already exists at %s", configPath)
	}

	defaultCfg := DefaultConfig()

	data, err := yaml.Marshal(defaultCfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}

	fmt.Printf("Created config file at %s\n", configPath)
	fmt.Printf("Created data directory at %s\n", dataDir)

	return nil
}
