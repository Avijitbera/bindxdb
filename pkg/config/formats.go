package config

import (
	"encoding/json"
	"fmt"
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

type TOMLFormat struct{}

type ConfigLoader struct {
	formats []ConfigFormat
}
