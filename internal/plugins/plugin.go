package plugins

import (
	"context"
	"fmt"
)

type Plugin interface {
	Name() string
	Description() string
	Start(ctx context.Context) error
	DefaultConfig() interface{}
	ValidateConfig(config interface{}) error
}

type Registry struct {
	plugins map[string]Plugin
}

func NewRegistry() *Registry {
	return &Registry{
		plugins: make(map[string]Plugin),
	}
}

func (r *Registry) Register(plugin Plugin) error {
	name := plugin.Name()
	if _, exists := r.plugins[name]; exists {
		return fmt.Errorf("plugin %s is already registered", name)
	}
	r.plugins[name] = plugin
	return nil
}

func (r *Registry) Get(name string) (Plugin, error) {
	plugin, exists := r.plugins[name]
	if !exists {
		return nil, fmt.Errorf("plugin %s not found", name)
	}
	return plugin, nil
}

func (r *Registry) List() []Plugin {
	plugins := make([]Plugin, 0, len(r.plugins))
	for _, plugin := range r.plugins {
		plugins = append(plugins, plugin)
	}
	return plugins
}

var globalRegistry = NewRegistry()

func Register(plugin Plugin) error {
	return globalRegistry.Register(plugin)
}

func Get(name string) (Plugin, error) {
	return globalRegistry.Get(name)
}

func List() []Plugin {
	return globalRegistry.List()
}
