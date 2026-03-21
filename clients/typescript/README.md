# CacheStorm TypeScript Client

Official TypeScript/JavaScript client for CacheStorm - High-Performance Redis-Compatible Database.

## Features

- Full TypeScript support with type definitions
- Promise-based API
- Connection pooling
- Pipeline support
- Pub/Sub support
- Automatic reconnection
- CacheStorm-specific features (tags, invalidation)

## Installation

```bash
npm install @cachestorm/client
# or
yarn add @cachestorm/client
# or
pnpm add @cachestorm/client
```

## Quick Start

```typescript
import { CacheStormClient } from '@cachestorm/client';

const client = new CacheStormClient({
  host: 'localhost',
  port: 6380,
});

await client.connect();

// Basic operations
await client.set('key', 'value');
const value = await client.get('key');
console.log(value); // 'value'

// With expiration
await client.set('temp', 'data', { EX: 60 });

// Close connection
await client.quit();
```

## Advanced Usage

### Connection Pool

```typescript
const client = new CacheStormClient({
  host: 'localhost',
  port: 6380,
  pool: {
    min: 5,
    max: 20,
  },
  retry: {
    maxRetries: 3,
    retryDelay: 100,
  },
});
```

### Pipeline

```typescript
const pipeline = client.pipeline();
pipeline.set('key1', 'value1');
pipeline.set('key2', 'value2');
pipeline.get('key1');

const results = await pipeline.exec();
```

### Pub/Sub

```typescript
const subscriber = client.duplicate();
await subscriber.connect();

await subscriber.subscribe('channel', (message) => {
  console.log('Received:', message);
});

// Publish
await client.publish('channel', 'Hello!');
```

### CacheStorm-Specific Features

```typescript
// Tag-based invalidation
await client.setWithTags('user:1', userData, ['user', 'session']);

// Invalidate by tag
await client.invalidate('user');

// Get keys by tag
const keys = await client.tagKeys('user');

// Namespace support
await client.set('key', 'value', { namespace: 'tenant1' });
```

## API Reference

See [API Documentation](./docs/api.md) for complete reference.
