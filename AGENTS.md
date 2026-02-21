# CacheStorm Agent Configuration

This file provides guidance for AI agents working on the CacheStorm codebase.

## Project Overview

CacheStorm is a high-performance, Redis-compatible in-memory database written in Go with 1,458+ commands across 50+ modules.

## Build Commands

```bash
# Build binary
go build -o cachestorm ./cmd/cachestorm

# Build for Windows
go build -o cachestorm.exe ./cmd/cachestorm
```

## Test Commands

```bash
# Run all tests
go test ./internal/... -v

# Run tests with coverage
go test ./internal/... -v -coverprofile=coverage.out

# Run specific test
go test ./internal/command/... -v -run TestStringCommands
```

## Lint Commands

```bash
# Run golangci-lint
golangci-lint run

# Run with timeout
golangci-lint run --timeout=5m
```

## Type Check

```bash
# Go vet
go vet ./...

# Build check
go build ./...
```

## Code Style

- No comments unless explicitly requested
- Follow existing patterns in the codebase
- Use unique names with X suffix when conflicts occur (e.g., `PoolX`, `CounterX2`)
- Each new command module needs:
  1. `Register*Commands(router *Router)` function
  2. Command handler functions following `func cmdNAME(ctx *Context) error` pattern
  3. Registration in `internal/server/server.go`
  4. Update to `CHANGELOG.md`

## File Structure

```
D:\Codebox\CacheStorm\
├── cmd\cachestorm\main.go          # Main entry point
├── internal\
│   ├── command\                    # Command handlers
│   │   ├── router.go               # Router and CommandDef
│   │   ├── context.go              # Context and helpers
│   │   ├── *_commands.go           # Command modules
│   │   └── *_test.go               # Tests
│   ├── server\server.go            # Server registration
│   ├── store\                      # Data store
│   └── resp\                       # RESP protocol
├── .github\workflows\              # CI/CD workflows
├── scripts\                        # Installation scripts
├── config\                         # Configuration files
├── docs\                           # Documentation
└── examples\                       # Example code
```

## Version Progress

| Version | Commands | Description |
|---------|----------|-------------|
| v0.1.0  | 289      | Core Redis commands |
| v0.1.18 | 854      | Integration commands |
| v0.1.19 | 951      | Extended commands |
| v0.1.20 | 1,080    | More commands |
| v0.1.21 | 1,218    | Extra commands |
| v0.1.22 | 1,393    | Advanced commands 2 |
| v0.1.23 | 1,458    | Resilience commands |

## Commit Convention

- Use conventional commits: `feat:`, `fix:`, `docs:`, `test:`, `ci:`
- Example: `feat: add circuit breaker commands for resilience patterns`

## Release Process

1. Tag version: `git tag v0.1.24`
2. Push tag: `git push origin v0.1.24`
3. CI/CD handles the rest (builds, tests, releases, Docker images)
