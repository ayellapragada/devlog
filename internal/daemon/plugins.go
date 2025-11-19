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
}

func (d *Daemon) startPlugins(ctx context.Context) {
	pluginCtx, cancel := context.WithCancel(ctx)
	d.pluginCtx = pluginCtx
	d.pluginCtxCancel = cancel

	allPlugins := plugins.List()
	for _, plugin := range allPlugins {
		pluginName := plugin.Name()
		if !d.config.IsPluginEnabled(pluginName) {
			d.logger.Debug("plugin disabled, skipping",
				slog.String("plugin", pluginName))
			continue
		}

		d.startPlugin(pluginCtx, plugin, pluginName)
	}
}

func (d *Daemon) startPlugin(parentCtx context.Context, plugin plugins.Plugin, pluginName string) {
	pluginCfgMap, ok := d.config.GetPluginConfig(pluginName)
	if !ok || pluginCfgMap == nil {
		d.logger.Warn("plugin config not found, using empty config",
			slog.String("plugin", pluginName))
		pluginCfgMap = make(map[string]interface{})
	}

	configForPlugin := cloneConfigMap(pluginCfgMap)
	configForPlugin["enabled"] = true

	pluginCtx, cancel := context.WithCancel(parentCtx)
	pluginConfigCtx := context.WithValue(pluginCtx, contextkeys.PluginConfig, configForPlugin)

	instance := &pluginInstance{
		ctx:    pluginCtx,
		cancel: cancel,
	}

	d.pluginsMu.Lock()
	d.plugins[pluginName] = instance
	d.pluginsMu.Unlock()

	instance.wg.Add(1)
	d.pluginWG.Add(1)
	go func() {
		defer instance.wg.Done()
		defer d.pluginWG.Done()

		metrics.GlobalSnapshot.RecordPluginStart(pluginName)

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
