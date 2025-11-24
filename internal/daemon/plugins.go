package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"devlog/internal/contextkeys"
	"devlog/internal/metrics"
	"devlog/internal/plugins"
)

type pluginInstance struct {
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	plugin plugins.Plugin
}

func (d *Daemon) startPlugins(ctx context.Context) {
	pluginCtx, cancel := context.WithCancel(ctx)
	d.pluginCtx = pluginCtx
	d.pluginCtxCancel = cancel

	allPlugins := plugins.List()
	enabledPlugins := make([]plugins.Plugin, 0)

	for _, plugin := range allPlugins {
		pluginName := plugin.Name()
		if !d.config.IsPluginEnabled(pluginName) {
			d.logger.Debug("plugin disabled, skipping",
				slog.String("plugin", pluginName))
			continue
		}
		enabledPlugins = append(enabledPlugins, plugin)
	}

	orderedPlugins, err := d.resolvePluginDependencies(enabledPlugins)
	if err != nil {
		d.logger.Error("failed to resolve plugin dependencies",
			slog.String("error", err.Error()))
		return
	}

	for _, plugin := range orderedPlugins {
		d.startPlugin(pluginCtx, plugin, plugin.Name())
	}
}

func (d *Daemon) resolvePluginDependencies(enabledPlugins []plugins.Plugin) ([]plugins.Plugin, error) {
	pluginMap := make(map[string]plugins.Plugin)
	for _, p := range enabledPlugins {
		pluginMap[p.Name()] = p
	}

	visited := make(map[string]bool)
	visiting := make(map[string]bool)
	var ordered []plugins.Plugin

	var visit func(string) error
	visit = func(name string) error {
		if visited[name] {
			return nil
		}
		if visiting[name] {
			return fmt.Errorf("circular dependency detected involving plugin %s", name)
		}

		plugin, exists := pluginMap[name]
		if !exists {
			return fmt.Errorf("plugin %s not found (required by another plugin)", name)
		}

		visiting[name] = true
		metadata := plugin.Metadata()

		for _, dep := range metadata.Dependencies {
			depPlugin, exists := pluginMap[dep]
			if !exists {
				return fmt.Errorf("plugin %s depends on %s, but %s is not enabled", name, dep, dep)
			}
			if err := visit(depPlugin.Name()); err != nil {
				return err
			}
		}

		visiting[name] = false
		visited[name] = true
		ordered = append(ordered, plugin)
		return nil
	}

	for _, plugin := range enabledPlugins {
		if err := visit(plugin.Name()); err != nil {
			return nil, err
		}
	}

	return ordered, nil
}

func (d *Daemon) startPlugin(parentCtx context.Context, plugin plugins.Plugin, pluginName string) {
	pluginCfgMap, ok := d.config.GetPluginConfig(pluginName)
	if !ok || pluginCfgMap == nil {
		d.logger.Debug("plugin has no config, using defaults",
			slog.String("plugin", pluginName))
		pluginCfgMap = make(map[string]interface{})
	}

	configForPlugin := cloneConfigMap(pluginCfgMap)
	configForPlugin["enabled"] = true

	pluginCtx, cancel := context.WithCancel(parentCtx)
	pluginConfigCtx := context.WithValue(pluginCtx, contextkeys.PluginConfig, configForPlugin)
	pluginConfigCtx = context.WithValue(pluginConfigCtx, contextkeys.Logger, d.logger)

	instance := &pluginInstance{
		ctx:    pluginCtx,
		cancel: cancel,
		plugin: plugin,
	}

	d.pluginsMu.Lock()
	d.plugins[pluginName] = instance
	d.pluginsMu.Unlock()

	if initializable, ok := plugin.(plugins.Initializable); ok {
		if err := initializable.Initialize(pluginConfigCtx); err != nil {
			d.logger.Error("failed to initialize plugin",
				slog.String("plugin", pluginName),
				slog.String("error", err.Error()))
			cancel()
			return
		}
	}

	if provider, ok := plugin.(plugins.ServiceProvider); ok {
		d.servicesMu.Lock()
		services := provider.Services()
		for name, service := range services {
			d.services[name] = service
			d.logger.Debug("plugin registered service",
				slog.String("plugin", pluginName),
				slog.String("service", name))
		}
		d.servicesMu.Unlock()
	}

	if injector, ok := plugin.(plugins.ServiceInjector); ok {
		d.servicesMu.RLock()
		servicesCopy := make(map[string]interface{})
		for k, v := range d.services {
			servicesCopy[k] = v
		}
		d.servicesMu.RUnlock()

		if err := injector.InjectServices(servicesCopy); err != nil {
			d.logger.Error("failed to inject services into plugin",
				slog.String("plugin", pluginName),
				slog.String("error", err.Error()))
			cancel()
			return
		}
	}

	instance.wg.Add(1)
	d.pluginWG.Add(1)
	go func() {
		defer instance.wg.Done()
		defer d.pluginWG.Done()

		metrics.GlobalSnapshot.RecordPluginStart(pluginName)
		d.logger.Info("plugin started", slog.String("plugin", pluginName))

		if err := plugin.Start(pluginConfigCtx); err != nil {
			metrics.GlobalSnapshot.RecordPluginError(pluginName, err)
			d.logger.Error("plugin error",
				slog.String("plugin", pluginName),
				slog.String("error", err.Error()))
			return
		}

		d.logger.Info("plugin stopped", slog.String("plugin", pluginName))
	}()
}

func (d *Daemon) stopPlugin(pluginName string) error {
	d.pluginsMu.Lock()
	instance, exists := d.plugins[pluginName]
	if !exists {
		d.pluginsMu.Unlock()
		return fmt.Errorf("plugin %s not found", pluginName)
	}
	d.pluginsMu.Unlock()

	d.logger.Debug("stopping plugin", slog.String("plugin", pluginName))
	instance.cancel()

	done := make(chan struct{})
	go func() {
		instance.wg.Wait()
		close(done)
	}()

	stopped := false
	select {
	case <-done:
		d.logger.Debug("plugin stopped gracefully", slog.String("plugin", pluginName))
		stopped = true
	case <-time.After(PluginShutdownTimeout):
		d.logger.Warn("plugin did not stop within timeout", slog.String("plugin", pluginName))
	}

	if stopped {
		d.pluginsMu.Lock()
		delete(d.plugins, pluginName)
		d.pluginsMu.Unlock()
		return nil
	}

	return fmt.Errorf("plugin %s did not stop within timeout", pluginName)
}

func (d *Daemon) restartPlugin(pluginName string) {
	plugin, err := plugins.Get(pluginName)
	if err != nil {
		d.logger.Error("failed to get plugin for restart",
			slog.String("plugin", pluginName),
			slog.String("error", err.Error()))
		return
	}

	if err := d.stopPlugin(pluginName); err != nil {
		d.logger.Error("failed to stop plugin before restart",
			slog.String("plugin", pluginName),
			slog.String("error", err.Error()))
		return
	}

	metrics.GlobalSnapshot.RecordPluginRestart(pluginName)
	d.logger.Info("restarting plugin", slog.String("plugin", pluginName))
	d.startPlugin(d.pluginCtx, plugin, pluginName)
}
