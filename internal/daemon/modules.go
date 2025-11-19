package daemon

import (
	"context"
	"fmt"
	"log/slog"

	"devlog/internal/config"
	"devlog/internal/modules"
)

func (d *Daemon) setupPollers() {
	allModules := modules.List()
	for _, module := range allModules {
		moduleName := module.Name()

		if !d.config.IsModuleEnabled(moduleName) {
			continue
		}

		d.registerModule(module, moduleName)
	}
}

func (d *Daemon) registerModule(module modules.Module, moduleName string) {
	ctx := d.moduleCtx
	if ctx == nil {
		ctx = context.Background()
	}
	d.setupModule(module, moduleName, false, ctx)
}

func (d *Daemon) startModule(module modules.Module, moduleName string) {
	ctx := d.moduleCtx
	if ctx == nil {
		ctx = context.Background()
	}
	d.setupModule(module, moduleName, true, ctx)
}

func (d *Daemon) startModuleWithContext(module modules.Module, moduleName string, ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	d.setupModule(module, moduleName, true, ctx)
}

func (d *Daemon) setupModule(module modules.Module, moduleName string, startImmediately bool, ctx context.Context) {
	pollable, ok := module.(modules.Pollable)
	if !ok {
		return
	}

	dataDir, err := config.DataDir()
	if err != nil {
		d.logger.Error("failed to get data dir for module",
			slog.String("module", moduleName),
			slog.String("error", err.Error()))
		return
	}

	modCfg, ok := d.config.GetModuleConfig(moduleName)
	if !ok {
		d.logger.Warn("module config not found",
			slog.String("module", moduleName))
		return
	}

	poller, err := pollable.CreatePoller(modCfg, dataDir)
	if err != nil {
		d.logger.Warn("failed to create poller",
			slog.String("module", moduleName),
			slog.String("error", err.Error()))
		return
	}

	d.pollerManager.Register(poller)

	if startImmediately {
		d.pollerManager.StartPoller(ctx, poller)
	}

	d.modulesMu.Lock()
	d.modules[moduleName] = poller.Name()
	d.modulesMu.Unlock()

	d.logger.Info("polling started",
		slog.String("module", moduleName),
		slog.Duration("interval", poller.PollInterval()))
}

func (d *Daemon) stopModule(moduleName string) error {
	d.modulesMu.Lock()
	pollerName, exists := d.modules[moduleName]
	if !exists {
		d.modulesMu.Unlock()
		module, err := modules.Get(moduleName)
		if err == nil {
			if _, ok := module.(modules.Pollable); !ok {
				return nil
			}
		}
		d.logger.Warn("module not found in registry",
			slog.String("module", moduleName))
		return fmt.Errorf("module %s not found in registry", moduleName)
	}
	delete(d.modules, moduleName)
	d.modulesMu.Unlock()

	d.logger.Info("stopping module",
		slog.String("module", moduleName),
		slog.String("poller", pollerName))
	d.pollerManager.StopPoller(pollerName)
	return nil
}

func (d *Daemon) restartModule(moduleName string) {
	module, err := modules.Get(moduleName)
	if err != nil {
		d.logger.Error("failed to get module for restart",
			slog.String("module", moduleName),
			slog.String("error", err.Error()))
		return
	}

	if err := d.stopModule(moduleName); err != nil {
		d.logger.Error("failed to stop module before restart",
			slog.String("module", moduleName),
			slog.String("error", err.Error()))
		return
	}

	d.logger.Info("restarting module", slog.String("module", moduleName))
	d.startModule(module, moduleName)
}
