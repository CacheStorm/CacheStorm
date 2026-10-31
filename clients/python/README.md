# CacheStorm Python Client

Official Python client for CacheStorm - High-Performance Redis-Compatible Database.

## Features

Requires Python 3.9+. See the [verified review and roadmap](../../docs/PROJECT_REVIEW.md) for remaining limitations.

- Binary-safe RESP protocol client (UTF-8 strings, raw bytes)
- Connection pooling
- Pipeline support
- Pub/Sub support
- Async/await support (asyncio)
- Type hints
- CacheStorm-specific features (tags, invalidation)

## Installation

The client is not published to a package index. Install it from a local checkout or from git:

```bash
pip install ./clients/python
# or
pip install "git+https://github.com/CacheStorm/CacheStorm.git#subdirectory=clients/python"
```

## Quick Start

```python
from cachestorm import CacheStormClient

# Create client
client = CacheStormClient(host='localhost', port=6380)

# Connect
client.connect()

# Basic operations
client.set('key', 'value')
value = client.get('key')
print(value)  # b'value'

# With expiration
client.set('temp', 'data', ex=60)

# Close
client.close()
```

## Async Usage

```python
import asyncio
from cachestorm import AsyncCacheStormClient

async def main():
    client = AsyncCacheStormClient(host='localhost', port=6380)
    await client.connect()

    await client.set('key', 'value')
    value = await client.get('key')
    print(value)

    await client.close()

asyncio.run(main())
```

## Advanced Usage

### Connection Pool

```python
from cachestorm import ConnectionPool

pool = ConnectionPool(
    host='localhost',
    port=6380,
    max_connections=20,
    min_connections=5,
)

client = CacheStormClient(connection_pool=pool)
```

### Pipeline

```python
with client.pipeline() as pipe:
    pipe.set('key1', 'value1')
    pipe.set('key2', 'value2')
    pipe.get('key1')
    results = pipe.execute()
```

### Pub/Sub

```python
pubsub = client.pubsub()
pubsub.subscribe('channel')

for message in pubsub.listen():
    print(message)
```

### CacheStorm-Specific Features

```python
# Tag-based invalidation
client.set('user:1', user_data, tags=['user', 'session'])

# Invalidate by tag
client.invalidate('user')

# Get keys by tag
keys = client.tag_keys('user')
```

## Known Limitations

- Pub/Sub does not reconnect. If its connection drops, the listener stops; create a new `PubSub` and subscribe again. Send failures raise `ConnectionError`.
- The client reconnects on the next command after its connection drops, running `AUTH`/`SELECT` again. A connection the server closed while idle is detected and replaced before the command is sent. A command whose connection breaks while it is in flight raises `ConnectionError` and is not resent.
- Pipelines and Pub/Sub use the connection pool, not the client's own socket. Pooled connections run `AUTH`/`SELECT` with the client's `password`/`db` when created and are health-checked on checkout.
- Only RESP2 replies are supported.

## API Reference

See the [client implementation](./cachestorm/client.py) for the available API and the [project review](../../docs/PROJECT_REVIEW.md) for current limitations.

## Running Tests

```bash
cd clients/python
python3 -m unittest discover -s tests -v
# or
pytest
```
