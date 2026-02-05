package hooks

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

type HookType string

const (
	//Database lifecycle hooks
	HookDatabaseStart HookType = "database.start"
	HookDatabaseStop  HookType = "database.stop"

	//Connection hooks
	HookConnectionOpen  HookType = "connection.open"
	HookConnectionClose HookType = "connection.close"

	//Transaction hooks
	HookTransactionBegin    HookType = "transaction.begin"
	HookTransactionCommit   HookType = "transaction.commit"
	HookTransactionRollback HookType = "transaction.rollback"

	//Query execution hooks
	HookQueryExecute HookType = "query.execute"
	HookQueryParse   HookType = "query.parse"
	HookQueryPlan    HookType = "query.plan"
	HookQueryResult  HookType = "query.result"

	//Schema hooks
	HookTableCreate HookType = "table.create"
	HookTableDrop   HookType = "table.drop"
	HookIndexCreate HookType = "index.create"
	HookIndexDrop   HookType = "index.drop"

	//Data modification hooks
	HookRowInsert HookType = "row.insert"
	HookRowUpdate HookType = "row.update"
	HookRowDelete HookType = "row.delete"

	//Authentication hooks
	HookAuthAttempt HookType = "auth.attempt"
	HookAuthSuccess HookType = "auth.success"
	HookAuthFailure HookType = "auth.failure"

	//Plugin hooks
	HookPluginLoad   HookType = "plugin.load"
	HookPluginUnload HookType = "plugin.unload"
)

type HookPriority int

const (
	PriorityFirst  HookPriority = 100
	PriorityHigh   HookPriority = 75
	PriorityNormal HookPriority = 50
	PriorityLow    HookPriority = 25
	PriorityLast   HookPriority = 0
)

// HookHandler is a function that handles a hook
type HookHandler func(ctx *HookContext) error

type HookContext struct {
	Context   context.Context
	HookType  HookType
	Timestamp int64
	PluginID  string

	//Data associated with the hook
	Data map[string]interface{}

	CanModify bool
	Modified  bool

	//Error associated with the hook
	Error     error
	StopChain bool
}

// HookRegistration is a struct that represents a registered hook
type HookRegistration struct {
	ID       string
	PluginID string
	HookType HookType
	Handler  HookHandler
	Priority HookPriority
	Enabled  bool
}

type HookRegistry struct {
	mu       sync.RWMutex
	hooks    map[HookType][]*HookRegistration
	byPlugin map[string][]*HookRegistration

	//Execution statistics
	stats map[string]*HookStats
}

type HookStats struct {
	TotalCalls    int64
	TotalErrors   int64
	TotalDuration int64
	LastCall      int64
}

// NewHookRegistry creates a new hook registry
func NewHookRegistry() *HookRegistry {
	return &HookRegistry{
		hooks:    make(map[HookType][]*HookRegistration),
		byPlugin: make(map[string][]*HookRegistration),
		stats:    make(map[string]*HookStats),
	}
}

func (r *HookRegistry) RegisterHook(
	pluginID string,
	hookType HookType,
	handler HookHandler,
	priority HookPriority,
) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	hookID := fmt.Sprintf("%s:%s:%d", pluginID, hookType, time.Now().UnixNano())
	registration := &HookRegistration{
		ID:       hookID,
		PluginID: pluginID,
		HookType: hookType,
		Handler:  handler,
		Priority: priority,
		Enabled:  true,
	}

	r.hooks[hookType] = append(r.hooks[hookType], registration)

	sort.Slice(r.hooks[hookType], func(i, j int) bool {
		return r.hooks[hookType][i].Priority > r.hooks[hookType][j].Priority
	})

	r.byPlugin[pluginID] = append(r.byPlugin[pluginID], registration)

	r.stats[hookID] = &HookStats{}

	return hookID, nil

}

func (r *HookRegistry) UnregisterHook(hookID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	//Find the registration
	var registration *HookRegistration
	// var hookType HookType
	var pluginID string

	for ht, registrations := range r.hooks {
		for i, reg := range registrations {
			if reg.ID == hookID {
				registration = reg
				// hookType = ht

				r.hooks[ht] = append(registrations[:i], registrations[i+1:]...)
				break

			}
		}

		if registration != nil {
			break
		}
	}
	if registration == nil {
		return fmt.Errorf("hook %s not found", hookID)
	}

	pluginID = registration.PluginID

	if pluginRegs, exists := r.byPlugin[pluginID]; exists {
		for i, reg := range pluginRegs {
			if reg.ID == hookID {
				r.byPlugin[pluginID] = append(pluginRegs[:i], pluginRegs[i+1:]...)
				break
			}
		}

		if len(r.byPlugin[pluginID]) == 0 {
			delete(r.byPlugin, pluginID)
		}
	}

	delete(r.stats, hookID)

	return nil
}
