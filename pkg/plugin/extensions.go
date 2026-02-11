package plugin

import (
	"context"
	"io"
)

type RecordID uint64

type PageID uint64

type StorageEngine interface {
	Plugin
	CreateTable(name string, schema *TableSchema) error
	DropTable(name string) error
	TruncateTable(name string) error

	AlterTables(name string, changes []TableChange) error
	ListTables() ([]string, error)

	Insert(table string, record map[string]interface{}) (RecordID, error)
	Update(table string, id RecordID, updates map[string]interface{}) error
	Delete(table string, id RecordID) error
	Get(table string, id RecordID) (map[string]interface{}, error)

	Scan(table string, filter Filter) (Iterator, error)
	ScanRange(table string, id RecordID) (map[string]interface{}, error)

	BeginTransaction(readOnly bool) (Transaction, error)

	TableStats(name string) (*TableStats, error)

	Vacuum(table string) error
	Analyze(table string) error
	CheckIntegrity(table string) (bool, []string, error)
}

type TableChange struct {
	Type    TableChangeType
	Column  *ColumnDef
	OldName string
	NewName string
}

type TableChangeType int

const (
	TableChangeAddColumn TableChangeType = iota
	TableChangeDropColumn
	TableChangeModifyColumn
	TableChangeRenameColumn
	TableChangeAddConstraint
	TableChangeDropConstraint
	TableChangeRenameTable
)

type IndexPlugin interface {
	Plugin
	CreateIndex(name string, table string, columns []string, config map[string]interface{}) error

	DropIndex(name string) error

	Lookup(indexName string, key interface{}) ([]RecordID, error)

	RangeScan(indexName string, start, end interface{}) ([]RecordID, error)

	Rebuild(indexName string) error
	Statistics(indexName string) (*IndexStats, error)
}

type AuthPlugin interface {
	Plugin

	Authenticate(ctx context.Context, credentials map[string]string) (*AuthResult, error)

	ValidateToken(ctx context.Context, token string) (*AuthResult, error)

	RefreshToken(ctx context.Context, token string) (*AuthResult, error)

	Authorize(ctx context.Context, subject *Subject, resource string, action string) (bool, error)

	CreateUser(ctx context.Context, user *User) error

	DeleteUser(ctx context.Context, username string) error
}

type TableSchema struct {
	Name    string
	Columns []ColumnDef
	Indexes []IndexDef
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
	Evaluate(record map[string]interface{}) (bool, error)
	GetUsedColumns() []string
	String() string
}

type FunctionPlugin interface {
	Plugin

	GetFunctions() []FunctionDef

	ExecuteFunction(ctx *FunctionContext, args []interface{}) (interface{}, error)

	CreateAggregateState() interface{}

	AggregateStep(state interface{}, value interface{}) error

	AggregateFinal(state interface{}) (interface{}, error)
}

type FunctionContext struct {
	Context   context.Context
	Session   *Session
	Arguments []interface{}
	Options   map[string]interface{}
}

