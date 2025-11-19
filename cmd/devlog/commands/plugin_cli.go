package commands

import (
	"devlog/internal/config"
	"devlog/internal/plugins"

	"github.com/urfave/cli/v2"
)

func PluginCommand() *cli.Command {
	return createComponentCommandCli(
		"plugin",
		"plugins",
		pluginRegistry{},
		func() ComponentConfig {
			cfg, _ := config.Load()
			return pluginConfigOps{cfg: cfg}
		},
	)
}

type pluginRegistry struct{}

func (r pluginRegistry) Get(name string) (Component, error) {
	return plugins.Get(name)
}

func (r pluginRegistry) List() []Component {
	plugs := plugins.List()
	components := make([]Component, len(plugs))
	for i, p := range plugs {
		components[i] = p
	}
	return components
}

type pluginConfigOps struct {
	cfg *config.Config
}

func (p pluginConfigOps) IsEnabled(name string) bool {
	return p.cfg.IsPluginEnabled(name)
}

func (p pluginConfigOps) GetConfig(name string) (map[string]interface{}, bool) {
	return p.cfg.GetPluginConfig(name)
}

func (p pluginConfigOps) SetEnabled(name string, enabled bool) {
	p.cfg.SetPluginEnabled(name, enabled)
}

func (p pluginConfigOps) SetConfig(name string, config map[string]interface{}) {
	p.cfg.SetPluginConfig(name, config)
}

func (p pluginConfigOps) ClearConfig(name string) {
	p.cfg.ClearPluginConfig(name)
}

func (p pluginConfigOps) Save() error {
	return p.cfg.Save()
}
