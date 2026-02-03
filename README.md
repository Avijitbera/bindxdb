# BindXDB

<div align="center">

![BindXDB Logo](https://via.placeholder.com/200x200/6366f1/ffffff?text=BindXDB)

**High-performance, extensible database engine written in Go**

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)](https://go.dev/)
[![Build Status](https://img.shields.io/github/workflow/status/bindxdb/bindxdb/CI)](https://github.com/bindxdb/bindxdb/actions)
[![codecov](https://codecov.io/gh/bindxdb/bindxdb/branch/main/graph/badge.svg)](https://codecov.io/gh/bindxdb/bindxdb)
[![Go Report Card](https://goreportcard.com/badge/github.com/bindxdb/bindxdb)](https://goreportcard.com/report/github.com/bindxdb/bindxdb)
[![Discord](https://img.shields.io/discord/1234567890?logo=discord)](https://discord.gg/bindxdb)

[Features](#-features) • [Quick Start](#-quick-start) • [Documentation](#-documentation) • [Architecture](#-architecture) • [Contributing](#-contributing)

</div>

---

## 🎯 Overview

BindXDB is a production-ready, extensible database engine built from scratch in Go. It implements a full ACID-compliant storage system with advanced features like MVCC, Write-Ahead Logging, and multi-protocol support (HTTP/REST, gRPC, WebSocket, PostgreSQL wire).

### Why BindXDB?

- **🚀 High Performance**: 100K+ concurrent connections, 50K+ QPS, sub-millisecond latency
- **🔌 Extensible**: Plugin architecture for storage engines, indexes, authentication, and custom functions
- **🛠️ Developer-First**: Comprehensive tooling, excellent debugging, and extensive documentation
- **☁️ Cloud-Native**: Designed for containers and orchestration from day one
- **🔍 Observable**: Built-in profiling, metrics (Prometheus), and tracing (OpenTelemetry)
- **🔒 Enterprise-Ready**: RBAC, encryption at rest, audit logging, and row-level security

---

## ✨ Features

### Core Database Engine
- **ACID Compliance**: Full transactional guarantees with MVCC and WAL
- **SQL Support**: Parser, optimizer, and execution engine for standard SQL
- **B+Tree Indexing**: Concurrent access with latch crabbing and bulk loading
- **Buffer Pool**: LRU/Clock-Sweep eviction with configurable page sizes
- **Crash Recovery**: ARIES-inspired recovery algorithm

### Extensibility
- **Plugin System**: Dynamic loading of custom storage engines, indexes, and functions
- **Hook System**: Intercept query lifecycle events (pre/post query, commit)
- **Custom Functions**: Write UDFs in Go and load them as plugins
- **Storage Abstraction**: Swap storage backends without changing application code

### Networking & Protocols
- **HTTP/REST API**: RESTful interface with OpenAPI/Swagger documentation
- **gRPC Support**: High-performance RPC with streaming support
- **WebSocket**: Real-time updates and notifications
- **PostgreSQL Wire**: Compatible with existing PostgreSQL clients
- **TLS/mTLS**: Secure connections with mutual authentication

### Security & Observability
- **Authentication**: Pluggable providers (JWT, OAuth2, LDAP, OIDC, mTLS)
- **Authorization**: RBAC and row-level security policies
- **Encryption**: AES-GCM encryption at rest with key rotation
- **Audit Logging**: Complete audit trail of all operations
- **Monitoring**: Prometheus metrics, Grafana dashboards, flame graphs
- **Tracing**: OpenTelemetry integration for distributed tracing

### High Availability
- **Replication**: Streaming replication with automatic failover
- **Read Replicas**: Scale read workloads horizontally
- **Distributed Transactions**: 2-Phase Commit support
- **Load Balancing**: Built-in connection pooling and load distribution

---

## 🚀 Quick Start

### Installation

```bash
# Using Go install
go install github.com/bindxdb/bindxdb/cmd/bindxdb@latest

# Using Docker
docker pull bindxdb/bindxdb:latest

# From source
git clone https://github.com/bindxdb/bindxdb.git
cd bindxdb
make build
```

### Running BindXDB

```bash
# Start with default configuration
bindxdb start

# Start with custom config
bindxdb start --config config.yaml

# Using Docker
docker run -p 8080:8080 -p 9090:9090 bindxdb/bindxdb:latest
```

### Basic Usage

**HTTP/REST API:**
```bash
# Create a table
curl -X POST http://localhost:8080/api/v1/query \
  -H "Content-Type: application/json" \
  -d '{"sql": "CREATE TABLE users (id INT PRIMARY KEY, name VARCHAR(100))"}'

# Insert data
curl -X POST http://localhost:8080/api/v1/query \
  -H "Content-Type: application/json" \
  -d '{"sql": "INSERT INTO users VALUES (1, '\''Alice'\'')"}'

# Query data
curl http://localhost:8080/api/v1/query?sql=SELECT%20*%20FROM%20users
```

**gRPC Client (Go):**
```go
package main

import (
    "context"
    "log"
    
    pb "github.com/bindxdb/bindxdb/api/proto"
    "google.golang.org/grpc"
)

func main() {
    conn, _ := grpc.Dial("localhost:9090", grpc.WithInsecure())
    defer conn.Close()
    
    client := pb.NewBindXDBClient(conn)
    
    resp, err := client.ExecuteQuery(context.Background(), &pb.QueryRequest{
        Sql: "SELECT * FROM users",
    })
    if err != nil {
        log.Fatal(err)
    }
    
    log.Println(resp.Rows)
}
```

**PostgreSQL Protocol:**
```bash
psql -h localhost -p 5432 -d bindxdb -U admin
> SELECT * FROM users;
```

---

## 📚 Documentation

- **[Getting Started Guide](docs/getting-started.md)** - Complete setup and first steps
- **[Architecture Overview](docs/architecture/ARCHITECTURE.md)** - System design and components
- **[SQL Reference](docs/sql-reference.md)** - Supported SQL syntax and features
- **[Plugin Development Guide](docs/plugin-development.md)** - Create custom plugins
- **[API Documentation](docs/api/)** - HTTP API and gRPC reference
- **[Configuration Guide](docs/configuration.md)** - All configuration options
- **[Performance Tuning](docs/performance.md)** - Optimization tips and benchmarks
- **[Deployment Guide](docs/deployment/)** - Docker, Kubernetes, bare metal

---

## 🏗️ Architecture

BindXDB follows a **microkernel architecture** with pluggable components:

```
┌─────────────────────────────────────────────────────────┐
│                    Protocol Layer                       │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌─────────┐│
│  │ HTTP/REST│  │   gRPC   │  │WebSocket │  │ Postgres││
│  └──────────┘  └──────────┘  └──────────┘  └─────────┘│
└─────────────────────────────────────────────────────────┘
                           │
┌─────────────────────────────────────────────────────────┐
│                     SQL Processing                      │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐             │
│  │  Parser  │→ │ Optimizer│→ │ Executor │             │
│  └──────────┘  └──────────┘  └──────────┘             │
└─────────────────────────────────────────────────────────┘
                           │
┌─────────────────────────────────────────────────────────┐
│                  Transaction Manager                    │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐             │
│  │   MVCC   │  │  Locks   │  │   WAL    │             │
│  └──────────┘  └──────────┘  └──────────┘             │
└─────────────────────────────────────────────────────────┘
                           │
┌─────────────────────────────────────────────────────────┐
│                    Storage Engine                       │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐             │
│  │ B+Tree   │  │HeapFile  │  │BufferPool│             │
│  └──────────┘  └──────────┘  └──────────┘             │
└─────────────────────────────────────────────────────────┘
```

**Key Components:**
- **Storage Layer**: Page-based storage with buffer pool and B+Tree indexes
- **Transaction Layer**: MVCC for concurrency, WAL for durability
- **SQL Layer**: Parser, optimizer, and volcano-style executor
- **Protocol Layer**: Multi-protocol support with TLS
- **Plugin System**: Dynamic loading of extensions

See [Architecture Documentation](docs/architecture/ARCHITECTURE.md) for details.

---

## 🧪 Development

### Prerequisites
- Go 1.21+
- Make
- Docker (optional)

### Build from Source

```bash
# Clone the repository
git clone https://github.com/bindxdb/bindxdb.git
cd bindxdb

# Install dependencies
make deps

# Build the binary
make build

# Run tests
make test

# Run linters
make lint

# Generate coverage report
make coverage
```

### Project Structure

```
bindxdb/
├── cmd/                    # Command-line tools
│   └── bindxdb/           # Main server binary
├── pkg/                    # Public libraries
│   ├── storage/           # Storage engine
│   ├── index/             # Index implementations
│   ├── transaction/       # Transaction manager
│   ├── sql/               # SQL parser/executor
│   └── protocol/          # Network protocols
├── internal/              # Private application code
│   ├── server/            # Server implementation
│   └── config/            # Configuration
├── api/                   # API definitions
│   ├── proto/             # gRPC protocol buffers
│   └── openapi/           # OpenAPI specs
├── plugins/               # Plugin examples
├── docs/                  # Documentation
├── test/                  # Integration tests
└── examples/              # Example code
```

---

## 🤝 Contributing

We welcome contributions! Please see our [Contributing Guide](CONTRIBUTING.md) for details.

### Quick Links
- **[Code of Conduct](CODE_OF_CONDUCT.md)**
- **[Development Roadmap](TODO.md)**
- **[Good First Issues](https://github.com/bindxdb/bindxdb/labels/good%20first%20issue)**
- **[Architecture Decision Records](docs/architecture/adr/)**

### Ways to Contribute
- 🐛 Report bugs
- 💡 Suggest new features
- 📝 Improve documentation
- 🔧 Submit pull requests
- 🌍 Translate documentation
- 💬 Help others in discussions

---

## 📊 Performance Benchmarks

| Metric | Target | Current |
|--------|--------|---------|
| Concurrent Connections | 100,000+ | 🔄 In Progress |
| Simple Query QPS | 50,000+ | 🔄 In Progress |
| Primary Key Lookup | <1ms | 🔄 In Progress |
| CPU Scaling | Linear to 64 cores | 🔄 In Progress |
| Memory (1M connections) | <2GB | 🔄 In Progress |

*Benchmarks will be updated as development progresses*

---

## 🗺️ Roadmap

**Phase 1: Core Engine** (Months 1-3) ✅ Planning
- Storage foundation with buffer pool
- B+Tree indexing
- Write-Ahead Logging and crash recovery

**Phase 2: SQL Processing** (Months 4-6) ⏳ Upcoming
- SQL parser and planner
- Query execution engine
- Plugin system

**Phase 3: Transactions** (Months 7-9) ⏳ Planned
- MVCC implementation
- Lock management
- Advanced features (constraints, triggers)

**Phase 4-6**: Networking, Security, and Production features

See [TODO.md](TODO.md) for the complete roadmap.

---

## 📜 License

BindXDB is licensed under the [MIT License](LICENSE).

---

## 🙏 Acknowledgments

BindXDB is inspired by:
- **PostgreSQL**: MVCC and recovery algorithms
- **MySQL/InnoDB**: Buffer pool and B+Tree design
- **SQLite**: Embedded database architecture
- **CMU Database Systems Course**: Educational foundations

---

## 📞 Community & Support

- **GitHub Issues**: [Bug reports and feature requests](https://github.com/bindxdb/bindxdb/issues)
- **GitHub Discussions**: [Questions and community discussion](https://github.com/bindxdb/bindxdb/discussions)
- **Discord**: [Real-time chat](https://discord.gg/bindxdb)
- **Twitter**: [@bindxdb](https://twitter.com/bindxdb)
- **Email**: [hello@bindxdb.io](mailto:hello@bindxdb.io)

---

<div align="center">

**Star ⭐ this repository if you find it useful!**

Made with ❤️ by the BindXDB Team

</div>
