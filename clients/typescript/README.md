# CacheStorm TypeScript Client

Official TypeScript/JavaScript client for CacheStorm - High-Performance Redis-Compatible Database.

## Features

- Full TypeScript support with type definitions
- Promise-based API over a single pipelined connection
- AUTH and SELECT on connect (`password`, `database` options)
- Reconnects on the next command after the connection drops, replaying AUTH and SELECT (`reconnect`, default `true`; `reconnectAttempts`, default 3, with exponential backoff; emits `reconnect`). Commands that were in flight when it dropped fail and are not resent. Pub/Sub connections do not reconnect
- Pipeline support (one write, replies in order)
- Pub/Sub on a dedicated connection (SUBSCRIBE, PSUBSCRIBE)
- Binary-safe reads with `getBuffer`
- CacheStorm-specific features (tags, invalidation)

Not implemented: connection pooling, retrying failed commands and namespaces. The `pool`, `retry` and `namespace` options throw instead of being silently ignored. Bulk replies are decoded as UTF-8 strings except through `getBuffer`, and integers are returned as JavaScript numbers. See the [verified review and roadmap](../../docs/PROJECT_REVIEW.md).

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

### Authentication and database

```typescript
const client = new CacheStormClient({ host: 'localhost', port: 6380, password: 'secret', database: 2 });
await client.connect();
```

`connect()` rejects if AUTH or SELECT fails.

### Pipeline

```typescript
const pipeline = client.pipeline();
pipeline.set('key1', 'value1');
pipeline.set('key2', 'value2');
pipeline.get('key1');

const results = await pipeline.exec();
```

If any reply is a server error, `exec()` rejects with `PipelineError`; its `results` array holds every reply, with errors as `ReplyError`.

### Pub/Sub

```typescript
const subscription = client.subscribe('channel', (message) => {
  console.log('Received:', message.channel, message.payload);
});
await subscription.subscribe();
await subscription.psubscribe('news.*');

await client.publish('channel', 'Hello!');

await subscription.quit();
```

The subscription opens its own connection; the original client stays usable for regular commands.

### CacheStorm-Specific Features

```typescript
// Value, expiry and tags in one atomic write
await client.set('user:1', userData, { EX: 300, TAGS: ['user:1', 'users'] });
await client.setWithTags('report', data, ['reports']); // no expiry

// Tag an existing key, or untag it; each resolves to how many tags changed
await client.addTags('session:9', 'user:1');
await client.removeTags('session:9', 'user:1');

// Delete every key of the tags, atomically; resolves to how many were deleted
await client.invalidate('user:1', 'users');

// Tag hierarchy: invalidateCascade also reaches the tags linked below
await client.tagLink('catalog', 'category:shoes');
await client.invalidateCascade('catalog');
await client.tagChildren('catalog');
await client.tagUnlink('catalog', 'category:shoes');

// Inspect
await client.tagKeys('users');
await client.tagCount('users');
await client.tags('user:1');
```

## API Reference

See the [client implementation](./src/client.ts) for the available API and the [project review](../../docs/PROJECT_REVIEW.md) for current limitations.
