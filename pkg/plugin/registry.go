package plugin

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

var (
	ErrPluginNotFound      = errors.New("plugin not found")
	ErrPluginAlreadyLoaded = errors.New("plugin already loaded")
	ErrDependencyMissing   = errors.New("missing dependency")
	ErrCircularDependency  = errors.New("circular dependency detected")
	ErrPluginNotReady      = errors.New("plugin not ready")
)

// PluginInfo holds information about a loaded plugin
type PluginInfo struct {
	Metadata   PluginMetadata
	Instance   Plugin
	State      PluginState
	Config     map[string]interface{}
	LoadedAt   time.Time
	StartedAt  time.Time
	Hooks      map[HookType][]HookHandler
	Dependents []string
}

type PluginRegistry struct {
	mu          sync.RWMutex
	plugins     map[string]*PluginInfo
	pluginOrder []string
	hooks       map[HookType][]*HookRegistration

	capabilities   map[string][]string
	pluginDir      string
	logger         Logger
	configProvider ConfigProvider
}

type HookRegistration struct {
	PluginID string
	Handler  HookHandler
	Priority int
}

type Logger interface {
	Debug(msg string, args ...interface{})
	Info(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Error(msg string, args ...interface{})
}

type ConfigProvider interface {
	GetPluginConfig(pluginID string) (map[string]interface{}, error)
}

// NewPluginRegistry creates a new plugin registry
func NewPluginRegistry(
	pluginDir string, logger Logger, configProvider ConfigProvider,
) *PluginRegistry {
	return &PluginRegistry{
		plugins:        make(map[string]*PluginInfo),
		hooks:          make(map[HookType][]*HookRegistration),
		capabilities:   make(map[string][]string),
		pluginDir:      pluginDir,
		logger:         logger,
		configProvider: configProvider,
	}
}

func (r *PluginRegistry) RegisterPlugin(plugin Plugin) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	metadata := plugin.Metadata()
	pluginID := metadata.ID

	if _, exists := r.plugins[pluginID]; exists {
		return fmt.Errorf("%w: %s", ErrPluginAlreadyLoaded, pluginID)
	}

	info := &PluginInfo{
		Metadata: metadata,
		Instance: plugin,
		State:    StateLoaded,
		LoadedAt: time.Now(),
		Hooks:    make(map[HookType][]HookHandler),
	}

	r.plugins[pluginID] = info

	r.logger.Info("Plugin registered", "plugin_id", pluginID, "name", metadata.Name)

	return nil
}

func (r *PluginRegistry) GetPlugin(pluginID string) (Plugin, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	info, exists := r.plugins[pluginID]

	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrPluginNotFound, pluginID)
	}
	return info.Instance, nil
}

func (r *PluginRegistry) GetPluginInfo(pluginID string) (*PluginInfo, error) {
	r.mu.RLock()
	defer r.mu.Unlock()

	info, exists := r.plugins[pluginID]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrPluginNotFound, pluginID)
	}

	return info, nil
}

func (r *PluginRegistry) AddHook(pluginID string, hookType HookType,
	handler HookHandler, priority int,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	info, exists := r.plugins[pluginID]

	if !exists {
		return fmt.Errorf("%w: %s", ErrPluginNotFound, pluginID)
	}

	if _, exists := info.Hooks[hookType]; !exists {
		info.Hooks[hookType] = append(info.Hooks[hookType], handler)
	}

	registration := &HookRegistration{
		PluginID: pluginID,
		Handler:  handler,
		Priority: priority,
	}

	hooks := r.hooks[hookType]
	hooks = append(hooks, registration)

	sort.Slice(hooks, func(i, j int) bool {
		return hooks[i].Priority < hooks[j].Priority
	})

	r.hooks[hookType] = hooks
	r.logger.Debug("hook registered", "plugin", pluginID, "hook", hookType, "priority", priority)
	return nil
}

func (r *PluginRegistry) ExecuteHooks(ctx context.Context, hookType HookType,
	data map[string]interface{}) error {
	r.mu.RLock()
	hooks := r.hooks[hookType]
	r.mu.RUnlock()

	if len(hooks) == 0 {
		return nil
	}

	for _, registration := range hooks {
		hookCtx := &HookContext{
			Ctx:      ctx,
			PluginID: registration.PluginID,
			Data:     data,
		}

		if err := registration.Handler(hookCtx); err != nil {
			r.logger.Error("Hook execution failed",
				"plugin", registration.PluginID,
				"hook", hookType, "error", err)
			return fmt.Errorf("hook %s from plugin %s failed %w",
				hookType, registration.PluginID, err)
		}
	}
	return nil
}
