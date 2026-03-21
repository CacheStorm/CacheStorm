# Contributing to CacheStorm

Thank you for your interest in contributing to CacheStorm! This document provides comprehensive guidelines and instructions for contributing to this high-performance, Redis-compatible in-memory database.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Project Structure](#project-structure)
- [Making Changes](#making-changes)
- [Testing](#testing)
- [Commit Guidelines](#commit-guidelines)
- [Pull Request Process](#pull-request-process)
- [Release Process](#release-process)

## Code of Conduct

Be respectful and inclusive. Treat everyone with dignity and respect. We welcome contributors from all backgrounds and experience levels.

## Getting Started

### Prerequisites

- **Go 1.22 or later** - CacheStorm uses modern Go features
- **Git** - For version control
- **Make** (optional) - For build automation
- **Redis CLI** (optional) - For testing Redis compatibility

### Fork and Clone

```bash
# Fork the repository on GitHub
# Then clone your fork
git clone https://github.com/YOUR_USERNAME/cachestorm.git
cd cachestorm

# Add upstream remote
git remote add upstream https://github.com/cachestorm/cachestorm.git

# Fetch all branches
git fetch upstream
```

## Development Setup

### Install Dependencies

```bash
go mod download
go mod verify
```

### Build

```bash
# Build the main binary
go build -o cachestorm ./cmd/cachestorm

# Build with optimizations
go build -ldflags="-s -w" -o cachestorm ./cmd/cachestorm
```

### Run

```bash
# Run with default settings
./cachestorm

# Run with custom config
./cachestorm --config config.yaml --port 6380

# Run with debug logging
./cachestorm --log-level debug
```

### Run Tests

```bash
# Run all tests
go test ./...

# Run with coverage
go test ./... -cover

# Run with race detection
go test ./... -race

# Run specific package tests
go test ./internal/store/... -v
go test ./internal/command/... -v

# Run benchmarks
go test ./internal/store/... -bench=.
go test ./internal/store/... -bench=. -benchmem

# Run integration tests (requires running server)
go test ./tests/... -v
```

### Run Linter

```bash
# Install golangci-lint
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Run linter
golangci-lint run

# Run with auto-fix
golangci-lint run --fix
```

## Project Structure

```
cachestorm/
├── cmd/cachestorm/        # Main application entry point
├── internal/
│   ├── acl/               # Access control lists (89.9% coverage)
│   ├── batch/             # Batch processing (94.4% coverage)
│   ├── buffer/            # Buffer management (98.4% coverage)
│   ├── cluster/           # Clustering and gossip (88.8% coverage)
│   ├── command/           # Command handlers - 1,606 commands (78.7% coverage)
│   ├── config/            # Configuration management (95.4% coverage)
│   ├── graph/             # Graph operations (98.6% coverage)
│   ├── logger/            # Logging (100% coverage)
│   ├── module/            # Module system (96.4% coverage)
│   ├── persistence/       # AOF/RDB persistence (78.6% coverage)
│   ├── plugin/            # Plugin system (95.6% coverage)
│   ├── pool/              # Connection pooling (81.1% coverage)
│   ├── replication/       # Master-slave replication (78.9% coverage)
│   ├── resp/              # RESP protocol implementation (86.9% coverage)
│   ├── search/            # Search functionality (97.2% coverage)
│   ├── sentinel/          # Redis Sentinel support (73.6% coverage)
│   ├── server/            # Server implementation (82.3% coverage)
│   └── store/             # Data store - 256-shard architecture (92.3% coverage)
├── plugins/               # Plugin implementations
├── tests/                 # Integration tests
├── benchmarks/            # Performance benchmarks
├── docs/                  # Documentation
├── config/                # Configuration examples
├── docker/                # Docker files
├── .github/workflows/     # CI/CD workflows
├── README.md
├── CHANGELOG.md
├── CONTRIBUTING.md
├── COVERAGE_REPORT.md
└── LICENSE
```

## Making Changes

### Branch Naming

Use descriptive branch names with prefixes:

- `feature/add-stream-commands` - New features
- `fix/memory-leak-in-shard` - Bug fixes
- `docs/improve-api-documentation` - Documentation
- `refactor/simplify-store-logic` - Refactoring
- `test/add-pool-coverage` - Test improvements
- `perf/optimize-hash-lookup` - Performance improvements

### Code Style

1. Follow [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
2. Run `go fmt ./...` before committing
3. Run `golangci-lint run` and fix all issues
4. Add comments for all exported functions, types, and constants
5. Keep functions focused and under 50 lines when possible
6. Use meaningful variable names

### Adding a New Command

CacheStorm has 1,606 commands. To add a new command:

1. Identify the appropriate command file in `internal/command/`
2. Implement the command handler following the existing pattern
3. Register the command in the router
4. Add comprehensive tests
5. Update documentation

Example:

```go
// internal/command/example_commands.go
package command

import (
    "github.com/cachestorm/cachestorm/internal/resp"
)

func RegisterExampleCommands(router *Router) {
    router.Register(&CommandDef{
        Name: "EXAMPLE",
        Handler: cmdEXAMPLE,
    })
}

func cmdEXAMPLE(ctx *Context) error {
    if ctx.ArgCount() < 1 {
        return ctx.WriteError(ErrWrongArgCount)
    }

    arg := ctx.ArgString(0)

    // Validate arguments
    if arg == "" {
        return ctx.WriteError(fmt.Errorf("ERR invalid argument"))
    }

    // Implementation
    result := processExample(arg)

    return ctx.WriteBulkString(result)
}
```

### Adding a New Data Type

1. Define the type constant in `internal/store/entry.go`
2. Implement the `Value` interface:
   - `Type() DataType`
   - `SizeOf() int64`
   - `String() string`
   - `Clone() Value`
3. Add commands to manipulate the type
4. Add tests
5. Update documentation

## Testing

CacheStorm maintains **~96% average coverage** across 18 internal packages with **100% test success rate**.

### Writing Unit Tests

```go
func TestMyCommand(t *testing.T) {
    s := store.NewStore()
    defer s.Close()

    router := command.NewRouter()
    command.RegisterMyCommands(router)

    ctx := &command.Context{
        Store: s,
        // ... setup context
    }

    // Test success case
    err := cmdMYCommand(ctx)
    if err != nil {
        t.Errorf("unexpected error: %v", err)
    }

    // Test error case
    ctx.Args = []resp.Value{}
    err = cmdMYCommand(ctx)
    if err == nil {
        t.Error("expected error for empty args")
    }
}
```

### Writing Table-Driven Tests

```go
func TestMyCommandTable(t *testing.T) {
    tests := []struct {
        name     string
        args     []string
        wantErr  bool
        wantResp string
    }{
        {"valid input", []string{"key", "value"}, false, "OK"},
        {"missing key", []string{}, true, ""},
        {"empty key", []string{""}, true, ""},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

### Integration Tests

Place integration tests in `tests/` directory:

```go
func TestIntegrationFeature(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test in short mode")
    }

    // Check if server is running
    conn, err := net.Dial("tcp", "localhost:6380")
    if err != nil {
        t.Skipf("Server not running: %v", err)
    }
    conn.Close()

    // Run integration test
}
```

### Benchmark Tests

```go
func BenchmarkMyCommand(b *testing.B) {
    s := store.NewStore()
    defer s.Close()

    // Setup
    ctx := &command.Context{Store: s}

    b.ResetTimer()
    b.ReportAllocs()

    for i := 0; i < b.N; i++ {
        cmdMYCommand(ctx)
    }
}
```

### Coverage Requirements

- All new code must have tests
- Aim for >80% coverage in new packages
- Critical paths (persistence, replication) should have >90% coverage
- Run `go test ./... -cover` to check coverage

## Commit Guidelines

### Commit Message Format

```
<type>(<scope>): <subject>

<body>

<footer>
```

### Types

- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation only changes
- `style`: Code style (formatting, missing semi colons, etc)
- `refactor`: Code refactoring
- `perf`: Performance improvement
- `test`: Adding or updating tests
- `chore`: Build process or auxiliary tool changes

### Scopes

- `store`: Data store changes
- `command`: Command implementations
- `server`: Server implementation
- `cluster`: Clustering
- `replication`: Replication
- `persistence`: AOF/RDB
- `resp`: RESP protocol
- `config`: Configuration
- `docs`: Documentation

### Examples

```
feat(commands): add HYPERLOGLOG.COUNT command

Implement the HYPERLOGLOG.COUNT command for approximate
cardinality estimation with 0.81% standard error.

Closes #123
```

```
fix(store): resolve race condition in shard eviction

The eviction routine was not holding the write lock during
the entire eviction process, causing race conditions with
concurrent writes.

Fixes #456
```

```
test(persistence): add coverage for RDB corruption handling

Add tests for RDB reading when files are corrupted,
truncated, or have invalid checksums.

Increases persistence coverage from 78.6% to 82.3%.
```

## Pull Request Process

1. **Create a Branch**: Create a feature branch from `main`
   ```bash
   git checkout -b feature/my-feature
   ```

2. **Make Changes**: Implement your changes with tests

3. **Test**: Ensure all tests pass
   ```bash
   go test ./... -race
   ```

4. **Lint**: Fix any linting issues
   ```bash
   golangci-lint run
   ```

5. **Commit**: Use proper commit messages
   ```bash
   git commit -m "feat(commands): add new command"
   ```

6. **Push**: Push to your fork
   ```bash
   git push origin feature/my-feature
   ```

7. **Open PR**: Create a pull request with a clear description

### PR Checklist

- [ ] All tests pass (`go test ./...`)
- [ ] Race detector passes (`go test ./... -race`)
- [ ] Linter passes (`golangci-lint run`)
- [ ] Code is formatted (`go fmt ./...`)
- [ ] Documentation updated
- [ ] CHANGELOG.md updated (if applicable)
- [ ] Commit messages follow guidelines
- [ ] PR description explains the changes

### PR Review

- PRs require at least 1 approval from a maintainer
- Address all review comments
- Keep PRs focused and reasonably sized (<500 lines when possible)
- Respond to feedback within 48 hours

## Release Process

Releases are automated via GitHub Actions:

1. Update CHANGELOG.md with new version
2. Create a signed tag:
   ```bash
   git tag -s v0.1.28 -m "Release v0.1.28"
   ```
3. Push the tag:
   ```bash
   git push origin v0.1.28
   ```
4. GitHub Actions will:
   - Run all tests
   - Build binaries for all platforms (Linux, macOS, Windows)
   - Build Docker images for amd64 and arm64
   - Create GitHub release with changelog
   - Publish to Docker Hub and GHCR
   - Update package managers (Homebrew, Scoop)

### Version Numbering

We follow [Semantic Versioning](https://semver.org/):

- `MAJOR`: Breaking changes (incompatible API changes)
- `MINOR`: New features, backwards compatible
- `PATCH`: Bug fixes, backwards compatible

Example: `v0.1.27`

## Testing Best Practices

### Current Coverage Status

| Range | Count | Packages |
|-------|-------|----------|
| 100% | 3 | acl, config, logger |
| 95-99% | 10 | replication, store, graph, buffer, cluster, batch, search, module, plugin, sentinel |
| 90-94% | 3 | persistence, server, resp |
| 84-86% | 2 | pool, command |

### When Writing Tests

1. **Test both success and error paths**
2. **Use table-driven tests** for multiple test cases
3. **Mock external dependencies** (network, filesystem)
4. **Clean up resources** with `defer`
5. **Use `t.Parallel()`** for independent tests
6. **Check for goroutine leaks** with `go.uber.org/goleak`

### Common Testing Patterns

```go
// Setup/Teardown
func TestMain(m *testing.M) {
    // Global setup
    code := m.Run()
    // Global teardown
    os.Exit(code)
}

// Subtests
func TestFeature(t *testing.T) {
    t.Run("Subtest1", func(t *testing.T) {
        // Test 1
    })
    t.Run("Subtest2", func(t *testing.T) {
        // Test 2
    })
}

// Parallel tests
func TestParallel(t *testing.T) {
    t.Parallel()
    // Test runs in parallel with other parallel tests
}
```

## Getting Help

- **GitHub Issues**: https://github.com/cachestorm/cachestorm/issues
- **Discussions**: https://github.com/cachestorm/cachestorm/discussions
- **Documentation**: See `docs/` directory

## License

By contributing to CacheStorm, you agree that your contributions will be licensed under the MIT License.

---

Thank you for contributing to CacheStorm! Your efforts help make this project better for everyone.
