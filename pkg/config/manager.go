package config

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"
)

type ConfigManager struct {
	sources     []ConfigSource
	values      map[string]*ConfigValue
	defaults    map[string]interface{}
	validators  map[string][]ConfigValidator
	watchers    map[string][]ConfigWatcher
	schema      *ConfigSchema
	mu          sync.RWMutex
	onChange    chan ConfigChange
	ctx         context.Context
	cancel      context.Context
	logger      Logger
	secretStore SecretStore
}

type Logger interface {
	Debug(msg string, args ...interface{})
	Info(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Error(msg string, args ...interface{})
}

type SecretStore interface {
	GetSecret(key string) (string, error)
	SetSecret(key string, value string) error
	DeleteSecret(key string) error
	ListSecrets() ([]string, error)
}

func NewConfigManager(logger Logger, secretStore SecretStore) *ConfigManager {
	ctx, cancel := context.WithCancel(context.Background())

	return &ConfigManager{
		sources:     make([]ConfigSource, 0),
		values:      make(map[string]*ConfigValue),
		defaults:    make(map[string]interface{}),
		validators:  make(map[string][]ConfigValidator),
		watchers:    make(map[string][]ConfigWatcher),
		onChange:    make(chan ConfigChange, 100),
		ctx:         ctx,
		cancel:      cancel,
		logger:      logger,
		secretStore: secretStore,
	}
}

func (m *ConfigManager) AddSource(source ConfigSource) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.sources = append(m.sources, source)

	for i := len(m.sources) - 1; i > 0; i-- {
		if m.sources[i].Priority() > m.sources[i-1].Priority() {
			m.sources[i], m.sources[i-1] = m.sources[i-1], m.sources[i]
		}
	}
	return nil
}

func (m *ConfigManager) AddValidator(key string, validator ConfigValidator) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.validators[key] == nil {
		m.validators[key] = make([]ConfigValidator, 0)
	}
	m.validators[key] = append(m.validators[key], validator)
}

func (m *ConfigManager) AddWatcher(key string, watcher ConfigWatcher) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.watchers[key] == nil {
		m.watchers[key] = make([]ConfigWatcher, 0)
	}
	m.watchers[key] = append(m.watchers[key], watcher)
}

func (m *ConfigManager) SetSchema(schema *ConfigSchema) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.schema = schema

	for key, node := range schema.Properties {
		if node.Default != nil {
			m.defaults[key] = node.Default
		}
	}
	return nil
}

func (m *ConfigManager) Load(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for key, defaultValue := range m.defaults {
		m.values[key] = &ConfigValue{
			Value:     defaultValue,
			Source:    SourceDefault,
			IsSet:     true,
			IsDefault: true,
			Timestamp: time.Now(),
		}
	}

	for _, source := range m.sources {
		config, err := source.Load(ctx)
		if err != nil {
			m.logger.Warn("Failed to load from source", "source", source.Name(), "error", err)
			continue
		}
		m.applyConfig(config, source.Priority())
	}

	if err := m.validateAll(); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}
	return nil
}

func (m *ConfigManager) applyConfig(config map[string]interface{}, priority int) {
	var flatten func(prefix string, value interface{})
	flatten = func(prefix string, value interface{}) {
		switch v := value.(type) {
		case map[string]interface{}:
			for k, val := range v {
				newPrefix := prefix

				if prefix == "" {
					newPrefix = k
				} else {
					newPrefix = prefix + "." + k
				}
				flatten(newPrefix, val)
			}
		default:
			existing, exists := m.values[prefix]
			if !exists || priority > int(existing.Source) {
				m.values[prefix] = &ConfigValue{
					Value:     v,
					Source:    ConfigSource(priority),
					IsSet:     true,
					IsDefault: false,
					Timestamp: time.Now(),
				}
				if m.isSecretKey(prefix) {
					m.values[prefix].IsSecret = true
				}

				if m.isDynamicKey(prefix) {
					m.values[prefix].IsDynamic = true
				}
			}
		}
	}
	flatten("", config)

}

func (m *ConfigManager) validateAll() error {
	var multiErr MultiError
	for key, value := range m.values {
		if !value.IsSet {
			continue
		}
		validators := m.validators[key]
		for _, validator := range validators {
			if err := validator.Validate(key, value.Value); err != nil {
				multiErr.Add(&ConfigError{
					Key:     key,
					Message: "validation failed",
					Err:     err,
				})
			}
		}

		if m.schema != nil {
			if err := m.validateAgainstSchema(key, value.Value); err != nil {
				multiErr.Add(err)
			}
		}

	}
	if multiErr.HasErrors() {
		return &multiErr
	}

	return nil
}

