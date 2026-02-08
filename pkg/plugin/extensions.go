package plugin

type StorageEngine interface {
	Plugin
	CreateTable(name string, schema *TableSchema) error
}

type TableSchema struct {
}

type ColumnDef struct {
	Name     string
	Type     string
	Nullable bool
	Default  interface{}
}

type Iterator interface {
	Next() bool
	Value() map[string]interface{}
	Error() error
	Close() error
}

type Filter interface {
	Evaluate(record map[string]interface{}) bool
}

type AuthResult struct {
	Authenticated bool
	UserID        string
	Roles         []string
	Permissions   []string
	Token         string
	ExpiresAt     int64
}

type FunctionDef struct {
	Name       string
	Arguments  []ArgumentDef
	ReturnType string
	Volatile   bool
}

type ArgumentDef struct {
	Name     string
	Type     string
	Optional bool
	Default  interface{}
}
