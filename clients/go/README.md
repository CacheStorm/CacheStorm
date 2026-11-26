# CacheStorm Go Client

Official Go client for CacheStorm - High-Performance Redis-Compatible Database.

## Features

Behavior notes: commands after `Close` return `ErrClosed`; server error replies are returned as `RedisError`; pipelines send all queued commands in one write on one connection; Pub/Sub uses a dedicated connection. `MinIdleConns` and `MaxRetries` are accepted but not used (commands are never retried). A pooled connection that the server closed while it was idle (timeout, `CLIENT KILL`, restart) is detected on Unix systems and replaced before a command is sent on it. See the [verified review and roadmap](../../docs/PROJECT_REVIEW.md).

- Full Redis protocol compatibility
- Connection pooling
- Pipeline support
- Pub/Sub support
- Automatic failover
- Type-safe commands
- Context support

## Installation

```bash
go get github.com/cachestorm/cachestorm/clients/go
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"

    cachestorm "github.com/cachestorm/cachestorm/clients/go"
)

func main() {
    // Create client
    client, err := cachestorm.NewClient("localhost:6380")
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    ctx := context.Background()

    // Basic operations
    err = client.Set(ctx, "key", "value", 0)
    if err != nil {
        log.Fatal(err)
    }

    val, err := client.Get(ctx, "key")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(val) // "value"
}
```

## Advanced Usage

### Connection Pool

```go
client, err := cachestorm.NewClient("localhost:6380",
    cachestorm.WithPoolSize(10),
    cachestorm.WithMinIdleConns(5),
    cachestorm.WithMaxRetries(3),
)
```

### Pipeline

```go
pipe := client.Pipeline()
pipe.Set(ctx, "key1", "value1", 0)
pipe.Set(ctx, "key2", "value2", 0)
pipe.Get(ctx, "key1")

cmders, err := pipe.Exec(ctx)
```

### Pub/Sub

```go
pubsub := client.Subscribe(ctx, "channel")
defer pubsub.Close()

msg, err := pubsub.ReceiveMessage(ctx)
```

### CacheStorm-Specific Features

```go
// Value, expiry and tags in one atomic write
err := client.SetWithTagsTTL(ctx, "user:1", data, 5*time.Minute, []string{"user:1", "users"})
err = client.SetWithTags(ctx, "report", data, []string{"reports"}) // no expiry

// Tag an existing key, or untag it; each returns how many tags changed
added, err := client.AddTags(ctx, "session:9", "user:1")
removed, err := client.RemoveTags(ctx, "session:9", "user:1")

// Delete every key of the tags, atomically; how many were deleted
n, err := client.InvalidateCount(ctx, "user:1", "users")

// Tag hierarchy: InvalidateCascade also reaches the tags linked below
err = client.TagLink(ctx, "catalog", "category:shoes")
n, err = client.InvalidateCascade(ctx, "catalog")
children, err := client.TagChildren(ctx, "catalog")
err = client.TagUnlink(ctx, "catalog", "category:shoes")

// Inspect
keys, err := client.TagKeys(ctx, "users")
count, err := client.TagCount(ctx, "users")
tags, err := client.Tags(ctx, "user:1")
```

## API Reference

See the [client implementation](./client.go) for the available API and the [project review](../../docs/PROJECT_REVIEW.md) for current limitations.
