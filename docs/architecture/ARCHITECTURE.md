# BindXDB Architecture

**Version:** 1.0  
**Last Updated:** 2026-02-03  
**Status:** Living Document

---

## Table of Contents

1. [Overview](#overview)
2. [Design Principles](#design-principles)
3. [System Architecture](#system-architecture)
4. [Component Details](#component-details)
5. [Data Flow](#data-flow)
6. [Concurrency Model](#concurrency-model)
7. [Storage Architecture](#storage-architecture)
8. [Transaction Management](#transaction-management)
9. [Query Processing](#query-processing)
10. [Plugin System](#plugin-system)
11. [Network Layer](#network-layer)
12. [Performance Optimizations](#performance-optimizations)
13. [Security Architecture](#security-architecture)

---

## Overview

BindXDB is a **microkernel-based** relational database management system built in pure Go. It follows a layered architecture with clear separation of concerns and pluggable components.

### Key Characteristics

- **Language**: 100% Go (no C dependencies)
- **Concurrency**: Lock-free hot paths, fine-grained locking
- **Storage**: Page-based with buffer pool caching
- **Transactions**: MVCC with snapshot isolation
- **Query Execution**: Volcano iterator model
- **Extensibility**: Dynamic plugin loading
- **Observability**: Built-in metrics, tracing, and profiling

---

## Design Principles

### 1. Microkernel Architecture
Core engine provides minimal functionality; extensions add features through plugins.

**Benefits:**
- Modularity and maintainability
- Easy to test individual components
- Plugins can be developed independently
- Reduced core complexity

### 2. Zero-Copy Where Possible
Minimize memory allocations and copies in hot paths.

**Techniques:**
- Reference counting for shared pages
- Buffer reuse through sync.Pool
- Direct I/O for large sequential reads

### 3. Lock-Free Hot Paths
Use atomic operations and lock-free data structures for frequently accessed paths.

**Examples:**
- Transaction ID allocation
- Page pin/unpin counters
- Statistics collection

### 4. Performance by Default
Fast path should be the default; slow path for edge cases.

**Strategy:**
- Optimize common case (cache hits, sequential access)
- Inline small, hot functions
- Use profiling to identify bottlenecks

### 5. Observable & Debuggable
Every component emits metrics and supports tracing.

**Implementation:**
- Prometheus metrics for quantitative data
- OpenTelemetry spans for request tracing
- Structured logging with levels
- pprof endpoints for profiling

---

## System Architecture

### High-Level Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                         Client Layer                            │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐       │
│  │HTTP/REST │  │   gRPC   │  │WebSocket │  │PostgreSQL│       │
│  │  :8080   │  │  :9090   │  │  :8081   │  │  :5432   │       │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘       │
└─────────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────────┐
│                    Authentication Layer                         │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐                     │
│  │   JWT    │  │  OAuth2  │  │   mTLS   │                     │
│  └──────────┘  └──────────┘  └──────────┘                     │
└─────────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────────┐
│                      SQL Processing Layer                       │
│  ┌──────────┐      ┌──────────┐      ┌──────────┐             │
│  │  Parser  │  →   │ Optimizer│  →   │ Executor │             │
│  │  (AST)   │      │ (Plan)   │      │(Iterator)│             │
│  └──────────┘      └──────────┘      └──────────┘             │
└─────────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────────┐
│                   Transaction Manager                           │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐            │
│  │    MVCC     │  │Lock Manager │  │  WAL Writer │            │
│  │(Visibility) │  │ (Deadlock)  │  │  (Durability│            │
│  └─────────────┘  └─────────────┘  └─────────────┘            │
└─────────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────────┐
│                      Index Layer                                │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐            │
│  │   B+Tree    │  │  Hash Index │  │  Full-Text  │            │
│  │  (Primary)  │  │  (Plugin)   │  │   (Plugin)  │            │
│  └─────────────┘  └─────────────┘  └─────────────┘            │
└─────────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────────┐
│                      Storage Layer                              │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐            │
│  │Buffer Pool  │  │  Heap File  │  │ File I/O    │            │
│  │  (Cache)    │  │  (Pages)    │  │  Manager    │            │
│  └─────────────┘  └─────────────┘  └─────────────┘            │
└─────────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────────┐
│                      Plugin System                              │
│  Custom Storage Engines │ UDFs │ Auth Providers │ Indexes      │
└─────────────────────────────────────────────────────────────────┘
```

---

## Component Details

### 1. Protocol Layer

**Responsibility:** Handle client connections and protocol translation

**Components:**
- **HTTP Server**: RESTful API with JSON encoding
- **gRPC Server**: Protocol buffer-based RPC
- **WebSocket Server**: Real-time bidirectional communication
- **PostgreSQL Wire Protocol**: Compatibility with psql clients

**Key Features:**
- Connection pooling and multiplexing
- TLS termination
- Rate limiting and throttling
- Protocol negotiation

**Interface:**
```go
type ProtocolHandler interface {
    Start(ctx context.Context, addr string) error
    Stop(ctx context.Context) error
    HandleQuery(req *QueryRequest) (*QueryResponse, error)
}
```

---

### 2. SQL Processing Layer

#### 2.1 Parser

**Responsibility:** Convert SQL text to Abstract Syntax Tree (AST)

**Implementation:**
- Hand-written recursive descent parser
- Support for SQL-92 core + extensions
- Context-free grammar with lookahead

**Example AST:**
```go
type SelectStmt struct {
    SelectList []SelectItem
    From       *FromClause
    Where      *WhereClause
    GroupBy    []Expr
    OrderBy    []OrderByItem
    Limit      *int64
}
```

#### 2.2 Optimizer

**Responsibility:** Generate efficient execution plan

**Strategies:**
1. **Rule-Based Optimization**
   - Predicate pushdown
   - Projection pruning
   - Constant folding
   - Join reordering

2. **Cost-Based Optimization**
   - Cardinality estimation
   - Selectivity calculation
   - Access path selection (index vs. scan)
   - Join algorithm selection

**Cost Model:**
```go
type Cost struct {
    CPUCost   float64  // CPU cycles
    IOCost    float64  // Disk I/O operations
    NetCost   float64  // Network bandwidth
    TotalCost float64  // Weighted sum
}
```

#### 2.3 Executor

**Responsibility:** Execute query plan and return results

**Model:** Volcano Iterator (pull-based)

**Operators:**
- **Scan**: Sequential/Index scan with predicate filtering
- **Project**: Column projection and expression evaluation
- **Filter**: Tuple filtering
- **Join**: Hash join, merge join, nested loop join
- **Aggregate**: Grouping and aggregation functions
- **Sort**: External sort for large datasets

**Interface:**
```go
type Operator interface {
    Open() error
    Next() (*Tuple, error)
    Close() error
}
```

---

### 3. Transaction Manager

#### 3.1 MVCC (Multi-Version Concurrency Control)

**Responsibility:** Allow concurrent transactions without blocking

**Design:**
- Each tuple has `xmin` (creator transaction ID) and `xmax` (deleter transaction ID)
- Snapshot isolation: Transaction sees consistent snapshot at start
- Version chains: Old tuple versions linked via pointers

**Visibility Rules:**
```go
func IsVisible(tuple *Tuple, snapshot Snapshot) bool {
    // Tuple created after snapshot
    if tuple.Xmin > snapshot.XID {
        return false
    }
    
    // Tuple deleted before snapshot
    if tuple.Xmax != InvalidTxnID && tuple.Xmax < snapshot.XID {
        return false
    }
    
    // Tuple in progress by another transaction
    if IsInProgress(tuple.Xmin, snapshot) || IsInProgress(tuple.Xmax, snapshot) {
        return false
    }
    
    return true
}
```

#### 3.2 Lock Manager

**Responsibility:** Prevent conflicts and detect deadlocks

**Lock Modes:**
- **S** (Shared): Read lock
- **X** (Exclusive): Write lock
- **IS** (Intent Shared): Intent to acquire S locks on children
- **IX** (Intent Exclusive): Intent to acquire X locks on children
- **SIX** (Shared Intent Exclusive): S + IX

**Deadlock Detection:**
- Wait-for graph construction
- Cycle detection using DFS
- Youngest transaction aborted on deadlock

#### 3.3 Write-Ahead Logging (WAL)

**Responsibility:** Ensure durability and enable crash recovery

**Log Record Format:**
```go
type LogRecord struct {
    LSN       uint64      // Log Sequence Number
    PrevLSN   uint64      // Previous LSN for this transaction
    TxnID     TxnID       // Transaction ID
    Type      RecordType  // INSERT, UPDATE, DELETE, COMMIT, ABORT
    PageID    PageID      // Affected page
    Offset    uint16      // Offset within page
    Length    uint16      // Data length
    OldData   []byte      // Before image (UNDO)
    NewData   []byte      // After image (REDO)
}
```

**Recovery Algorithm (ARIES):**
1. **Analysis**: Scan log to determine dirty pages and active transactions
2. **REDO**: Replay all logged operations from checkpoint
3. **UNDO**: Roll back uncommitted transactions

---

### 4. Storage Layer

#### 4.1 Page Format

**Standard Page (8KB):**
```
┌─────────────────────────────────────────┐
│        Page Header (24 bytes)           │
├─────────────────────────────────────────┤
│        Slot Array (2 bytes each)        │
│               ↓ grows                   │
├─────────────────────────────────────────┤
│           Free Space                    │
├─────────────────────────────────────────┤
│               ↑ grows                   │
│        Tuple Data (var length)          │
└─────────────────────────────────────────┘
```

**Page Header:**
```go
type PageHeader struct {
    PageID      uint32    // Page identifier
    LSN         uint64    // Last modification LSN
    Checksum    uint32    // CRC32 checksum
    Flags       uint16    // Flags (dirty, etc.)
    SlotCount   uint16    // Number of slots
    FreeStart   uint16    // Start of free space
    FreeEnd     uint16    // End of free space
}
```

#### 4.2 Buffer Pool

**Responsibility:** Cache frequently accessed pages in memory

**Design:**
- **Eviction Policy**: Clock-Sweep (approximation of LRU)
- **Page Replacement**: Second-chance algorithm
- **Pin/Unpin**: Reference counting to prevent eviction

**Structure:**
```go
type BufferPool struct {
    frames    []*Frame      // Fixed-size array of frames
    pageTable map[PageID]FrameID  // Page → Frame mapping
    freeList  *list.List    // Free frames
    clockHand int            // Clock sweep pointer
    mu        sync.RWMutex   // Protects pool structures
}

type Frame struct {
    page      *Page
    pinCount  atomic.Int32   // Number of active users
    dirty     atomic.Bool    // Modified since load
    refBit    atomic.Bool    // Referenced recently
}
```

#### 4.3 B+Tree Index

**Responsibility:** Fast key-based access and range scans

**Node Format:**
- **Internal Node**: Keys + child pointers
- **Leaf Node**: Keys + tuple pointers + sibling links

**Concurrency Control:**
- **Latch Crabbing**: Acquire latches top-down, release as soon as safe
- **Optimistic Locking**: Assume no conflicts for reads

**Operations:**
```go
type BPlusTree interface {
    Insert(key []byte, value uint64) error
    Delete(key []byte) error
    Search(key []byte) (uint64, bool)
    RangeScan(startKey, endKey []byte) Iterator
}
```

---

### 5. Plugin System

**Responsibility:** Dynamic extension loading and management

**Architecture:**
```go
type Plugin interface {
    Name() string
    Version() string
    Init(ctx context.Context, config Config) error
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
}

type PluginRegistry struct {
    plugins map[string]Plugin
    hooks   map[HookType][]HookFunc
    mu      sync.RWMutex
}
```

**Extension Points:**
1. **Storage Engines**: Custom storage backends
2. **Index Types**: Specialized indexes (GiST, R-Tree, etc.)
3. **Functions**: User-defined functions (UDFs)
4. **Auth Providers**: Custom authentication mechanisms
5. **Query Hooks**: Intercept query lifecycle

**Example Plugin:**
```go
package main

import "github.com/bindxdb/bindxdb/pkg/plugin"

type MyStorageEngine struct{}

func (m *MyStorageEngine) Name() string { return "my_storage" }
func (m *MyStorageEngine) Init(ctx context.Context, cfg Config) error {
    // Initialize storage engine
    return nil
}

// Export plugin
var Plugin plugin.Plugin = &MyStorageEngine{}
```

---

## Data Flow

### Query Execution Flow

```
Client Request
    ↓
[Protocol Handler] Parse request format
    ↓
[Auth Layer] Validate credentials & permissions
    ↓
[SQL Parser] SQL → AST
    ↓
[Validator] Semantic validation & type checking
    ↓
[Optimizer] AST → Logical Plan → Physical Plan
    ↓
[Transaction Manager] Begin transaction, acquire snapshot
    ↓
[Executor] Execute operators (Scan, Join, etc.)
    │
    ├→ [Index Layer] B+Tree lookup
    │      ↓
    ├→ [Buffer Pool] Fetch pages
    │      ↓
    └→ [File I/O] Read from disk (if cache miss)
    ↓
[WAL] Write log records for modifications
    ↓
[Transaction Manager] Commit/Abort transaction
    ↓
[Protocol Handler] Format and send response
    ↓
Client Response
```

---

## Concurrency Model

### Goroutine Usage

1. **Connection Handler**: One goroutine per client connection
2. **WAL Writer**: Background goroutine for log flushing
3. **Checkpoint**: Periodic background checkpoints
4. **VACUUM**: Garbage collection of old tuple versions
5. **Statistics Collector**: Background stats aggregation

### Synchronization Primitives

| Primitive | Use Case |
|-----------|----------|
| `sync.RWMutex` | Buffer pool page table, catalog |
| `sync.Mutex` | Transaction state, lock table |
| `atomic.*` | Counters, flags, pin counts |
| `chan` | Task queues, shutdown signals |
| Lock-free structures | Transaction ID allocation |

---

## Performance Optimizations

### 1. Hot Path Optimizations
- Inline frequently called functions
- Avoid allocations in loops
- Use `sync.Pool` for temporary objects
- Batch I/O operations

### 2. Memory Management
- Object pooling for tuples and pages
- Fixed-size allocations where possible
- Copy-on-write for large structures

### 3. I/O Optimizations
- Sequential prefetching for scans
- Group commit for WAL writes
- Direct I/O for large transfers
- Asynchronous I/O with io_uring (future)

### 4. CPU Optimizations
- Vectorized execution (SIMD)
- Parallel query execution
- JIT compilation for expressions (future)

---

## Security Architecture

### Defense in Depth

1. **Network Layer**: TLS 1.3, mTLS, rate limiting
2. **Authentication**: Multi-factor, token expiration
3. **Authorization**: RBAC, row-level security
4. **Data Protection**: Encryption at rest, key rotation
5. **Audit**: Complete audit trail
6. **Isolation**: Plugin sandboxing

---

## Future Enhancements

- **Distributed Transactions**: 2PC/3PC for multi-node
- **Parallel Query**: Multi-threaded execution
- **Columnar Storage**: For analytical workloads
- **Vector Database**: Similarity search support
- **Time-Series**: Optimized time-series storage
- **Graph Extensions**: Property graph model

---

**Document Owners:** Architecture Team  
**Review Cycle:** Quarterly  
**Feedback:** [architecture@bindxdb.io](mailto:architecture@bindxdb.io)
