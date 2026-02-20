# Contributing to CacheStorm

Thank you for your interest in contributing to CacheStorm! This document provides guidelines and instructions for contributing.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Making Changes](#making-changes)
- [Testing](#testing)
- [Commit Guidelines](#commit-guidelines)
- [Pull Request Process](#pull-request-process)
- [Release Process](#release-process)

## Code of Conduct

Be respectful and inclusive. Treat everyone with dignity and respect.

## Getting Started

### Prerequisites

- Go 1.21 or later
- Git
- Make (optional)

### Fork and Clone

```bash
# Fork the repository on GitHub
# Then clone your fork
git clone https://github.com/YOUR_USERNAME/cachestorm.git
cd cachestorm

# Add upstream remote
git remote add upstream https://github.com/cachestorm/cachestorm.git
```

## Development Setup

### Install Dependencies

```bash
go mod download
```

### Build

```bash
go build -o cachestorm ./cmd/cachestorm
```

### Run

```bash
./cachestorm
```

### Run Tests

```bash
# Run all tests
go test ./...

# Run with coverage
go test ./... -cover

# Run specific package tests
go test ./internal/store/... -v

# Run benchmarks
go test ./internal/store/... -bench=.
```

### Run Linter

```bash
# Install golangci-lint
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Run linter
golangci-lint run
```

## Making Changes

### Branch Naming

Use descriptive branch names:

- `feature/add-bitmap-commands` - New features
- `fix/memory-leak-in-shard` - Bug fixes
- `docs/improve-api-documentation` - Documentation
- `refactor/simplify-store-logic` - Refactoring

### Code Style

1. Follow [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
2. Run `go fmt` before committing
3. Run `golangci-lint run` and fix issues
4. Add comments for exported functions and types

### Adding a New Command

1. Create or modify command file in `internal/command/`
2. Implement the command handler
3. Register the command in the router
4. Add tests
5. Update documentation

Example:

```go
// internal/command/my_commands.go
package command

func RegisterMyCommands(router *Router) {
    router.Register(&CommandDef{Name: "MYCMD", Handler: cmdMYCMD})
}

func cmdMYCMD(ctx *Context) error {
    if ctx.ArgCount() < 1 {
        return ctx.WriteError(ErrWrongArgCount)
    }
    
    arg := ctx.ArgString(0)
    // ... implementation
    
    return ctx.WriteOK()
}
```

### Adding a New Data Type

1. Define the type in `internal/store/entry.go`
2. Implement the `Value` interface:
   - `Type() DataType`
   - `SizeOf() int64`
   - `String() string`
   - `Clone() Value`
3. Add commands to manipulate the type
4. Add tests
5. Update documentation

## Testing

### Unit Tests

```go
func TestMyCommand(t *testing.T) {
    s := store.NewStore()
    router := command.NewRouter()
    command.RegisterMyCommands(router)
    
    // Test implementation
}
```

### Integration Tests

Place integration tests in `tests/` directory:

```go
func TestIntegration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }
    // ... integration test
}
```

### Benchmark Tests

```go
func BenchmarkMyCommand(b *testing.B) {
    s := store.NewStore()
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        // Benchmark code
    }
}
```

## Commit Guidelines

### Commit Message Format

```
<type>(<scope>): <subject>

<body>

<footer>
```

Types:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation
- `style`: Code style (formatting, etc.)
- `refactor`: Refactoring
- `test`: Adding/updating tests
- `chore`: Maintenance tasks

Examples:

```
feat(tags): add cascade invalidation support

Implement cascade invalidation that invalidates all child tags
when a parent tag is invalidated.

Closes #123
```

```
fix(store): resolve memory leak in shard eviction

The eviction routine was not properly cleaning up expired entries,
causing memory to grow unbounded.
```

## Pull Request Process

1. **Create a Branch**: Create a feature branch from `main`
2. **Make Changes**: Implement your changes
3. **Test**: Ensure all tests pass
4. **Lint**: Fix any linting issues
5. **Commit**: Use proper commit messages
6. **Push**: Push to your fork
7. **Open PR**: Create a pull request

### PR Checklist

- [ ] Tests pass
- [ ] Linter passes
- [ ] Documentation updated
- [ ] CHANGELOG.md updated (if applicable)
- [ ] Commit messages follow guidelines

### PR Review

- PRs require at least 1 approval
- Address all review comments
- Keep PRs focused and reasonably sized

## Release Process

Releases are automated via GitHub Actions:

1. Update CHANGELOG.md
2. Create a tag: `git tag v1.0.0`
3. Push tag: `git push origin v1.0.0`
4. GitHub Actions will:
   - Build binaries for all platforms
   - Create GitHub release
   - Publish Docker images

### Version Numbering

We follow [Semantic Versioning](https://semver.org/):

- `MAJOR`: Breaking changes
- `MINOR`: New features, backwards compatible
- `PATCH`: Bug fixes, backwards compatible

## Project Structure

```
cachestorm/
├── cmd/cachestorm/        # Main application
├── internal/
│   ├── command/           # Command handlers
│   ├── config/            # Configuration
│   ├── logger/            # Logging
│   ├── resp/              # RESP protocol
│   ├── server/            # Server implementation
│   └── store/             # Data store
├── plugins/               # Plugin implementations
├── docs/                  # Documentation
├── examples/              # Example code
├── config/                # Configuration examples
├── docker/                # Docker files
├── .github/workflows/     # CI/CD workflows
├── README.md
├── CHANGELOG.md
├── CONTRIBUTING.md
└── LICENSE
```

## Getting Help

- GitHub Issues: https://github.com/cachestorm/cachestorm/issues
- Discussions: https://github.com/cachestorm/cachestorm/discussions

## License

By contributing, you agree that your contributions will be licensed under the Apache 2.0 License.
