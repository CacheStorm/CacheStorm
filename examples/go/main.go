package main

import (
	"context"
	"fmt"
	"log"
	"time"

	cachestorm "github.com/cachestorm/cachestorm/clients/go"
)

func main() {
	fmt.Println("══════════════════════════════════════════════════════════════")
	fmt.Println("  CacheStorm Go SDK Demo")
	fmt.Println("══════════════════════════════════════════════════════════════")
	fmt.Println()

	// Create client
	client, err := cachestorm.NewClient("localhost:6380")
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// Test connection
	fmt.Println("→ Testing connection...")
	pong, err := client.Ping(ctx)
	if err != nil {
		log.Fatalf("Failed to ping server: %v", err)
	}
	fmt.Printf("  ✓ Connected to CacheStorm (%s)\n", pong)
	fmt.Println()

	// String operations
	fmt.Println("→ String Operations:")

	// SET
	if err := client.Set(ctx, "demo:key", "Hello from Go!", 0); err != nil {
		log.Printf("  ✗ SET failed: %v", err)
	} else {
		fmt.Println("  ✓ SET demo:key 'Hello from Go!'")
	}

	// GET
	val, err := client.Get(ctx, "demo:key")
	if err != nil {
		log.Printf("  ✗ GET failed: %v", err)
	} else {
		fmt.Printf("  ✓ GET demo:key = %s\n", val)
	}

	// SET with expiration
	if err := client.Set(ctx, "demo:temp", "This expires in 10s", 10*time.Second); err != nil {
		log.Printf("  ✗ SET with TTL failed: %v", err)
	} else {
		fmt.Println("  ✓ SET demo:temp with 10s expiration")
	}

	// INCR
	client.Del(ctx, "demo:counter")
	newVal, err := client.Incr(ctx, "demo:counter")
	if err != nil {
		log.Printf("  ✗ INCR failed: %v", err)
	} else {
		fmt.Printf("  ✓ INCR demo:counter = %d\n", newVal)
	}
	fmt.Println()

	// Hash operations
	fmt.Println("→ Hash Operations:")

	// HSET
	hsetResult, err := client.HSet(ctx, "demo:user:1", "name", "John Doe")
	if err != nil {
		log.Printf("  ✗ HSET failed: %v", err)
	} else {
		fmt.Printf("  ✓ HSET demo:user:1 name = %d\n", hsetResult)
	}

	client.HSet(ctx, "demo:user:1", "email", "john@example.com")
	client.HSet(ctx, "demo:user:1", "age", "30")

	// HGET
	name, err := client.HGet(ctx, "demo:user:1", "name")
	if err != nil {
		log.Printf("  ✗ HGET failed: %v", err)
	} else {
		fmt.Printf("  ✓ HGET demo:user:1 name = %s\n", name)
	}

	// HGETALL
	user, err := client.HGetAll(ctx, "demo:user:1")
	if err != nil {
		log.Printf("  ✗ HGETALL failed: %v", err)
	} else {
		fmt.Printf("  ✓ HGETALL demo:user:1 = %v\n", user)
	}
	fmt.Println()

	// List operations
	fmt.Println("→ List Operations:")

	// LPUSH
	lpushResult, err := client.LPush(ctx, "demo:queue", "task1", "task2", "task3")
	if err != nil {
		log.Printf("  ✗ LPUSH failed: %v", err)
	} else {
		fmt.Printf("  ✓ LPUSH demo:queue (3 items) = %d\n", lpushResult)
	}

	// LRANGE
	tasks, err := client.LRange(ctx, "demo:queue", 0, -1)
	if err != nil {
		log.Printf("  ✗ LRANGE failed: %v", err)
	} else {
		fmt.Printf("  ✓ LRANGE demo:queue = %v\n", tasks)
	}

	// LPOP
	task, err := client.LPop(ctx, "demo:queue")
	if err != nil {
		log.Printf("  ✗ LPOP failed: %v", err)
	} else {
		fmt.Printf("  ✓ LPOP demo:queue = %s\n", task)
	}
	fmt.Println()

	// Set operations
	fmt.Println("→ Set Operations:")

	// SADD
	saddResult, err := client.SAdd(ctx, "demo:tags", "go", "redis", "cache")
	if err != nil {
		log.Printf("  ✗ SADD failed: %v", err)
	} else {
		fmt.Printf("  ✓ SADD demo:tags (3 items) = %d\n", saddResult)
	}

	// SMEMBERS
	tags, err := client.SMembers(ctx, "demo:tags")
	if err != nil {
		log.Printf("  ✗ SMEMBERS failed: %v", err)
	} else {
		fmt.Printf("  ✓ SMEMBERS demo:tags = %v\n", tags)
	}

	// SCARD (instead of SISMEMBER which is not in the client)
	tagCount, err := client.SCard(ctx, "demo:tags")
	if err != nil {
		log.Printf("  ✗ SCARD failed: %v", err)
	} else {
		fmt.Printf("  ✓ SCARD demo:tags = %d\n", tagCount)
	}
	fmt.Println()

	// Sorted Set operations
	fmt.Println("→ Sorted Set Operations:")

	// ZADD
	zaddResult, err := client.ZAdd(ctx, "demo:leaderboard", 100, "player1")
	if err != nil {
		log.Printf("  ✗ ZADD failed: %v", err)
	} else {
		fmt.Printf("  ✓ ZADD demo:leaderboard player1:100 = %d\n", zaddResult)
	}
	client.ZAdd(ctx, "demo:leaderboard", 200, "player2")
	client.ZAdd(ctx, "demo:leaderboard", 150, "player3")

	// ZREVRANGE (use ZRevRange instead of ZRange, which is not in the client)
	leaders, err := client.ZRevRange(ctx, "demo:leaderboard", 0, -1)
	if err != nil {
		log.Printf("  ✗ ZREVRANGE failed: %v", err)
	} else {
		fmt.Printf("  ✓ ZREVRANGE demo:leaderboard = %v\n", leaders)
	}
	fmt.Println()

	// CacheStorm-specific: Tags
	fmt.Println("→ CacheStorm Tags (Unique Feature):")

	// Set with tags
	if err := client.SetWithTags(ctx, "demo:product:1", "Product Data", []string{"products", "featured"}); err != nil {
		log.Printf("  ✗ SET with tags failed: %v", err)
	} else {
		fmt.Println("  ✓ SET demo:product:1 with tags [products, featured]")
	}

	client.SetWithTags(ctx, "demo:product:2", "Another Product", []string{"products"})
	client.SetWithTags(ctx, "demo:article:1", "Article Content", []string{"articles", "featured"})

	// Get keys by tag
	productKeys, err := client.TagKeys(ctx, "products")
	if err != nil {
		log.Printf("  ✗ TAGKEYS failed: %v", err)
	} else {
		fmt.Printf("  ✓ TAGKEYS products = %v\n", productKeys)
	}

	// Invalidate by tag
	if err := client.Invalidate(ctx, "featured"); err != nil {
		log.Printf("  ✗ INVALIDATE failed: %v", err)
	} else {
		fmt.Println("  ✓ INVALIDATE featured - keys removed")
	}
	fmt.Println()

	// Pipeline demo
	fmt.Println("→ Pipeline (Batch Operations):")
	pipe := client.Pipeline()
	pipe.Set(ctx, "demo:pipe:1", "value1", 0)
	pipe.Set(ctx, "demo:pipe:2", "value2", 0)
	pipe.Set(ctx, "demo:pipe:3", "value3", 0)
	pipe.Get(ctx, "demo:pipe:1")

	pipeResults, err := pipe.Exec(ctx)
	if err != nil {
		log.Printf("  ✗ Pipeline failed: %v", err)
	} else {
		fmt.Printf("  ✓ Pipeline executed %d commands\n", len(pipeResults))
	}
	fmt.Println()

	// Cleanup
	fmt.Println("→ Cleanup:")
	deleted, err := client.Del(ctx,
		"demo:key", "demo:temp", "demo:counter",
		"demo:user:1", "demo:queue", "demo:tags",
		"demo:leaderboard", "demo:product:1", "demo:product:2",
		"demo:article:1", "demo:pipe:1", "demo:pipe:2", "demo:pipe:3",
	)
	if err != nil {
		log.Printf("  ✗ Cleanup failed: %v", err)
	} else {
		fmt.Printf("  ✓ Deleted %d demo keys\n", deleted)
	}
	fmt.Println()

	fmt.Println("══════════════════════════════════════════════════════════════")
	fmt.Println("  Demo completed successfully!")
	fmt.Println("══════════════════════════════════════════════════════════════")
}
