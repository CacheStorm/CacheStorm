#!/usr/bin/env python3
"""
CacheStorm Python SDK Demo

This demo shows all major features of the CacheStorm Python client.
Run this after starting CacheStorm server.
"""

import sys
import time
import asyncio
from typing import Optional

# Add the client to path (in real usage, install via pip)
sys.path.insert(0, '../../clients/python')

from cachestorm import CacheStormClient, AsyncCacheStormClient
from cachestorm.exceptions import ConnectionError, ResponseError


def print_header(title: str):
    """Print section header."""
    print(f"\n→ {title}")


def print_success(msg: str):
    """Print success message."""
    print(f"  ✓ {msg}")


def print_error(msg: str):
    """Print error message."""
    print(f"  ✗ {msg}")


def demo_sync():
    """Synchronous client demo."""
    print("=" * 64)
    print("  CacheStorm Python SDK Demo (Sync)")
    print("=" * 64)

    # Create client
    client = CacheStormClient(host='localhost', port=6379)

    try:
        # Test connection
        print_header("Testing connection...")
        try:
            client.connect()
            print_success("Connected to CacheStorm")
        except ConnectionError as e:
            print_error(f"Failed to connect: {e}")
            print("\n  Make sure CacheStorm is running:")
            print("    docker run -d -p 6379:6379 cachestorm/cachestorm:latest")
            return

        # String operations
        print_header("String Operations:")

        # SET
        if client.set("demo:key", "Hello from Python!"):
            print_success("SET demo:key 'Hello from Python!'")

        # GET
        value = client.get("demo:key")
        if value:
            print_success(f"GET demo:key = {value.decode()}")

        # SET with expiration
        if client.set("demo:temp", "Expires soon", ex=10):
            print_success("SET demo:temp with 10s expiration")

        # INCR
        client.delete("demo:counter")
        new_val = client.execute_command("INCR", "demo:counter")
        print_success(f"INCR demo:counter = {new_val}")

        # Hash operations
        print_header("Hash Operations:")

        # HSET
        hset_result = client.hset("demo:user:1", "name", "Jane Doe")
        print_success(f"HSET demo:user:1 name = {hset_result}")

        client.hset("demo:user:1", "email", "jane@example.com")
        client.hset("demo:user:1", "age", "25")

        # HGET
        name = client.hget("demo:user:1", "name")
        if name:
            print_success(f"HGET demo:user:1 name = {name.decode()}")

        # HGETALL
        user = client.hgetall("demo:user:1")
        print_success(f"HGETALL demo:user:1 = {user}")

        # List operations
        print_header("List Operations:")

        # LPUSH
        lpush_result = client.lpush("demo:queue", "task1", "task2", "task3")
        print_success(f"LPUSH demo:queue (3 items) = {lpush_result}")

        # LRANGE
        tasks = client.lrange("demo:queue", 0, -1)
        print_success(f"LRANGE demo:queue = {[t.decode() for t in tasks]}")

        # LPOP
        task = client.lpop("demo:queue")
        if task:
            print_success(f"LPOP demo:queue = {task.decode()}")

        # Set operations
        print_header("Set Operations:")

        # SADD
        sadd_result = client.sadd("demo:tags", "python", "redis", "cache")
        print_success(f"SADD demo:tags (3 items) = {sadd_result}")

        # SMEMBERS
        tags = client.smembers("demo:tags")
        print_success(f"SMEMBERS demo:tags = {[t.decode() for t in tags]}")

        # SISMEMBER
        is_member = client.sismember("demo:tags", "python")
        print_success(f"SISMEMBER demo:tags python = {is_member}")

        # Sorted Set operations
        print_header("Sorted Set Operations:")

        # ZADD using execute_command for complex operations
        client.execute_command("ZADD", "demo:leaderboard", "100", "alice")
        client.execute_command("ZADD", "demo:leaderboard", "200", "bob")
        client.execute_command("ZADD", "demo:leaderboard", "150", "charlie")
        print_success("ZADD demo:leaderboard (3 players)")

        # ZRANGE
        leaders = client.zrange("demo:leaderboard", 0, -1)
        print_success(f"ZRANGE demo:leaderboard = {[l.decode() for l in leaders]}")

        # CacheStorm-specific: Tags
        print_header("CacheStorm Tags (Unique Feature):")

        # Set with tags
        if client.set("demo:product:1", "Product Data", tags=["products", "featured"]):
            print_success("SET demo:product:1 with tags [products, featured]")

        client.set("demo:product:2", "Another Product", tags=["products"])
        client.set("demo:article:1", "Article Content", tags=["articles", "featured"])

        # Get keys by tag
        product_keys = client.tag_keys("products")
        print_success(f"TAGKEYS products = {product_keys}")

        # Get tags of a key
        key_tags = client.tags("demo:product:1")
        print_success(f"TAGS demo:product:1 = {key_tags}")

        # Invalidate by tag
        invalidated = client.invalidate("featured")
        print_success(f"INVALIDATE featured = {invalidated} keys removed")

        # Pipeline demo
        print_header("Pipeline (Batch Operations):")

        with client.pipeline() as pipe:
            pipe.set("demo:pipe:1", "value1")
            pipe.set("demo:pipe:2", "value2")
            pipe.set("demo:pipe:3", "value3")
            pipe.get("demo:pipe:1")

            results = pipe.execute()
            print_success(f"Pipeline executed {len(results)} commands")

        # Pub/Sub demo (brief)
        print_header("Pub/Sub (brief demo):")
        print("  Starting pub/sub demo...")

        # Create pub/sub in background
        import threading

        def subscriber():
            """Background subscriber."""
            try:
                pubsub = client.pubsub()
                pubsub.subscribe("demo:channel")

                # Get one message
                msg = pubsub.get_message(timeout=2)
                if msg:
                    print_success(f"Received: {msg}")

                pubsub.close()
            except Exception as e:
                print_error(f"Subscriber error: {e}")

        # Start subscriber
        sub_thread = threading.Thread(target=subscriber, daemon=True)
        sub_thread.start()
        time.sleep(0.5)  # Let subscriber connect

        # Publish message
        published = client.publish("demo:channel", "Hello from pub/sub!")
        print_success(f"Published message to demo:channel ({published} subscribers)")

        time.sleep(0.5)  # Let message be received

        # Cleanup
        print_header("Cleanup:")
        deleted = client.delete(
            "demo:key", "demo:temp", "demo:counter",
            "demo:user:1", "demo:queue", "demo:tags",
            "demo:leaderboard", "demo:product:1", "demo:product:2",
            "demo:article:1", "demo:pipe:1", "demo:pipe:2", "demo:pipe:3"
        )
        print_success(f"Deleted {deleted} demo keys")

        print("\n" + "=" * 64)
        print("  Sync Demo completed successfully!")
        print("=" * 64)

    finally:
        client.close()


