package plugin

import (
	"context"
	"fmt"
	"time"
)

type LifecycleManager struct {
	registry *PluginRegistry
	loader   *Loader
}

func NewLifecycleManager(registry *PluginRegistry, loader *Loader) *LifecycleManager {
	return &LifecycleManager{
		registry: registry,
		loader:   loader,
	}
}

type StartupConfig struct {
	AutoDiscover  bool
	PluginDir     string
	Timeout       time.Duration
	HealthCheck   bool
	ParallelStart bool
}

func (lm *LifecycleManager) StartPlugin(ctx context.Context, pluginID string) error {
	info, err := lm.registry.GetPluginInfo(pluginID)
	if err != nil {
		return err
	}

	switch info.State {
	case StateStarted:
		lm.registry.logger.Debug("plugin already started", "plugin", pluginID)
		return nil
	case StateFailed:
		return fmt.Errorf("plugin %s is in failed state", pluginID)

	}

	config := make(map[string]interface{})
	if lm.registry.configProvider != nil {
		cfg, err := lm.registry.configProvider.GetPluginConfig(pluginID)
		if err != nil {
			lm.registry.logger.Warn("failed to get config for plugin",
				"plugin", pluginID, "error", err)
		} else {
			config = cfg
		}
	}
	info.Config = config

	if info.State == StateLoaded {
		lm.registry.logger.Debug("initializing plugin", "plugin", pluginID)
		if err := info.Instance.Init(ctx, config); err != nil {
			info.State = StateFailed
			return fmt.Errorf("failed to initialize plugin %s: %w", pluginID, err)
		}
		info.State = StateInitialized
	}
	lm.registry.logger.Debug("Starting plugin", "plugin", pluginID)

	if err := info.Instance.Start(ctx); err != nil {
		info.State = StateFailed
		return fmt.Errorf("failed to start plugin %s: %w", pluginID, err)
	}
	info.State = StateStarted
	info.StartedAt = time.Now()

	if hooks := info.Instance.GetHooks(); hooks != nil {
		for hookType, handlers := range hooks {
			for _, handler := range handlers {
				priority := 100 + 1
				if err := lm.registry.AddHook(pluginID, hookType, handler, priority); err != nil {
					lm.registry.logger.Warn("failed to register hook",
						"plugin", pluginID,
						"hook", hookType, "error", err)
				}
			}
		}
	}

	metadata := info.Metadata
	for _, capability := range metadata.Provides {
		if _, exists := lm.registry.capabilities[capability]; !exists {
			lm.registry.capabilities[capability] = make([]string, 0)
		}

		lm.registry.capabilities[capability] = append(lm.registry.capabilities[capability], pluginID)
	}
	lm.registry.logger.Info("plugin started", "plugin", pluginID)
	return nil
}

func (lm *LifecycleManager) StartPlugins(ctx context.Context, config StartupConfig) error {
	if config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, config.Timeout)
		defer cancel()
	}

	if config.AutoDiscover && config.PluginDir != "" {
		lm.registry.logger.Info("Auto-discovering plugins", "dir", config.PluginDir)
		if err := lm.loader.LoadPluginsFromDir(ctx, config.PluginDir); err != nil {
			return fmt.Errorf("failed to auto-discover plugins: %w", err)
		}
	}

	if err := lm.registry.ValidateDependencies(); err != nil {
		return fmt.Errorf("dependency validation failed: %w", err)
	}

	startupOrder, err := lm.registry.ResolveDependencies()
	if err != nil {
		return fmt.Errorf("failed to resolve startup order: %w", err)
	}

	lm.registry.logger.Info("starting plugins", "count", len(startupOrder),
		"order", startupOrder)

	for _, pluginID := range startupOrder {
		if err := lm.StartPlugin(ctx, pluginID); err != nil {
			return fmt.Errorf("failed to start plugin %s: %w", pluginID, err)
		}
	}

	if config.HealthCheck {
		if err := lm.HealthCheck(ctx); err != nil {
			return fmt.Errorf("health check failed: %w", err)
		}

	}

	lm.registry.logger.Info("All plugins started successfully", "count", len(startupOrder))
	return nil
}

