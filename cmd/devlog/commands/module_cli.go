package commands

import (
	"devlog/internal/config"
	"devlog/internal/modules"

	"github.com/urfave/cli/v2"
)

func ModuleCommand() *cli.Command {
	return createComponentCommandCli(
		"module",
		"modules",
		moduleRegistry{},
		func() ComponentConfig {
			cfg, _ := config.Load()
			return moduleConfigOps{cfg: cfg}
		},
	)
}

type moduleRegistry struct{}

func (r moduleRegistry) Get(name string) (Component, error) {
	return modules.Get(name)
}

func (r moduleRegistry) List() []Component {
	mods := modules.List()
	components := make([]Component, len(mods))
	for i, m := range mods {
		components[i] = m
	}
	return components
}

type moduleConfigOps struct {
	cfg *config.Config
}

func (m moduleConfigOps) IsEnabled(name string) bool {
	return m.cfg.IsModuleEnabled(name)
}

func (m moduleConfigOps) GetConfig(name string) (map[string]interface{}, bool) {
	return m.cfg.GetModuleConfig(name)
}

func (m moduleConfigOps) SetEnabled(name string, enabled bool) {
	m.cfg.SetModuleEnabled(name, enabled)
}

func (m moduleConfigOps) SetConfig(name string, config map[string]interface{}) {
	m.cfg.SetModuleConfig(name, config)
}

func (m moduleConfigOps) ClearConfig(name string) {
	m.cfg.ClearModuleConfig(name)
}

func (m moduleConfigOps) Save() error {
	return m.cfg.Save()
}
