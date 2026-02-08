package plugin

import "context"

type PluginState int

const (
	StateUnknown PluginState = iota
	StateLoaded
	StateInitialized
	StateStarted
	StateStopped
	StateFailed
)

func (s PluginState) String() string {
	return [...]string{
		"Unknown",
		"Loaded",
		"Initialized",
		"Started",
		"Stopped",
		"Failed",
	}[s]
}

type HookType string

const (
	HookPreQuery    HookType = "pre_query"
	HookPostQuery   HookType = "post_query"
	HookPreTx       HookType = "pre_transaction"
	HookPostTx      HookType = "post_transaction"
	HookPreExecute  HookType = "pre_execute"
	HookPostExecute HookType = "post_execute"
	HookShutdown    HookType = "shutdown"
)

type HookContext struct {
	Ctx      context.Context
	PluginID string
	Data     map[string]interface{}
}

type HookHandler func(ctx *HookContext) error

type PluginMetadata struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Version      string                 `json:"version"`
	Description  string                 `json:"description"`
	Author       string                 `json:"author"`
	License      string                 `json:"license"`
	Dependencies []Dependency           `json:"dependencies"`
	Provides     []string               `json:"provides"`
	Requires     []string               `json:"requires"`
	ConfigSchema map[string]interface{} `json:"config_schema"`
}

type Dependency struct {
	PluginID string `json:"plugin_id"`
	Version  string `json:"version"`
	Optional bool   `json:"optional"`
}

type Plugin interface {
	Metadata() PluginMetadata

	Init(ctx context.Context, config map[string]interface{}) error

	Start(ctx context.Context) error

	Stop(ctx context.Context) error

	GetHooks() map[HookType][]HookHandler

	Ready() bool
}
