# Contributing to BindXDB

Thank you for your interest in contributing to BindXDB! This document provides guidelines and instructions for contributing to the project.

---

## Table of Contents

1. [Code of Conduct](#code-of-conduct)
2. [Getting Started](#getting-started)
3. [Development Setup](#development-setup)
4. [How to Contribute](#how-to-contribute)
5. [Coding Standards](#coding-standards)
6. [Testing Guidelines](#testing-guidelines)
7. [Submitting Changes](#submitting-changes)
8. [Review Process](#review-process)
9. [Community](#community)

---

## Code of Conduct

This project adheres to a [Code of Conduct](CODE_OF_CONDUCT.md). By participating, you are expected to uphold this code. Please report unacceptable behavior to [conduct@bindxdb.io](mailto:conduct@bindxdb.io).

---

## Getting Started

### Prerequisites

- **Go**: Version 1.21 or higher
- **Git**: For version control
- **Make**: For build automation
- **Docker**: Optional, for testing containerized deployments

### Fork and Clone

1. Fork the repository on GitHub
2. Clone your fork locally:
   ```bash
   git clone https://github.com/YOUR_USERNAME/bindxdb.git
   cd bindxdb
   ```
3. Add the upstream repository:
   ```bash
   git remote add upstream https://github.com/bindxdb/bindxdb.git
   ```

---

## Development Setup

### Install Dependencies

```bash
# Install Go dependencies
make deps

# Install development tools (linters, formatters)
make install-tools
```

### Build the Project

```bash
# Build the binary
make build

# Build with debug symbols
make build-debug

# Cross-compile for multiple platforms
make build-all
```

### Run Tests

```bash
# Run all tests
make test

# Run specific test package
go test ./pkg/storage/...

# Run with coverage
make coverage

# Run integration tests
make test-integration
```

### Start Development Server

```bash
# Start with default config
make dev

# Start with custom config
bindxdb start --config dev-config.yaml
```

---

## How to Contribute

### Types of Contributions

**🐛 Bug Reports**
- Search existing issues first
- Use the bug report template
- Include reproduction steps
- Provide system information

**💡 Feature Requests**
- Check the roadmap in [TODO.md](TODO.md)
- Use the feature request template
- Explain the use case
- Discuss design before implementation

**📝 Documentation**
- Fix typos and clarify content
- Add examples and tutorials
- Improve API documentation
- Translate documentation

**🔧 Code Contributions**
- Pick an issue labeled `good first issue` or `help wanted`
- Comment on the issue to claim it
- Follow the coding standards
- Write tests for all changes

### Finding Issues

Good starting points:
- Issues labeled [`good first issue`](https://github.com/bindxdb/bindxdb/labels/good%20first%20issue)
- Issues labeled [`help wanted`](https://github.com/bindxdb/bindxdb/labels/help%20wanted)
- Documentation improvements
- Performance optimizations

---

## Coding Standards

### Go Style Guide

We follow the official [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments) and [Effective Go](https://golang.org/doc/effective_go).

**Key Principles:**

1. **Formatting**: Use `gofmt` (enforced by CI)
   ```bash
   gofmt -w .
   ```

2. **Linting**: Pass `golangci-lint` checks
   ```bash
   make lint
   ```

3. **Naming Conventions**:
   - Use `MixedCaps` or `mixedCaps` (not underscores)
   - Interfaces: `-er` suffix when possible (e.g., `Reader`, `Writer`)
   - Packages: short, lowercase, no underscores
   - Files: lowercase with underscores (e.g., `buffer_pool.go`)

4. **Error Handling**:
   ```go
   // Good
   if err != nil {
       return fmt.Errorf("failed to open file: %w", err)
   }
   
   // Avoid
   if err != nil {
       panic(err)
   }
   ```

5. **Comments**:
   - All exported functions, types, and constants must have doc comments
   - Comments should be complete sentences
   - Start with the name of the thing being described
   
   ```go
   // BufferPool manages a pool of in-memory pages with LRU eviction.
   type BufferPool struct {
       // ...
   }
   
   // FetchPage retrieves a page from the buffer pool or loads it from disk.
   func (bp *BufferPool) FetchPage(pageID PageID) (*Page, error) {
       // ...
   }
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
├── scripts/               # Build and deployment scripts
├── docs/                  # Documentation
├── test/                  # Integration tests
└── examples/              # Example code
```

### Code Organization

1. **Keep packages focused**: Single responsibility principle
2. **Avoid circular dependencies**: Use interfaces to break cycles
3. **Minimize public API**: Only export what's necessary
4. **Use dependency injection**: Pass dependencies explicitly

**Example:**
```go
// Good: Interface for testability
type PageFetcher interface {
    FetchPage(pageID PageID) (*Page, error)
}

type BufferPool struct {
    fetcher PageFetcher
}

// Bad: Hard-coded dependency
type BufferPool struct {
    fileManager *FileManager // Tightly coupled
}
```

---

## Testing Guidelines

### Test Coverage Requirements

- **Core components**: 90%+ coverage
- **New features**: Must include tests
- **Bug fixes**: Add regression tests

### Writing Tests

**Unit Tests:**
```go
func TestBufferPool_FetchPage(t *testing.T) {
    bp := NewBufferPool(1024)
    defer bp.Close()
    
    page, err := bp.FetchPage(PageID{FileID: 1, PageNum: 0})
    require.NoError(t, err)
    assert.NotNil(t, page)
}
```

**Table-Driven Tests:**
```go
func TestParser_Parse(t *testing.T) {
    tests := []struct {
        name    string
        sql     string
        wantErr bool
    }{
        {"simple select", "SELECT * FROM users", false},
        {"invalid syntax", "SELECT FROM", true},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            _, err := parser.Parse(tt.sql)
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

**Benchmark Tests:**
```go
func BenchmarkBPlusTree_Insert(b *testing.B) {
    tree := NewBPlusTree()
    b.ResetTimer()
    
    for i := 0; i < b.N; i++ {
        tree.Insert([]byte(fmt.Sprintf("key%d", i)), uint64(i))
    }
}
```

### Integration Tests

Located in `test/integration/`:
```bash
make test-integration
```

### Performance Tests

Run benchmarks before and after changes:
```bash
make bench > before.txt
# Make changes
make bench > after.txt
benchcmp before.txt after.txt
```

---

## Submitting Changes

### Workflow

1. **Create a branch**
   ```bash
   git checkout -b feature/my-feature
   # or
   git checkout -b fix/issue-123
   ```

2. **Make your changes**
   ```bash
   # Write code
   # Add tests
   # Update documentation
   ```

3. **Commit with meaningful messages**
   ```bash
   git add .
   git commit -m "feat: add B+Tree bulk loading optimization
   
   - Implement bottom-up bulk loading algorithm
   - Reduce tree height for sorted inserts
   - Add benchmarks showing 3x improvement
   
   Closes #123"
   ```

4. **Keep your branch updated**
   ```bash
   git fetch upstream
   git rebase upstream/main
   ```

5. **Push to your fork**
   ```bash
   git push origin feature/my-feature
   ```

6. **Create a Pull Request**
   - Use the PR template
   - Link related issues
   - Add screenshots for UI changes
   - Request reviews from maintainers

### Commit Message Format

We follow [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <subject>

<body>

<footer>
```

**Types:**
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting)
- `refactor`: Code refactoring
- `perf`: Performance improvements
- `test`: Adding tests
- `chore`: Maintenance tasks
- `ci`: CI/CD changes

**Example:**
```
feat(storage): implement write-ahead logging

- Add WAL record format with LSN tracking
- Implement group commit for better throughput
- Add ARIES recovery algorithm

Closes #45
```

### Pull Request Checklist

Before submitting a PR, ensure:

- [ ] Code follows the style guide
- [ ] All tests pass (`make test`)
- [ ] Linters pass (`make lint`)
- [ ] Documentation updated
- [ ] Changelog entry added (for user-facing changes)
- [ ] Commit messages follow convention
- [ ] PR description explains the change
- [ ] Related issues are linked

---

## Review Process

### What to Expect

1. **Automated Checks**: CI runs tests, linters, and builds
2. **Code Review**: Maintainers review your code
3. **Feedback**: You may be asked to make changes
4. **Approval**: At least one maintainer approval required
5. **Merge**: Maintainer merges your PR

### Review Criteria

Reviewers consider:
- **Correctness**: Does it work as intended?
- **Testing**: Are there adequate tests?
- **Performance**: Any performance implications?
- **Design**: Is the design clean and maintainable?
- **Documentation**: Is it well-documented?
- **Breaking Changes**: Are they necessary and documented?

### Responding to Feedback

- Be open to suggestions
- Ask questions if unclear
- Make requested changes promptly
- Update the PR description if scope changes
- Re-request review after addressing feedback

---

## Development Guidelines

### Performance Considerations

1. **Profile before optimizing**
   ```bash
   go test -bench=. -cpuprofile=cpu.prof
   go tool pprof cpu.prof
   ```

2. **Avoid premature optimization**
   - Write clear code first
   - Optimize hot paths only
   - Use benchmarks to validate improvements

3. **Memory efficiency**
   - Reuse buffers when possible
   - Use object pooling for frequently allocated objects
   - Be mindful of allocations in tight loops

### Concurrency

1. **Avoid data races**
   ```bash
   go test -race ./...
   ```

2. **Use channels for communication**
   ```go
   // Good
   ch := make(chan *Page)
   go func() { ch <- page }()
   
   // Avoid shared state without synchronization
   ```

3. **Document locking invariants**
   ```go
   // Mutex protects pages map and evictionList.
   // Must be held when accessing either field.
   mu sync.RWMutex
   ```

### Error Handling

1. **Wrap errors with context**
   ```go
   if err != nil {
       return fmt.Errorf("failed to fetch page %v: %w", pageID, err)
   }
   ```

2. **Define sentinel errors**
   ```go
   var ErrPageNotFound = errors.New("page not found")
   ```

3. **Use custom error types when needed**
   ```go
   type TransactionError struct {
       TxnID TransactionID
       Err   error
   }
   ```

---

## Community

### Communication Channels

- **GitHub Issues**: Bug reports, feature requests
- **GitHub Discussions**: Questions, ideas, general discussion
- **Discord**: Real-time chat with the community
- **Twitter**: [@bindxdb](https://twitter.com/bindxdb) for announcements

### Getting Help

- Check existing documentation
- Search closed issues
- Ask in GitHub Discussions
- Join our Discord server

### Maintainers

- @maintainer1 - Core Engine
- @maintainer2 - SQL Processing
- @maintainer3 - Networking

---

## Recognition

Contributors are recognized in:
- `CONTRIBUTORS.md` file
- Release notes
- Annual contributor highlights

---

## License

By contributing to BindXDB, you agree that your contributions will be licensed under the MIT License.

---

Thank you for contributing to BindXDB! 🎉
