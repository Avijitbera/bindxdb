package core

import "sync"

type Registry struct {
	mu      sync.RWMutex
	plugins map[string]Plugin
	byType  map[PluginType]map[string]Plugin

	storageEngines map[string]StoragePlugin

	indexType map[string]IndexPlugin

	// authProviders map[string]AuthPlugin

	functions map[string]Function
}

type ServiceDescriptor struct {
	Name        string
	PluginID    string
	Service     interface{}
	Description string
	Metadata    map[string]interface{}
}
