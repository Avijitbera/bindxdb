package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"plugin"
	"strings"
	"sync"
)

type Loader struct {
	registry *PluginRegistry
	loaded   map[string]string
	mu       sync.RWMutex
}

// NewLoader creates a new plugin loader
func NewLoader(registry *PluginRegistry) *Loader {
	return &Loader{
		registry: registry,
		loaded:   make(map[string]string),
	}
}

type PluginManifest struct {
	Metadata   PluginMetadata `json:"metadata"`
	EntryPoint string         `json:"entry_point"`
	Path       string         `json:"path"`
	Type       string         `json:"type"`
}

func (l *Loader) LoadPlugin(
	ctx context.Context, manifestPath string,
) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	manifest, err := l.readManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to read manifest: %w", err)
	}

	pluginID := manifest.Metadata.ID

	if _, exists := l.loaded[pluginID]; exists {
		return fmt.Errorf("%w: %s", ErrPluginAlreadyLoaded, pluginID)
	}

	var pluginInstance Plugin

	switch manifest.Type {
	case "go":
		pluginInstance, err = l.loadGoPlugin(manifest)
	case "wasm":
		pluginInstance, err = l.loadWASMPlugin(manifest)
	case "external":
		pluginInstance, err = l.loadExternalPlugin(manifest)
	default:
		return fmt.Errorf("unsupported plugin type: %s", manifest.Type)
	}
	if err != nil {
		return fmt.Errorf("failed to load plugin %s: %w", pluginID, err)
	}

	if err := l.registry.RegisterPlugin(pluginInstance); err != nil {
		return fmt.Errorf("failed to register plugin %s: %w", pluginID, err)
	}

	l.loaded[pluginID] = manifestPath
	l.registry.logger.Info("Plugin loaded", "plugin", pluginID, "type", manifest.Type)

	return nil

}

func (l *Loader) LoadPluginsFromDir(ctx context.Context, dir string) error {
	entries, err := ioutil.ReadDir(dir)

	if err != nil {
		return fmt.Errorf("failed to read plugin directory: %w", err)
	}
	var loadErrors []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasSuffix(entry.Name(), ".manifest.json") {
			manifestPath := filepath.Join(dir, entry.Name())
			if err := l.LoadPlugin(ctx, manifestPath); err != nil {
				loadErrors = append(loadErrors, fmt.Sprintf("%s: %v", entry.Name(), err))
				l.registry.logger.Error("failed to load plugin",
					"manifest", entry.Name(),
					"error", err)
			}
		}
	}

	if len(loadErrors) > 0 {
		return fmt.Errorf("failed to load some plugins: %v", strings.Join(loadErrors, "; "))
	}
	return nil
}

func (l *Loader) readManifest(path string) (*PluginManifest, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var manifest PluginManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("invalid mainfest JSON: %w", err)
	}

	if manifest.Metadata.ID == "" {
		return nil, errors.New("manifest missing plugin ID")
	}

	if manifest.Metadata.Name == "" {
		return nil, errors.New("manifest missing plugin name")
	}

	if manifest.Metadata.Version == "" {
		return nil, errors.New("manifest missing plugin version")
	}

	if manifest.Type == "" {
		manifest.Type = "go"
	}

	if manifest.Path == "" && manifest.Type == "go" {
		baseName := strings.TrimSuffix(filepath.Base(path), ".manifest.json")
		manifest.Path = filepath.Join(filepath.Dir(path), baseName+".so")
	}
	return &manifest, nil
}

func (l *Loader) loadGoPlugin(manifest *PluginManifest) (Plugin, error) {
	if _, err := os.Stat(manifest.Path); os.IsNotExist(err) {
		return nil, fmt.Errorf("plugin file not found: %s", manifest.Path)
	}

	p, err := plugin.Open(manifest.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to open plugin: %w", err)
	}

	var pluginSymbol plugin.Symbol
	if manifest.EntryPoint != "" {
		pluginSymbol, err = p.Lookup(manifest.EntryPoint)
	} else {
		for _, name := range []string{"Plugin", "NewPlugin"} {
			pluginSymbol, err = p.Lookup(name)
			if err == nil {
				break
			}
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to find plugin symbol: %w", err)
	}

	var pluginInstance Plugin
	switch p := pluginSymbol.(type) {
	case *Plugin:
		pluginInstance = *p
	case Plugin:
		pluginInstance = p
	case func() Plugin:
		pluginInstance = p()
	case func() (Plugin, error):
		pluginInstance, err = p()
		if err != nil {
			return nil, fmt.Errorf("plugin constructor failed: %w", err)
		}
	default:
		return nil, fmt.Errorf("unexpected plugin type: %T", p)
	}

	metadata := pluginInstance.Metadata()
	if metadata.ID != manifest.Metadata.ID {
		return nil, fmt.Errorf("plugin ID mismatch: manifest=%s, plugin=%s",
			manifest.Metadata.ID, metadata.ID)
	}

	return pluginInstance, nil
}

func (l *Loader) loadWASMPlugin(manifest *PluginManifest) (Plugin, error) {
	return nil, errors.New("WASM plugin support not implemented yet")
}

func (l *Loader) loadExternalPlugin(manifest *PluginManifest) (Plugin, error) {
	return nil, errors.New("external plugin support not implemented yet")
}

func (l *Loader) UnloadPlugin(ctx context.Context, pluginID string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if _, exists := l.loaded[pluginID]; !exists {
		return fmt.Errorf("%w: %s", ErrPluginNotFound, pluginID)
	}

	info, err := l.registry.GetPluginInfo(pluginID)

	if err != nil {
		return err
	}

	if len(info.Dependents) > 0 {
		return fmt.Errorf("cannot unload plugin %s: %d dependents found",
			pluginID, len(info.Dependents))
	}

	if info.State == StateStarted {
		if err := info.Instance.Stop(ctx); err != nil {
			l.registry.logger.Warn("failed to stop plugin during unload",
				"plugin", pluginID, "error", err)
		}
	}

	l.registry.mu.Lock()
	delete(l.registry.plugins, pluginID)
	l.registry.mu.Unlock()

	delete(l.loaded, pluginID)
	l.registry.logger.Info("Plugin unloaded", "plugin", pluginID)
	return nil
}
