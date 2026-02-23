package config

import (
	"fmt"
	"reflect"
	"strings"
	"time"
)

type ConfigSource int

const (
	SourceDefault ConfigSource = iota
	SourceFile
	SourceEnvironment
	SourceFlag
	SourceDynamic
	SourceSecret
)

func (s ConfigSource) String() string {
	return [...]string{
		"default",
		"file",
		"environment",
		"flag",
		"dynamic",
		"secret",
	}[s]
}

type ConfigValue struct {
	Value     interface{}
	Source    ConfigSource
	IsSet     bool
	IsDefault bool
	IsSecret  bool
	IsDynamic bool
	Timestamp time.Time
}

type ConfigChange struct {
	Key       string
	OldValue  interface{}
	NewValue  interface{}
	Source    ConfigSource
	Timestamp time.Time
}

type ConfigWatcher interface {
	OnConfigChange(change ConfigChange)
}

type ConfigValidator interface {
	Validate(key string, value interface{}) error
}

type ValidationRule struct {
	Key      string
	Required bool
	Type     reflect.Kind
	Min      interface{}
	Max      interface{}
	Pattern  string
	Custom   func(value interface{}) error
}

type ConfigError struct {
	Key     string
	Message string
	Err     error
}

func (e *ConfigError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("config error for key %s: %s: %v", e.Key, e.Message, e.Err)
	}
	return fmt.Sprintf("config error for key %s: %s", e.Key, e.Message)
}

type MultiError struct {
	Errors []error
}

func (e *MultiError) Error() string {
	var msgs []string
	for _, err := range e.Errors {
		msgs = append(msgs, err.Error())
	}
	return fmt.Sprintf("multiple config errors: \n%s", strings.Join(msgs, "\n"))
}

func (e *MultiError) Add(err error) {
	if err != nil {
		e.Errors = append(e.Errors, err)
	}
}

func (e *MultiError) HasErrors() bool {
	return len(e.Errors) > 0
}

type SchemaNode struct {
	Type                 string                 `json:"type"`
	Description          string                 `json:"description"`
	Default              interface{}            `json:"default,omitempty"`
	Required             bool                   `json:"required,omitempty"`
	Secret               bool                   `json:"secret,omitempty"`
	Dynamic              bool                   `json:"dynamic,omitempty"`
	Min                  interface{}            `json:"min,omitempty"`
	Max                  interface{}            `json:"max,omitempty"`
	Pattern              string                 `json:"pattern,omitempty"`
	Enum                 []interface{}          `json:"enum,omitempty"`
	Properties           map[string]*SchemaNode `json:"properties,omitempty"`
	Items                *SchemaNode            `json:"items,omitempty"`
	AdditionalProperties *SchemaNode            `json:"additionalProperties,omitempty"`
}

type ConfigSchema struct {
	Version     string                 `json:"version"`
	Description string                 `json:"description"`
	Properties  map[string]*SchemaNode `json:"properties"`
	Required    []string               `json:"required"`
}
