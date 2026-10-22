# CacheStorm Python Client

Official Python client for CacheStorm - High-Performance Redis-Compatible Database.

## Features

Current limitations: binary and non-ASCII command encoding is incorrect, and the async client's default connection path fails during initialization. Packaging and supported Python versions are not defined in this repository. See the [verified review and roadmap](../../docs/PROJECT_REVIEW.md).

- Redis protocol client with known encoding limitations
- Connection pooling
- Pipeline support
- Pub/Sub support
- Async/await support (asyncio)
- Type hints
- CacheStorm-specific features (tags, invalidation)

## Installation

```bash
pip install cachestorm
# or
poetry add cachestorm
# or
conda install -c conda-forge cachestorm
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

# Namespace support
client.set('key', 'value', namespace='tenant1')
```

## API Reference

See the [client implementation](./cachestorm/client.py) for the available API and the [project review](../../docs/PROJECT_REVIEW.md) for current limitations.