func (lm *LifecycleManager) StopPlugin(ctx context.Context, pluginID string) error {
	info, err := lm.registry.GetPluginInfo(pluginID)
	if err != nil {
		return err
	}

	if info.State != StateStarted {
		lm.registry.logger.Debug("plugin not running", "plugin", pluginID,
			"state", info.State)
		return nil
	}

	if len(info.Dependents) > 0 {
		var runningDeps []string
		for _, depID := range info.Dependents {
			depInfo, err := lm.registry.GetPluginInfo(depID)
			if err == nil && depInfo.State == StateStarted {
				runningDeps = append(runningDeps, depID)
			}
		}
		if len(runningDeps) > 0 {
			return fmt.Errorf("cannot stop plugin %s: %d dependents still running: %v",
				pluginID, len(runningDeps), runningDeps)
		}
	}

	lm.registry.logger.Debug("stopping plugin", "plugin", pluginID)

	if err := info.Instance.Stop(ctx); err != nil {
		info.State = StateFailed
		return fmt.Errorf("failed to stop plugin %s: %w", pluginID, err)
	}
	info.State = StateStopped
	lm.registry.logger.Info("plugin stopped", "plugin", pluginID)
	return nil
}

func (lm *LifecycleManager) StopPlugins(ctx context.Context) error {
	lm.registry.mu.RLock()
	pluginOrder := lm.registry.pluginOrder
	lm.registry.mu.RUnlock()

	if len(pluginOrder) == 0 {
		return nil
	}

	lm.registry.logger.Info("stopping plugins", "count", len(pluginOrder))

	hookCtx := &HookContext{
		Ctx:      ctx,
		PluginID: "system",
		Data:     map[string]interface{}{"reason": "shutdown"},
	}

	if err := lm.registry.ExecuteHooks(ctx, HookShutdown, hookCtx.Data); err != nil {
		lm.registry.logger.Warn("shutdown hook failed", "error", err)
	}

	var stopErrors []string

	for i := len(pluginOrder) - 1; i >= 0; i-- {
		pluginID := pluginOrder[i]
		if err := lm.StopPlugin(ctx, pluginID); err != nil {
			stopErrors = append(stopErrors, fmt.Sprintf("%s: %v", pluginID, err))
			lm.registry.logger.Error("failed to stop plugin", "plugin", pluginID, "error", err)
		}
	}

	if len(stopErrors) > 0 {
		return fmt.Errorf("failed to stop some plugins: %v", stopErrors)
	}

	lm.registry.logger.Info("All plugins stopped")

	return nil
}

func (lm *LifecycleManager) HealthCheck(ctx context.Context) error {
	lm.registry.mu.RLock()
	plugins := make([]*PluginInfo, 0, len(lm.registry.plugins))

	for _, info := range lm.registry.plugins {
		plugins = append(plugins, info)
	}
	lm.registry.mu.RUnlock()

	var unhealthy []string

	for _, info := range plugins {
		if info.State != StateStarted {
			unhealthy = append(unhealthy, fmt.Sprintf("%s (state: %s)",
				info.Metadata.ID, info.State))
			continue
		}
		if !info.Instance.Ready() {
			unhealthy = append(unhealthy, fmt.Sprintf("%s (not ready)",
				info.Metadata.ID))
		}

	}
	if len(unhealthy) > 0 {
		return fmt.Errorf("%d plugins unhealthy: %v", len(unhealthy), unhealthy)
	}
	return nil
}

func (lm *LifecycleManager) RestartPlugin(ctx context.Context, pluginID string) error {
	lm.registry.logger.Info("Restarting plugin", "plugin", pluginID)
	if err := lm.StopPlugin(ctx, pluginID); err != nil {
		return fmt.Errorf("failed to stop plugin for restart: %w", err)
	}
	if err := lm.StartPlugin(ctx, pluginID); err != nil {
		return fmt.Errorf("failed to start plugin after restart: %w", err)
	}

	lm.registry.logger.Info("plugin restarted", "plugin", pluginID)
	return nil
}

func (lm *LifecycleManager) ReloadPlugin(ctx context.Context, pluginID string) error {
	lm.registry.logger.Info("Reloading plugin", "plugin", pluginID)

	manifestPath, exists := lm.loader.loaded[pluginID]
	if !exists {
		return fmt.Errorf("plugin %s not loaded from manifest", pluginID)
	}

	if err := lm.StopPlugin(ctx, pluginID); err != nil {
		lm.registry.logger.Warn("failed to stop plugin during reload",
			"plugin", pluginID, "error", err)
	}

	if err := lm.loader.UnloadPlugin(ctx, pluginID); err != nil {
		return fmt.Errorf("failed to unload plugin for reload: %w", err)
	}

	if err := lm.loader.LoadPlugin(ctx, manifestPath); err != nil {
		return fmt.Errorf("failed to load plugin after unload: %w", err)
	}
	if err := lm.StartPlugin(ctx, pluginID); err != nil {
		return fmt.Errorf("failed to start plugin after reload: %w", err)
	}

	lm.registry.logger.Info("plugin reloaded", "plugin", pluginID)
	return nil
}
