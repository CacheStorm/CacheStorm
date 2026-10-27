# CacheStorm Documentation

Start with the installation and configuration guides. The [project review](./PROJECT_REVIEW.md) records verified limitations, the [roadmap](../ROADMAP.md) defines the next milestones, [benchmarks](./BENCHMARKS.md) records measured throughput, and the [compatibility report](../tests/compat/REPORT.md) lists the measured differences from Redis 7.4.

## Guides

- [Getting started](./01-getting-started.md)
- [Configuration](./02-configuration.md)
- [Command reference](./commands.md)
- [Command examples](./commands-detailed.md)
- [Tag-based invalidation](./commands.md#tag-based-invalidation-commands)
- [HTTP API](./07-http-api.md)
- [Admin UI](./08-admin-ui.md)

The command reference and examples serve different purposes; neither is a Redis conformance certificate. The production router registered **1,596 unique command names** during the 2026-10-20 review. Some commands have incomplete behavior; see the project review before relying on transactions, persistence, SDKs or distributed operation.

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

Client capabilities differ. Consult the project review for the confirmed pipeline, protocol, async and lifecycle defects.

## Historical material

[Archived designs and release configuration](./archive/README.md) describe earlier plans, rather than current support commitments. Missing numbered guides are no longer listed as existing documentation.

CacheStorm is released under the [MIT License](../LICENSE).
