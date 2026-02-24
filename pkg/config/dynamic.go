package config

import (
	"context"
	"strings"
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

func NewDynamicConfigManager(manager *ConfigManager) *DynamicConfigManager {
	ctx, cancel := context.WithCancel(context.Background())
	dcm := &DynamicConfigManager{
		manager:     manager,
		updaters:    make(map[string]DynamicUpdater),
		updateQueue: make(chan UpdateRequest, 100),
		ctx:         ctx,
		cancel:      cancel,
	}

	return dcm
}

func (d *DynamicConfigManager) processUpdates() {
	for {
		select {
		case <-d.ctx.Done():
			return
		case request := <-d.updateQueue:
			d.processUpdate(request)
		}
	}
}

func (d *DynamicConfigManager) processUpdate(request UpdateRequest) {
	var response UpdateResponse
	oldValue, err := d.manager.Get(request.Key)
	if err != nil {
		response = UpdateResponse{
			Success: false,
			Error:   err,
		}
		d.sendResponse(request, response)
		return
	}

	var updater DynamicUpdater
	d.mu.RLock()
	for _, u := range d.updaters {
		if u.CanUpdate(request.Key) {
			updater = u
			break
		}
	}
	d.mu.RUnlock()

	if updater == nil {
		if err := d.manager.Set(request.Key, request.Value, request.Source, true); err != nil {
			response = UpdateResponse{
				Success: false,
				Error:   err,
			}
		} else {
			response = UpdateResponse{
				Success:  true,
				OldValue: oldValue,
				NewValue: request.Value,
			}
		}
	} else {
		if err := updater.ApplyUpdate(request.Key, request.Value); err != nil {
			if rollbackErr := updater.RollbackUpdate(request.Key, oldValue); rollbackErr != nil {
				d.manager.logger.Error("failed to rollback update",
					"key", request.Key,
					"error", rollbackErr)
			}

			response = UpdateResponse{
				Success: false,
				Error:   err,
			}
		} else {
			if err := d.manager.Set(request.Key, request.Value, request.Source, true); err != nil {
				d.manager.logger.Error("failed to update stored value after successful apply",
					"key", request.Key,
					"error", err)
			}
			response = UpdateResponse{
				Success:  true,
				OldValue: oldValue,
				NewValue: request.Value,
			}
		}
	}

	d.sendResponse(request, response)
}

func (d *DynamicConfigManager) sendResponse(request UpdateRequest, response UpdateResponse) {
	defer close(request.Response)

	select {
	case request.Response <- response:
	default:
		d.manager.logger.Warn("failed to send update response", "key", request.Key)
	}
}

func (d *DynamicConfigManager) Stop() {
	d.cancel()
}

type ComponentUpdater struct {
	name         string
	keys         []string
	applyFunc    func(key string, value interface{}) error
	rollbackFunc func(key string, oldValue interface{}) error
}

func (c *ComponentUpdater) CanUpdate(key string) bool {
	for _, k := range c.keys {
		if k == key || strings.HasPrefix(key, k+".") {
			return true
		}
	}
	return false
}

func (c *ComponentUpdater) ApplyUpdate(key string, value interface{}) error {
	if c.applyFunc != nil {
		return c.applyFunc(key, value)
	}
	return nil
}

func (c *ComponentUpdater) RollbackUpdate(key string, oldValue interface{}) error {
	if c.rollbackFunc != nil {
		return c.rollbackFunc(key, oldValue)
	}
	return nil
}