type Session struct {
	ID         string
	UserID     string
	CreatedAt  int64
	LastActive int64
	ClientAddr string
	ClientInfo string
	Variables  map[string]interface{}
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

type IndexDef struct {
	Name    string
	Columns []string
	Unique  bool
	Primary bool
	Type    IndexType
	Options map[string]interface{}
}

type IndexType int

const (
	IndexTypeBTree IndexType = iota
	IndexTypeHash
	IndexTypeRTree
	IndexTypeFullText
	IndexTypeBitmap
)

type ConstraintDef struct {
	Name       string
	Type       ConstraintType
	Columns    []string
	CheckExpr  string
	References *ForeignKeyDef
}

type ConstraintType int

type ForeignKeyDef struct {
	Table    string
	Columns  []string
	OnDelete ForeignKeyAction
	OnUpdate ForeignKeyAction
}

type ForeignKeyAction int

const (
	ForeignKeyNoAction ForeignKeyAction = iota
	ForeignKeyRestrict
	ForeignKeyCascade
	ForeignKeySetNull
	ForeignKeySetDefault
)

type CheckConstraint struct {
	Expression string
}

type BasicFilter struct {
	Column   string
	Value    interface{}
	Operator FilterOperator
}

type FilterOperator int

type QueryPlan struct {
	Root          *PlanNode
	Cost          float64
	EstimatedRows int64
	Operations    []string
}

type PlanNode struct {
	Type     PlanNodeType
	Children []*PlanNode
	Data     map[string]interface{}
}

type PlanNodeType int

const (
	PlanNodeScan PlanNodeType = iota
	PlanNodeFilter
	PlanNodeProject
	PlanNodeJoin
	PlanNodeAggregate
	PlanNodeSort
	PlanNodeLimit
	PlanNodeUnion
)

type BackupPlugin interface {
	Plugin
	Backup(ctx context.Context, config map[string]interface{},
		writer io.Writer) error

	Restore(ctx context.Context, config map[string]interface{},
		reader io.Reader) error

	VerifyBackup(ctx context.Context, config map[string]interface{}) (bool, error)

	CreateCheckpoint() (string, error)

	IncrementalBackup(ctx context.Context, checkpoint string, writer io.Writer) error
}

type ReplicationPlugin interface {
	StartReplication(masterConfig map[string]interface{}) error
	StopReplication() error
	GetReplicationStatus() (ReplicationStatus, error)
	AddReplica(slaveConfig map[string]interface{}) error
	RemoveReplica(slaveID string) error
	GetReplicas() ([]ReplicaInfo, error)
}

type ReplicationStatus struct {
	Running     bool
	Lag         int64
	LastApplied int64
	Errors      []string
}

type ReplicaInfo struct {
	ID        string
	Address   string
	Lag       int64
	Status    string
	Connected bool
}

type MonitoringPlugin interface {
	Plugin

	CollectMetrics() (map[string]interface{}, error)

	HealthCheck() (bool, map[string]interface{}, error)

	SetAlert(condition string, action string) error

	RemoveAlert(id string) error

	ListAlerts() ([]Alert, error)
}

type PerformanceMetric struct {
	Name      string
	Value     float64
	Timestamp int64
	Tags      map[string]string
}

type Alert struct {
	ID        string
	Condition string
	Action    string
	Status    string
	CreatedAt int64
	Triggered int64
}

type EncryptionPlugin interface {
	Plugin
	EncryptData(data []byte, keyID string) ([]byte, error)
	DecryptData(data []byte, keyID string) ([]byte, error)
	GenerateKey(keyID string) error
	RotateKey(keyID string) error
	DeleteKey(keyID string) error
	ListKeys() ([]string, error)

	EncryptFile(path string, keyID string) error
	DecryptFile(path string, keyID string) error
}

const (
	OperatorEqual FilterOperator = iota
	OperatorNotEqual
	OperatorGreaterThen
	OperatorGreaterThenOrEqual
	OperatorLessThen
	OperatorLessThenOrEqual
	OperatorLike
	OperatorIn
	OperatorIsNull
	OperatorIsNotNull
	OperatorBetween
)

func (f *BasicFilter) Evaluate(record map[string]interface{}) (bool, error) {
	value, exists := record[f.Column]

	if !exists {
		return false, nil
	}

	switch f.Operator {
	case OperatorEqual:
		return value == f.Value, nil
	case OperatorNotEqual:
		return value != f.Value, nil
	case OperatorGreaterThen:
		return false, nil
	default:
		return false, nil
	}
}

func (f *BasicFilter) GetUsedColumns() []string {
	return []string{f.Column}
}

func (f *BasicFilter) String() string {
	return f.Column + " = ?"
}

type CompositeFilter struct {
	Filters []Filter
	And     bool
}

func (cf *CompositeFilter) Evaluate(record map[string]interface{}) (bool, error) {
	if len(cf.Filters) == 0 {
		return true, nil
	}

	for _, filter := range cf.Filters {
		match, err := filter.Evaluate(record)
		if err != nil {
			return false, err
		}
		if cf.And && !match {
			return false, nil
		}
		if !cf.And && match {
			return true, nil
		}
	}
	return cf.And, nil
}

func (cf *CompositeFilter) GetUsedColumns() []string {
	columns := make([]string, 0)
	seen := make(map[string]bool)

	for _, filter := range cf.Filters {
		for _, col := range filter.GetUsedColumns() {
			if !seen[col] {
				columns = append(columns, col)
				seen[col] = true
			}
		}
	}
	return columns

}

func (cf *CompositeFilter) String() string {
	op := " OR "
	if cf.And {
		op = " AND "
	}

	result := ""

	for i, filter := range cf.Filters {
		if i > 0 {
			result += op
		}
		result += filter.String()
	}
	return result
}

type Transaction interface {
	Commit() error
	Rollback() error
	ID() uint64

	Status() TransactionStatus

	IsReadOnly() bool

	Savepoint(name string) error

	RollbackTo(name string) error

	ReleaseSavepoint(name string) error
}

type TransactionStatus int

const (
	TransactionActive TransactionStatus = iota
	TransactionCommitted
	TransactionRolledBack
	TransactionFailed
)

type TableStats struct {
	RowCount     int64
	DataSize     int64
	IndexSize    int64
	AvgRowSize   float64
	LastAnalyzed int64
}

type IndexStats struct {
	Name         string
	Size         int64
	UniqueValues int64
	Height       int
	LeafPages    int64
	LasrRebuilt  int64
}

type Subject struct {
	ID          string
	Username    string
	Email       string
	Groups      []string
	Roles       []string
	Permissions []string
	Attributes  map[string]interface{}
}

type User struct {
	Username   string
	Password   string //hashed
	Email      string
	FullName   string
	IsActive   bool
	CreatedAt  int64
	LastLogin  int64
	Attributes map[string]interface{}
}
