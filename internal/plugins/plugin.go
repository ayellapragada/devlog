package plugins

import (
	"context"
	"fmt"
	"sync"

	"devlog/internal/install"
)

type Plugin interface {
	Name() string
	Description() string
	Install(ctx *install.Context) error
	Uninstall(ctx *install.Context) error
	Start(ctx context.Context) error
	DefaultConfig() interface{}
	ValidateConfig(config interface{}) error
}

var (
	mu      sync.RWMutex
	plugins = make(map[string]Plugin)
)

func Register(plugin Plugin) error {
	mu.Lock()
	defer mu.Unlock()

	name := plugin.Name()
	if _, exists := plugins[name]; exists {
		return fmt.Errorf("plugin %s is already registered", name)
	}

	plugins[name] = plugin
	return nil
}

func Get(name string) (Plugin, error) {
	mu.RLock()
	defer mu.RUnlock()

	p, exists := plugins[name]
	if !exists {
		return nil, fmt.Errorf("plugin %s not found", name)
	}
	return p, nil
}

func List() []Plugin {
	mu.RLock()
	defer mu.RUnlock()

	result := make([]Plugin, 0, len(plugins))
	for _, p := range plugins {
		result = append(result, p)
	}
	return result
}
