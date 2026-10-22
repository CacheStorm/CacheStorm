# CacheStorm Test Coverage Report

Measured on **2026-10-20**, against the working tree at commit `fe598ff` with pre-existing uncommitted changes. Go **1.26.9**.

The complete root-module test run passed. Statement-weighted coverage is **90.8% for the 18 internal packages**, or **88.2% across all instrumented root-module packages**, including the untested CLI and Go SDK. These percentages are not Redis compatibility or durability scores.

## Reproduce the measurement

```bash
go test ./... -count=1 -timeout=180s -coverprofile=coverage.out
go tool cover -func=coverage.out
```

The review's profile and output are retained locally under `.temp_files/project_review_2026_10_20/coverage.out` and `go-test.log`. These generated files are ignored by Git.

## Internal packages

| Package | Statement coverage |
|---------|-------------------:|
| `internal/acl` | 100.0% |
| `internal/batch` | 97.9% |
| `internal/buffer` | 96.9% |
| `internal/cluster` | 97.7% |
| `internal/command` | 89.8% |
| `internal/config` | 97.9% |
| `internal/graph` | 98.7% |
| `internal/logger` | 100.0% |
| `internal/module` | 93.3% |
| `internal/persistence` | 87.2% |
| `internal/plugin` | 98.8% |
| `internal/pool` | 86.1% |
| `internal/replication` | 96.5% |
| `internal/resp` | 90.2% |
| `internal/search` | 94.4% |
| `internal/sentinel` | 95.3% |
| `internal/server` | 93.2% |
| `internal/store` | 94.0% |

## Other instrumented packages

| Package | Statement coverage |
|---------|-------------------:|
| `clients/go` | 0.0% |
| `cmd/benchmark` | 0.0% |
| `cmd/cachestorm` | 0.0% |
| `plugins/example-hooks` | 60.6% |
| `plugins/metrics` | 77.5% |

## Interpretation and remaining work

`internal/command` contains most instrumented statements, so a simple average of package percentages overstates overall coverage. Use statement-weighted totals when comparing runs.

The 2026-10-20 race run also passed: `go test -race ./internal/... -count=1 -timeout=180s`.

Some persistence and replication tests contain unconditional skips. External-server integration tests can skip when localhost:6380 is unavailable. Nested Go example modules, the Python and TypeScript clients and website behavior are not exercised by the root Go test command.

The [project review](docs/PROJECT_REVIEW.md) reproduces acknowledged transaction writes disappearing on replay, incomplete EXEC dispatch and disconnected distributed configuration despite a passing suite. It also records SDK defects. The [roadmap](ROADMAP.md) prioritizes regression tests for these guarantees before raising headline coverage targets.

Replace this snapshot after the next significant code change. Do not reuse its success claim for another commit or for skipped scenarios.
