package config

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"
)

type ConfigManager struct {
	sources     []ConfigSources
	values      map[string]*ConfigValue
	defaults    map[string]interface{}
	validators  map[string][]ConfigValidator
	watchers    map[string][]ConfigWatcher
	schema      *ConfigSchema
	mu          sync.RWMutex
	onChange    chan ConfigChange
	ctx         context.Context
	cancel      context.CancelFunc
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
		sources:     make([]ConfigSources, 0),
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

func (m *ConfigManager) AddSource(source ConfigSources) error {
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

func (m *ConfigManager) SetDefault(key string, value interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.defaults[key] = value

	if _, exists := m.values[key]; !exists {
		m.values[key] = &ConfigValue{
			Value:     value,
			Source:    SourceDefault,
			IsSet:     true,
			IsDefault: true,
			Timestamp: time.Now(),
		}
	}

}

func (m *ConfigManager) Get(key string) (interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	value, exists := m.values[key]
	if !exists {
		return nil, &ConfigError{
			Key:     key,
			Message: "key not found",
		}
	}

	if value.IsSecret && m.secretStore != nil {
		secretValue, err := m.secretStore.GetSecret(key)
		if err == nil {
			return secretValue, nil
		}
		m.logger.Warn("failed to get secret", "key", key, "error", err)
	}
	return value.Value, nil
}

func (m *ConfigManager) GetString(key string) (string, error) {
	value, err := m.Get(key)
	if err != nil {
		return "", err
	}
	strValue, ok := value.(string)
	if !ok {
		return "", &ConfigError{
			Key:     key,
			Message: "value is not a string",
		}
	}
	return strValue, nil
}

func (m *ConfigManager) GetInt(key string) (int, error) {
	value, err := m.Get(key)
	if err != nil {
		return 0, err
	}
	switch v := value.(type) {
	case int:
		return v, nil
	case float64:
		return int(v), nil
	case json.Number:
		i, err := v.Int64()
		if err != nil {
			return 0, &ConfigError{
				Key:     key,
				Message: "cannot convert to int",
			}
		}
		return int(i), nil
	default:
		return 0, &ConfigError{
			Key:     key,
			Message: fmt.Sprintf("value is not an int: %T", value),
		}
	}
}

func (m *ConfigManager) GetBool(key string) (bool, error) {
	value, err := m.Get(key)
	if err != nil {
		return false, err
	}
	boolValue, ok := value.(bool)
	if !ok {
		return false, &ConfigError{
			Key:     key,
			Message: "value is not a bool",
		}
	}
	return boolValue, nil
}

func (m *ConfigManager) GetDuration(key string) (time.Duration, error) {
	value, err := m.Get(key)
	if err != nil {
		return 0, err
	}

	switch v := value.(type) {
	case time.Duration:
		return v, nil
	case string:
		return time.ParseDuration(v)
	case int:
		return time.Duration(v) * time.Second, nil
	case float64:
		return time.Duration(v) * time.Second, nil
	default:
		return 0, &ConfigError{
			Key:     key,
			Message: "cannot convert to duration",
		}
	}
}

func (m *ConfigManager) GetFloat(key string) (float64, error) {
	value, err := m.Get(key)
	if err != nil {
		return 0, err
	}

	switch v := value.(type) {
	case float64:
		return v, nil
	case int:
		return float64(v), nil
	case json.Number:
		return v.Float64()
	default:
		return 0, &ConfigError{
			Key:     key,
			Message: "cannot convert to float",
		}
	}
}

func (m *ConfigManager) GetStringSlice(key string) ([]string, error) {
	value, err := m.Get(key)
	if err != nil {
		return nil, err
	}

	switch v := value.(type) {
	case []interface{}:
		result := make([]string, len(v))
		for i, item := range v {
			str, ok := item.(string)
			if !ok {
				return nil, &ConfigError{
					Key:     key,
					Message: "slice contains non-string",
				}
			}
			result[i] = str
		}
		return result, nil
	case []string:
		return v, nil
	default:
		return nil, &ConfigError{
			Key:     key,
			Message: "value is not string slice",
		}
	}
}

func (m *ConfigManager) Set(key string, value interface{},
	source ConfigSource, dynamic bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	oldValue, exists := m.values[key]

	newValue := &ConfigValue{
		Value:     value,
		Source:    source,
		IsSet:     true,
		IsDefault: false,
		Timestamp: time.Now(),
	}

	if m.isSecretKey(key) {
		newValue.IsSecret = true
		if m.secretStore != nil {
			strValue, ok := value.(string)
			if ok {
				if err := m.secretStore.SetSecret(key, strValue); err != nil {
					m.logger.Error("failed to set secret", "key", key, "error", err)
				}
			}
		}
	}

	m.values[key] = newValue

	change := ConfigChange{
		Key:       key,
		OldValue:  nil,
		NewValue:  value,
		Source:    source,
		Timestamp: time.Now(),
	}

	if exists {
		change.OldValue = oldValue.Value
	}
	go m.notifyWatchers(change)
	return nil
}

func (m *ConfigManager) AddDefault(key string, value interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.defaults[key] = value

	if _, exists := m.values[key]; !exists {
		m.values[key] = &ConfigValue{
			Value:     value,
			Source:    SourceDefault,
			IsSet:     true,
			IsDefault: true,
			Timestamp: time.Now(),
		}
	}
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

	if err := m.ValidateAll(); err != nil {
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

func (m *ConfigManager) ValidateAll() error {
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
