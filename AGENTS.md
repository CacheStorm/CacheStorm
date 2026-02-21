# CacheStorm Agent Configuration

This file provides guidance for AI agents working on the CacheStorm codecodebase.

## Project Overview

CacheStorm is a high-performance, Redis-compatible in-memory database written in Go with 1,578+ commands across 48+ modules.

## Goal

- **100% Redis compatibility** - Currently at ~99%
- **1,500+ commands** - Currently at **1,578 commands**
- **100% test coverage and 100% success rate** - Currently at **16.3% coverage, 100% pass rate**

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
│   │   ├── *_commands.go           # Command modules (48 files)
│   │   ├── comprehensive_test.go   # Comprehensive tests
│   │   └── *_test.go               # Tests
│   ├── server\server.go            # Server registration
│   ├── store\                      # Data store
│   │   ├── store.go                # Store with shards
│   │   ├── entry.go                # Value types (String, Hash, List, Set, SortedSet)
│   │   ├── shard.go                # Sharding implementation
│   │   ├── datastructures.go       # PriorityQueue, LRUCache, etc.
│   │   └── events.go               # Event management
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
| v0.1.24 | 1,598    | ML commands |
| v0.1.25 | 1,606    | Redis compatibility improvements |
| v0.1.26 | 1,578    | Bug fixes and test improvements |

## Test Coverage (Current)

| Package | Coverage |
|---------|----------|
| command | 16.3% |
| store | 12.3% |
| cluster | 23.3% |
| resp | 45.6% |

## Known Issues Fixed

1. **Deadlock in HDEL/HGETDEL/SREM/SPOP/ZREM** - Fixed by unlocking before calling `Store.Delete`
2. **Deadlock in PriorityQueue** - Fixed by removing lock from `Len()` method (heap interface)
3. **Infinite loop in XADD** - Fixed by using labeled break
4. **Negative random indices** - Fixed in `generateUUID()` and `generateID()` by using `math/rand` or `abs()`
5. **Test context missing Transaction** - Fixed by using `NewContext()` instead of direct struct literal

## Commit Convention

- Use conventional commits: `feat:`, `fix:`, `docs:`, `test:`, `ci:`
- Example: `feat: add circuit breaker commands for resilience patterns`

## Release Process

1. Update CHANGELOG.md
2. Commit: `git add -A && git commit -m "fix: ..."`
3. Push: `git push origin main`
4. Tag: `git tag v0.1.XX`
5. Push tag: `git push origin v0.1.XX`
6. CI/CD handles the rest (builds, tests, releases, Docker images)

## Next Steps

1. Continue improving test coverage until 100%
2. Fix any remaining test failures
3. Maintain Redis compatibility
