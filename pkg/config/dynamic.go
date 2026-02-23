package config

import (
	"context"
	"sync"
	"time"
)

type DynamicUpdater interface {
	CanUpdate(key string) bool
	ApplyUpdate(key string, value interface{}) error

	RollbackUpdate(key string, oldValue interface{}) error
}

type DynamicConfigManager struct {
	manager     *ConfigManager
	updaters    map[string]DynamicUpdater
	mu          sync.RWMutex
	updateQueue chan UpdateRequest
	ctx         context.Context
	cancel      context.CancelFunc
}

type UpdateRequest struct {
	Key      string
	Value    interface{}
	Source   ConfigSource
	Response chan UpdateResponse

	Timeout time.Duration
}

type UpdateResponse struct {
	Success  bool
	Error    error
	OldValue interface{}
	NewValue interface{}
}