async def demo_async():
    """Asynchronous client demo."""
    print("\n" + "=" * 64)
    print("  CacheStorm Python SDK Demo (Async)")
    print("=" * 64)

    client = AsyncCacheStormClient(host='localhost', port=6379)

    try:
        print_header("Testing async connection...")
        try:
            await client.connect()
            print_success("Connected to CacheStorm (async)")
        except ConnectionError as e:
            print_error(f"Failed to connect: {e}")
            return

        # Async operations
        print_header("Async Operations:")

        # SET
        if await client.set("demo:async:key", "Hello from async Python!"):
            print_success("SET demo:async:key")

        # GET
        value = await client.get("demo:async:key")
        if value:
            print_success(f"GET demo:async:key = {value.decode()}")

        # Concurrent operations
        print_header("Concurrent Operations:")

        async def set_key(i: int):
            await client.set(f"demo:concurrent:{i}", f"value-{i}")

        # Run 10 concurrent SET operations
        await asyncio.gather(*[set_key(i) for i in range(10)])
        print_success("Executed 10 concurrent SET operations")

        # Cleanup
        print_header("Cleanup:")
        await client.delete("demo:async:key")
        for i in range(10):
            await client.delete(f"demo:concurrent:{i}")
        print_success("Cleaned up async demo keys")

        print("\n" + "=" * 64)
        print("  Async Demo completed successfully!")
        print("=" * 64)

    finally:
        await client.close()


def main():
    """Main demo function."""
    # Run sync demo
    demo_sync()

    # Run async demo
    print("\n")
    asyncio.run(demo_async())

    print("\n" + "=" * 64)
    print("  All demos completed!")
    print("=" * 64)


if __name__ == "__main__":
    main()
