# CacheStorm Examples

These examples exercise the repository clients. Known client and module limitations are recorded in the [project review](../docs/PROJECT_REVIEW.md); the examples do not establish full Redis compatibility.

## Start the server

From the repository root:

```bash
go run ./cmd/cachestorm --config config/cachestorm.example.yaml
```

Alternatively, from the `examples/` directory:

```bash
docker compose -f docker/docker-compose.simple.yml up -d --build
```

The default RESP port is 6380. Check it with `redis-cli -p 6380 PING` if Redis CLI is installed. The admin UI is served on port 8080.

## Go

From `examples/`:

```bash
cd go
go mod tidy
go run main.go
```

Use the root project's Go toolchain requirement. Each Go example is a separate module; its existing Go 1.22 metadata is stale relative to the root module and currently fails a readonly build. Run `go mod tidy` in an example before building it. The Go client's Pub/Sub receive operation is not implemented, so examples requiring message delivery need the SDK work in the roadmap.

## Python

From `examples/`:

```bash
cd python
pip install -r requirements.txt
python demo.py
```

The requirements file installs a published client package. To exercise the checked-out client instead, run with `PYTHONPATH=../../clients/python python demo.py` in the same directory. The repository Python client currently has confirmed non-ASCII/binary encoding and async initialization defects; use these examples with that limitation in mind.

## TypeScript

From `examples/`, build the local package before running the demo:

```bash
cd ../clients/typescript
npm install
npm run build
cd ../../examples/typescript
npm install
npm run demo
```

The TypeScript example references the repository client through a local file dependency. Its pipeline currently returns success without sending commands, and the protocol decoder does not correctly handle fragmented responses. Pub/Sub transport is also unfinished.

## Other examples

`analytics/`, `chat/`, `e-commerce/` and `realtime/` contain additional Go programs. They share the Go module and SDK limitations above and are not included in the root test command.

The root [Compose file](../docker-compose.yml) has optional `monitoring` and `gui` profiles. The production monitoring authentication setup still needs reconciliation with the server configuration.

## References

- [Documentation](../docs/README.md)
- [Command reference](../docs/commands.md)
- [Go client](../clients/go/README.md)
- [Python client](../clients/python/README.md)
- [TypeScript client](../clients/typescript/README.md)
