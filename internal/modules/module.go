package modules

import (
	"fmt"
	"sync"

	"devlog/internal/install"
	"devlog/internal/poller"
)

type Module interface {
	Name() string
	Description() string
	Install(ctx *install.Context) error
	Uninstall(ctx *install.Context) error
	DefaultConfig() interface{}
	ValidateConfig(config interface{}) error
}

type Pollable interface {
	CreatePoller(config map[string]interface{}, dataDir string) (poller.Poller, error)
}

type ModuleWithPoller interface {
	Module
	Pollable
}

var (
	mu      sync.RWMutex
	modules = make(map[string]Module)
)

func Register(module Module) error {
	mu.Lock()
	defer mu.Unlock()

	name := module.Name()
	if _, exists := modules[name]; exists {
		return fmt.Errorf("module %s is already registered", name)
	}

	modules[name] = module
	return nil
}

func Get(name string) (Module, error) {
	mu.RLock()
	defer mu.RUnlock()

	mod, exists := modules[name]
	if !exists {
		return nil, fmt.Errorf("module %s not found", name)
	}
	return mod, nil
}

func List() []Module {
	mu.RLock()
	defer mu.RUnlock()

	result := make([]Module, 0, len(modules))
	for _, mod := range modules {
		result = append(result, mod)
	}
	return result
}
