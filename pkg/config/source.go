package config

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

type ConfigSources interface {
	Name() string
	Load(ctx context.Context) (map[string]interface{}, error)
	Watch(ctx context.Context, onChange func(ConfigChange)) error

	Priority() int
}

type FileSource struct {
	paths    []string
	priority int
	watcher  FileWatcher
	lastLoad time.Time
}

func NewFileSource(paths []string, priority int) *FileSource {
	return &FileSource{
		paths:    paths,
		priority: priority,
		watcher:  NewFileWatcher(),
	}
}

func (f *FileSource) Name() string {
	return "file"
}

func (f *FileSource) Priority() int {
	return f.priority
}

func (f *FileSource) Load(ctx context.Context) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	for _, path := range f.paths {
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("failed to read file %s: %w", path, err)
		}
		var config map[string]interface{}
		if err := json.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("failed to unmarshal file %s: %w", path, err)
		}
		result = mergeMaps(result, config)

	}
	f.lastLoad = time.Now()
	return result, nil
}

func (f *FileSource) Watch(ctx context.Context, onChange func(ConfigChange)) error {
	for _, path := range f.paths {
		if err := f.watcher.Watch(path, func() {
			config, err := f.Load(ctx)
			if err != nil {
				return
			}

			onChange(ConfigChange{
				Key:       "file",
				NewValue:  config,
				Source:    SourceFile,
				Timestamp: time.Now(),
			})
		}); err != nil {
			return fmt.Errorf("failed to watch file %s: %w", path, err)
		}
	}
	return nil
}

type EnironmentSource struct {
	prefix   string
	priority int
}

func NewEnvironmentSource(prefix string, priority int) *EnironmentSource {
	return &EnironmentSource{
		prefix:   prefix,
		priority: priority,
	}
}

func (e *EnironmentSource) Name() string {
	return "environment"
}

func (e *EnironmentSource) Priority() int {
	return e.priority
}

func (e *EnironmentSource) Load(ctx context.Context) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := parts[0]
		value := parts[1]
		if e.prefix != "" && !strings.HasPrefix(key, e.prefix) {
			continue
		}

		configKey := strings.ToLower(strings.TrimPrefix(key, e.prefix))
		configKey = strings.ReplaceAll(configKey, "_", ".")

		parsedValue := parseEnvValue(value)

		setNestedValue(result, configKey, parsedValue)

	}
	return result, nil
}

func (e *EnironmentSource) Watch(ctx context.Context, onChange func(ConfigChange)) error {
	return nil
}

type FlagSource struct {
	args     map[string]interface{}
	priority int
}

func NewFlagSource(args map[string]interface{}, priority int) *FlagSource {
	return &FlagSource{
		args:     args,
		priority: priority,
	}
}

func (f *FlagSource) Name() string {
	return "flag"
}

func (f *FlagSource) Priority() int {
	return f.priority
}

func (f *FlagSource) Load(ctx context.Context) (map[string]interface{}, error) {
	return f.args, nil
}

func (f *FlagSource) Watch(ctx context.Context, onChange func(ConfigChange)) error {
	return nil
}

type DynamicSource struct {
	backend  DynamicBackend
	priority int
	watchCh  chan ConfigChange
}

type DynamicBackend interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Watch(ctx context.Context, key string) (<-chan []byte, error)
	List(ctx context.Context, prefix string) (map[string][]byte, error)
	Put(ctx context.Context, key string, value []byte) error
}

func (d *DynamicSource) Name() string {
	return "dynamic"
}

func (d *DynamicSource) Priority() int {
	return d.priority

}

func (d *DynamicSource) Load(ctx context.Context) (map[string]interface{}, error) {
	kvPairs, err := d.backend.List(ctx, "/config/")
	if err != nil {
		return nil, fmt.Errorf("failed to list dynamic config: %w", err)
	}

	result := make(map[string]interface{})

	for key, value := range kvPairs {
		configKey := strings.TrimPrefix(key, "/config/")
		var parsed interface{}
		if err := json.Unmarshal(value, &parsed); err != nil {
			parsed = string(value)
		}
		setNestedValue(result, configKey, parsed)
	}
	return result, nil
}