func (m *ConfigManager) validateAgainstSchema(
	key string, value interface{},
) error {
	parts := strings.Split(key, ".")
	currentNode := m.schema.Properties
	for i, part := range parts {
		node, exists := currentNode[part]
		if !exists {
			if i == len(parts)-1 {
				return nil
			}
			return nil
		}
		if i == len(parts)-1 {
			return m.validateNode(node, value)
		}
		if node.Properties == nil {
			return &ConfigError{
				Key:     key,
				Message: "schema mismatch: expected object",
			}

		}
		currentNode = node.Properties
	}
	return nil
}

func (m *ConfigManager) validateNode(node *SchemaNode, value interface{}) error {
	valueType := reflect.TypeOf(value)
	switch node.Type {
	case "string":
		if valueType.Kind() != reflect.String {
			return &ConfigError{
				Message: fmt.Sprintf("expected string, got %s", valueType.Kind()),
			}
		}
		// strValue := value.(string)
		if node.Pattern != "" {

		}
	case "integer":
		if valueType.Kind() != reflect.Int && valueType.Kind() != reflect.Float64 {
			return &ConfigError{
				Message: fmt.Sprintf("expected integer, got %s", valueType.Kind()),
			}
		}
		if node.Min != nil {
			min, _ := node.Min.(float64)
			if value.(float64) < min {
				return &ConfigError{
					Message: fmt.Sprintf("value %f is less than min %f", value.(float64), min),
				}
			}
		}
		if node.Max != nil {
			max, _ := node.Max.(float64)
			if value.(float64) > max {
				return &ConfigError{
					Message: fmt.Sprintf("value %v is greater then max %v", value, max),
				}
			}

		}
	case "number":
		if valueType.Kind() != reflect.Float64 && valueType.Kind() != reflect.Int {
			return &ConfigError{
				Message: fmt.Sprintf("expected number, got %s", valueType.Kind()),
			}
		}
	case "array":
		if valueType.Kind() != reflect.Slice && valueType.Kind() != reflect.Array {
			return &ConfigError{
				Message: fmt.Sprint("expected array, got %s", valueType.Kind()),
			}
		}
		if node.Items != nil {
			slice := reflect.ValueOf(value)
			for i := 0; i < slice.Len(); i++ {
				if err := m.validateNode(node.Items, slice.Index(i).Interface()); err != nil {
					return &ConfigError{
						Message: fmt.Sprintf("item %d: %v", i, err),
					}
				}
			}
		}
	case "boolean":
		if valueType.Kind() != reflect.Bool {
			return &ConfigError{
				Message: fmt.Sprintf("expected boolean, got %s", valueType.Kind()),
			}
		}
	case "object":
		if valueType.Kind() != reflect.Map {
			return &ConfigError{
				Message: fmt.Sprintf("expected object, got %s", valueType.Kind()),
			}
		}

		if len(node.Enum) > 0 {
			found := false
			for _, enumValue := range node.Enum {
				if reflect.DeepEqual(enumValue, value) {
					found = true
					break
				}
			}
			if !found {
				return &ConfigError{
					Message: fmt.Sprintf("value %v is not in enum values", value),
				}
			}
		}
	}
	return nil

}

func (m *ConfigManager) notifyWatchers(change ConfigChange) {
	m.mu.RLock()
	watchers := m.watchers[change.Key]
	m.mu.RUnlock()
	for _, watcher := range watchers {
		go watcher.OnConfigChange(change)
	}

	select {
	case m.onChange <- change:
	default:
		m.logger.Warn("Config change channel full, dropping change", "key", change.Key)
	}
}

func (m *ConfigManager) Watch() <-chan ConfigChange {
	return m.onChange
}

func (m *ConfigManager) isSecretKey(key string) bool {
	if m.schema == nil {
		return false
	}
	parts := strings.Split(key, ".")
	currentNode := m.schema.Properties

	for i, part := range parts {
		node, exists := currentNode[part]
		if !exists {
			return false
		}
		if i == len(parts)-1 {
			return node.Secret
		}
		if node.Properties == nil {
			return false
		}
	}
	return false

}

func (m *ConfigManager) isDynamicKey(key string) bool {
	if m.schema == nil {
		return false
	}
	parts := strings.Split(key, ".")
	currentNode := m.schema.Properties

	for i, part := range parts {
		node, exists := currentNode[part]
		if !exists {
			return false
		}
		if i == len(parts)-1 {
			return node.Dynamic
		}
		if node.Properties == nil {
			return false
		}
		currentNode = node.Properties
	}
	return false
}
