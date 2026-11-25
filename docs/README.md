# CacheStorm Documentation

Start with the installation and configuration guides. The [roadmap](../ROADMAP.md) defines the next milestones, [benchmarks](./BENCHMARKS.md) records measured throughput, and the [compatibility report](../tests/compat/REPORT.md) lists the measured differences from Redis 7.4.

## Guides

- [Getting started](./01-getting-started.md)
- [Configuration](./02-configuration.md)
- [Command reference](./commands.md)
- [Command examples](./commands-detailed.md)
- [Tag-based invalidation](./commands.md#tag-based-invalidation-commands) ([overview](../README.md#tag-based-invalidation))
- [Running a cluster](./09-cluster.md)
- [HTTP API](./07-http-api.md)
- [Admin UI](./08-admin-ui.md)

The command reference and examples serve different purposes; neither is a Redis conformance certificate. The server registers 1,238 command names; [the compatibility report](../tests/compat/REPORT.md) lists the measured differences from Redis 7.4. The [project review](./PROJECT_REVIEW.md) is the October 2026 audit that the roadmap's milestones M0–M5 resolved.

## Development and operations

- [Contribution guide](../CONTRIBUTING.md)
- [Measured test coverage](../COVERAGE_REPORT.md)
- [Security policy](../SECURITY.md)
- [Configuration template](../config/cachestorm.example.yaml)
- [Production configuration template](../config/cachestorm.production.yaml)
- [Container deployment](../docker-compose.yml)
- [Release workflow](../.github/workflows/release.yml)
- [Examples](../examples/README.md)
- [Version history](../CHANGELOG.md)

## Clients

- [Go](../clients/go/README.md)
- [Python](../clients/python/README.md)
- [TypeScript](../clients/typescript/README.md)

Client capabilities differ; each client's README lists its limitations.

## Historical material

[Archived designs and release configuration](./archive/README.md) describe earlier plans, rather than current support commitments. Missing numbered guides are no longer listed as existing documentation.

CacheStorm is released under the [MIT License](../LICENSE).
