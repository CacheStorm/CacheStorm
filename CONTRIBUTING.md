# Contributing to CacheStorm

CacheStorm is a Go server with separate Go, Python and TypeScript clients and a React documentation website. Read the [project review](docs/PROJECT_REVIEW.md) and [roadmap](ROADMAP.md) before starting a feature or compatibility change.

## Development requirements

Use the Go version required by [go.mod](go.mod), currently **1.26.9**. The website workflow uses Node.js 22 and npm. The Python client uses Python's standard library; its packaging and supported-version policy remain roadmap work.

```bash
go mod download
go mod verify
go build -o cachestorm ./cmd/cachestorm
./cachestorm --config config/cachestorm.example.yaml
```

The executable accepts `--config`, `--bind`, `--port` and `--version`. Configuration can also be selected with `CACHESTORM_CONFIG`.

## Validation

Run these checks from the repository root:

```bash
go test ./... -count=1
go test -race ./internal/... -count=1
go build ./...
go vet ./...
golangci-lint run --timeout=5m
git diff --check
```

CI pins its Go and golangci-lint versions in [.github/workflows/ci.yml](.github/workflows/ci.yml). Match the CI linter version when comparing diagnostics. The 2026-10-20 local review found three staticcheck findings with a newer installed linter; see the project review.

For coverage and benchmarks:

```bash
go test ./internal/... -count=1 -coverprofile=coverage.out
go tool cover -func=coverage.out
go test ./internal/store/... ./benchmarks/... -run '^$' -bench=. -benchmem
```

Coverage measures executed statements. It does not establish command compatibility, transaction isolation, crash durability or distributed failover. The [coverage report](COVERAGE_REPORT.md) records the measured snapshot and its scope.

Some tests under `tests/` skip when no server listens on localhost:6380. Others start their own server. Check skips before describing an integration run as complete. Nested Go example modules are excluded from root `go test ./...`; their module metadata currently needs maintenance.

Build the website separately:

```bash
cd website
npm ci
npm run build
```

SDK build and test gates are tracked separately in the roadmap. The TypeScript SDK's declared `jest` and `eslint` commands currently lack corresponding dependencies and checked-in tests. Do not describe the website build as an SDK test.

## Code changes

Follow [AGENTS.md](AGENTS.md) and existing patterns. Do not add code comments unless explicitly requested. Format changed Go files with `gofmt`.

A new command module needs a `Register*Commands(router *Router)` function, handlers using `func cmdNAME(ctx *Context) error`, registration in `internal/server/server.go`, tests for its observable behavior and a changelog entry. A registered name alone is not a completed feature.

For write commands, check error propagation, memory rejection, TTL, tag bookkeeping, ACL key extraction, scripting, transaction dispatch and persistence. Avoid adding another command-specific table when existing metadata can represent the same rule.

Prefer regression tests that assert client-visible behavior. Durability changes need replay checks; synchronization changes need deterministic ordering and race detection. Keep integration tests independent of machine state and external services where possible.

## Pull requests

Use conventional commits such as `fix: preserve transaction writes during AOF replay`, `test: cover fragmented RESP replies`, or `docs: correct supported deployment options`.

Describe the concrete problem, the resulting behavior and the commands used to validate the change. Keep patches focused. Preserve unrelated working-tree changes.

## Releases

The active [release workflow](.github/workflows/release.yml) runs tests, builds a platform matrix, publishes container images to GHCR and creates a GitHub release with checksums. Stable versions update `latest`; prereleases do not.

Releases are triggered by a `v*` tag or the workflow's manual version input. The GoReleaser configuration is [archived](docs/archive/release/goreleaser.yml); automatic Homebrew, Scoop, Snap, package-format publication and GPG signing are not implemented by the active workflow.

Publishing a release is a separate action from preparing or validating a patch.

## License

CacheStorm is released under the [MIT License](LICENSE). By contributing, you agree that your contributions are licensed under the same terms.
