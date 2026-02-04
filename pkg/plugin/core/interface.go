package core

import (
	"context"
	"time"
)

type PluginType int

const (
	PluginTypeStorage PluginType = iota + 1
	PluginTypeIndex
	PluginTypeAuth
	PluginTypeFunction
	PluginTypeDataType
	PluginTypeAggregate
	PluginTypeHook
	PluginTypeProtocol
)

func (pt PluginType) String() string {
	return [...]string{
		"storage",
		"index",
		"auth",
		"function",
		"datatype",
		"aggregate",
		"hook",
		"protocol",
	}[pt-1]
}

type PluginMetadata struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Type        PluginType `json:"type"`
	Version     string     `json:"version"`
	Description string     `json:"description"`
	Author      string     `json:"author"`
	License     string     `json:"license"`
	Repository  string     `json:"repository"`

	Dependencies []string `json:"dependencies"`
	Exports      []string `json:"exports"`
	Capabilities []string `json:"capabilities"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Dependency struct {
	PluginID string `json:"plugin_id"`
	Version  string `json:"version"`
	Optional bool   `json:"optional"`
}

type PluginContext struct {
	Context     context.Context
	Logger      Logger
	Config      map[string]interface{}
	DatabaseAPI DatabaseAPI
	EventBus    EventBus
	Metrics     MetricsCollector
}

type Logger interface {
}

type EventBus interface {
	Publish(event string, data interface{}) error
}

type MetricsCollector interface {
}

type DatabaseAPI interface {
	ExecuteQuery(ctx context.Context, query string, params ...interface{}) (ResultSet, error)
	GetTableInfo(ctx context.Context, tableName string) (*TableInfo, error)
	RegisterFunction(name string, fn Function) error
	RegisterType(name string, typ DataType) error
}

type ResultSet interface {
}

type Function interface {
	Execute(ctx context.Context, args []interface{}) (interface{}, error)
}

type DataType interface {
}

type TableInfo struct {
}

type Plugin interface {
	Metadata() PluginMetadata
	Initialize(ctx *PluginContext) error
	Shutdown() error
	HealthCheck() (*HealthStatus, error)
}

type HealthStatus struct {
	Status  HealthStatusCode `json:"status"`
	Message string           `json:"message"`
}

type HealthStatusCode int

const (
	HealthStatusHealthy HealthStatusCode = iota
	HealthStatusUnhealthy
	HealthStatusDegraded
)

type StoragePlugin interface {
	Plugin

	CreateTable(ctx context.Context, def *TableDefinition) error
	DropTable(ctx context.Context, tableName string) error
	Scan(ctx context.Context, tableName string, filter Filter) (Iterator, error)
	Insert(ctx context.Context, tableName string, rows []Row) error
	Update(ctx context.Context, tableName string, filter Filter, update map[string]interface{}) (int64, error)
	Delete(ctx context.Context, tableName string, filter Filter) (int64, error)
	BeginTransaction(ctx context.Context, opts *TxOptions) (Transaction, error)

	CreateIndex(ctx context.Context, index *IndexDefinition) error
	DropIndex(ctx context.Context, indexName string) error
}

type Iterator interface {
	Next() (Row, error)
	Close() error
}

type Row interface {
	Values() []interface{}
}

type Filter interface {
}

type TxOptions struct {
	IsolationLevel string `json:"isolation_level"`
	ReadOnly       bool   `json:"read_only"`
}

type Transaction interface {
	Commit() error
	Rollback() error
}

type TableDefinition struct {
	Name    string             `json:"name"`
	Columns []ColumnDefinition `json:"columns"`
}

type ColumnDefinition struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type IndexDefinition struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type FunctionPlugin interface {
	Plugin

	RegisterFunction(registry FunctionRegistry) error
}

type FunctionRegistry interface {
	RegisterFunction(function Function) error
}

type HookPlugin interface {
	Plugin

	RegisterHook(registry HookRegistry) error
}

type HookRegistry interface {
	RegisterHook(hook Hook) error
}

type Hook interface {
	// Type() HookType
}

type IndexPlugin interface {
	Plugin

	Create(def *IndexDefinition) (Index, error)
	SupportType(dataType string) bool
	Statistics() *IndexStats
}

type IndexType string

const (
	IndexTypeBTree IndexType = "btree"
	IndexTypeHash  IndexType = "hash"
	IndexTypeGin   IndexType = "gin"
	IndexTypeGist  IndexType = "gist"
)

type Index interface {
	Type() IndexType
}

type IndexStats struct {
}
