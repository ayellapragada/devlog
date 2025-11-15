package modules

import (
	"fmt"
)

type Module interface {
	Name() string
	Description() string
	Install(ctx *InstallContext) error
	Uninstall(ctx *InstallContext) error
	DefaultConfig() interface{}
	ValidateConfig(config interface{}) error
}

type InstallContext struct {
	Interactive bool
	ConfigDir   string
	DataDir     string
	HomeDir     string
	Log         func(format string, args ...interface{})
}

type Registry struct {
	modules map[string]Module
}

func NewRegistry() *Registry {
	return &Registry{
		modules: make(map[string]Module),
	}
}

func (r *Registry) Register(module Module) error {
	name := module.Name()
	if _, exists := r.modules[name]; exists {
		return fmt.Errorf("module %s is already registered", name)
	}
	r.modules[name] = module
	return nil
}

func (r *Registry) Get(name string) (Module, error) {
	module, exists := r.modules[name]
	if !exists {
		return nil, fmt.Errorf("module %s not found", name)
	}
	return module, nil
}

func (r *Registry) List() []Module {
	modules := make([]Module, 0, len(r.modules))
	for _, module := range r.modules {
		modules = append(modules, module)
	}
	return modules
}

var globalRegistry = NewRegistry()

func Register(module Module) error {
	return globalRegistry.Register(module)
}

func Get(name string) (Module, error) {
	return globalRegistry.Get(name)
}

func List() []Module {
	return globalRegistry.List()
}
