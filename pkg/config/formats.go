package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type ConfigFormat interface {
	Name() string
	Extension() []string
	Unmarshal(data []byte) (map[string]interface{}, error)
	Marshal(config map[string]interface{}) ([]byte, error)
}

type JSONFormat struct{}

func (f *JSONFormat) Name() string { return "json" }

func (f *JSONFormat) Extension() []string { return []string{".json"} }

func (f *JSONFormat) Unmarshal(data []byte) (map[string]interface{}, error) {
	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("invlid JSON: %w", err)
	}
	return config, nil
}

func (f *JSONFormat) Marshal(config map[string]interface{}) ([]byte, error) {
	return json.MarshalIndent(config, "", " ")
}

type YAMLFormat struct{}

func (f *YAMLFormat) Name() string        { return "yaml" }
func (f *YAMLFormat) Extension() []string { return []string{".yaml", ".yml"} }

func (f *YAMLFormat) Unmarshal(data []byte) (map[string]interface{}, error) {
	var config map[string]interface{}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("invalid YAML: %w", err)
	}
	return config, nil
}

func (f *YAMLFormat) Marshal(config map[string]interface{}) ([]byte, error) {
	return yaml.Marshal(config)
}

type TOMLFormat struct{}

func (f *TOMLFormat) Name() string { return "toml" }

func (f *TOMLFormat) Extension() []string { return []string{".toml"} }

func (f *TOMLFormat) Unmarshal(data []byte) (map[string]interface{}, error) {
	return nil, fmt.Errorf("TOML format not yet implemented")
}

func (f *TOMLFormat) Marshal(config map[string]interface{}) ([]byte, error) {
	return nil, fmt.Errorf("TOML format not yet implemented")
}

type ConfigLoader struct {
	formats []ConfigFormat
}

func NewConfigLoader() *ConfigLoader {
	loader := &ConfigLoader{
		formats: make([]ConfigFormat, 0),
	}

	loader.RegisterFormat(&JSONFormat{})
	loader.RegisterFormat(&YAMLFormat{})

	return loader

}

func (l *ConfigLoader) RegisterFormat(format ConfigFormat) {
	l.formats = append(l.formats, format)
}

func (l *ConfigLoader) LoadFile(path string) (map[string]interface{}, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	format := l.detectFormat(path)
	if format == nil {
		return nil, fmt.Errorf("unsupported file format: %s", path)
	}

	config, err := format.Unmarshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}
	return config, nil

}

func (l *ConfigLoader) LoadDir(dir string) (map[string]interface{}, error) {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	result := make(map[string]interface{})
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		path := filepath.Join(dir, file.Name())
		config, err := l.LoadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to load %s: %w", path, err)
		}
		result = mergeMaps(result, config)
	}
	return result, nil
}

func (l *ConfigLoader) detectFormat(path string) ConfigFormat {
	ext := strings.ToLower(filepath.Ext(path))
	for _, format := range l.formats {
		for _, formatExt := range format.Extension() {
			if ext == formatExt {
				return format
			}
		}
	}

	return nil
}
