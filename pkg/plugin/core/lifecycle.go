package core

import (
	"bindxdb/pkg/plugin/loader"
	"sync"
	"time"
)

type LifecycleState int

const (
	StateUnloaded LifecycleState = iota
	StateLoaded
	StateInitializing
	StateActive
	StateStopping
	StateError
)

func (s LifecycleState) String() string {
	return [...]string{
		"unloaded",
		"loaded",
		"initializing",
		"active",
		"stopping",
		"error",
	}[s]
}

type LifecycleManager struct {
	mu      sync.RWMutex
	plugins map[string]*pluginInstance
}

type pluginInstance struct {
	plugin     Plugin
	loader     loader.PluginLoader
	metadata   PluginMetadata
	state      LifecycleState
	lastError  error
	startedAt  time.Time
	dependsOn  []string
	dependedBy []string
}

type StateChangeEvent struct {
	PluginID  string
	OldState  LifecycleState
	NewState  LifecycleState
	Timestamp time.Time
	Error     error
}
