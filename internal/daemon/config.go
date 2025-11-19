package daemon

import (
	"context"
	"encoding/json"
	"log/slog"

	"devlog/internal/config"
	"devlog/internal/modules"
	"devlog/internal/plugins"
)

func (d *Daemon) startConfigWatcher(ctx context.Context) error {
	configPath, err := config.ConfigPath()
	if err != nil {
		return err
	}

	watcher, err := config.NewWatcher(configPath, d.handleConfigChange, d.logger.Logger)
	if err != nil {
		return err
	}

	d.configWatcher = watcher

	go func() {
		if err := watcher.Start(ctx); err != nil {
			d.logger.Error("config watcher error",
				slog.String("error", err.Error()))
		}
	}()

	d.logger.Info("config watcher started", slog.String("path", configPath))
	return nil
}

func (d *Daemon) handleConfigChange(newConfig *config.Config) {
	d.configMu.Lock()
	oldConfig := d.config
	d.config = newConfig
	d.configMu.Unlock()

	d.logger.Info("configuration reloaded")

	if oldConfig.HTTP.Port != newConfig.HTTP.Port {
		d.logger.Warn("http port changed, restart required",
			slog.Int("old_port", oldConfig.HTTP.Port),
			slog.Int("new_port", newConfig.HTTP.Port))
	}

	d.handleExtensionConfigChanges("module", oldConfig.Modules, newConfig.Modules)
	d.handleExtensionConfigChanges("plugin", oldConfig.Plugins, newConfig.Plugins)
}

func (d *Daemon) handleExtensionConfigChanges(extensionType string, oldComponents, newComponents map[string]config.ComponentConfig) {
	for name := range newComponents {
		oldCfg, oldExists := oldComponents[name]
		newCfg := newComponents[name]

		if !oldExists {
			if newCfg.Enabled {
				d.logger.Info(extensionType+" enabled, starting",
					slog.String(extensionType, name))
				d.startExtension(extensionType, name)
			}
			continue
		}

		if oldCfg.Enabled && !newCfg.Enabled {
			d.logger.Info(extensionType+" disabled, stopping",
				slog.String(extensionType, name))
			d.stopExtension(extensionType, name)
			continue
		}

		if !oldCfg.Enabled && newCfg.Enabled {
			d.logger.Info(extensionType+" enabled, starting",
				slog.String(extensionType, name))
			d.startExtension(extensionType, name)
			continue
		}

		if oldCfg.Enabled && newCfg.Enabled && !configMapsEqual(oldCfg.Config, newCfg.Config) {
			d.logger.Info(extensionType+" config changed, restarting",
				slog.String(extensionType, name))
			d.restartExtension(extensionType, name)
		}
	}

	for name := range oldComponents {
		if _, exists := newComponents[name]; !exists {
			oldCfg := oldComponents[name]
			if oldCfg.Enabled {
				d.logger.Info(extensionType+" removed, stopping",
					slog.String(extensionType, name))
				d.stopExtension(extensionType, name)
			}
		}
	}
}

func (d *Daemon) startExtension(extensionType, name string) {
	switch extensionType {
	case "plugin":
		plugin, err := plugins.Get(name)
		if err != nil {
			d.logger.Error("failed to get plugin",
				slog.String("plugin", name),
				slog.String("error", err.Error()))
			return
		}
		d.startPlugin(d.pluginCtx, plugin, name)
	case "module":
		module, err := modules.Get(name)
		if err != nil {
			d.logger.Error("failed to get module",
				slog.String("module", name),
				slog.String("error", err.Error()))
			return
		}
		ctx := d.moduleCtx
		if ctx == nil {
			ctx = context.Background()
		}
		d.startModuleWithContext(module, name, ctx)
	}
}

func (d *Daemon) stopExtension(extensionType, name string) {
	var err error
	switch extensionType {
	case "plugin":
		err = d.stopPlugin(name)
	case "module":
		err = d.stopModule(name)
	}
	if err != nil {
		d.logger.Warn("failed to stop extension",
			slog.String("type", extensionType),
			slog.String("name", name),
			slog.String("error", err.Error()))
	}
}

func (d *Daemon) restartExtension(extensionType, name string) {
	switch extensionType {
	case "plugin":
		d.restartPlugin(name)
	case "module":
		module, err := modules.Get(name)
		if err != nil {
			d.logger.Error("failed to get module for restart",
				slog.String("module", name),
				slog.String("error", err.Error()))
			return
		}

		if _, ok := module.(modules.Pollable); ok {
			d.restartModule(name)
		} else {
			d.logger.Info("module config changed (no runtime component, changes will apply on daemon restart)",
				slog.String("module", name))
		}
	}
}

func configMapsEqual(a, b map[string]interface{}) bool {
	if len(a) != len(b) {
		return false
	}

	aJSON, err := json.Marshal(a)
	if err != nil {
		return false
	}

	bJSON, err := json.Marshal(b)
	if err != nil {
		return false
	}

	return string(aJSON) == string(bJSON)
}

func cloneConfigMap(src map[string]interface{}) map[string]interface{} {
	if src == nil {
		return make(map[string]interface{})
	}
	clone := make(map[string]interface{}, len(src))
	for k, v := range src {
		clone[k] = v
	}
	return clone
}
