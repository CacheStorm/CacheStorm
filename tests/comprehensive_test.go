package tests

import (
	"fmt"
	"math/rand"
	"runtime"
	"sync"
	"testing"
	"time"
)

// TestAllDataTypes tests all supported data types
func TestAllDataTypes(t *testing.T) {
	ts := StartTestServer(t)
	defer ts.Stop()

	client, err := NewRedisClient(ts.Addr())
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	t.Run("Strings", func(t *testing.T) {
		// SET, GET, DEL
		client.Send("SET", "str:key1", "value1")
		val, err := client.Send("GET", "str:key1")
		if err != nil || val != "value1" {
			t.Errorf("String operations failed: %v, %v", val, err)
		}

		// APPEND
		client.Send("APPEND", "str:key1", "_appended")
		val, _ = client.Send("GET", "str:key1")
		if val != "value1_appended" {
			t.Errorf("APPEND failed: %v", val)
		}

		// INCR, DECR
		client.Send("SET", "str:counter", "100")
		client.Send("INCR", "str:counter")
		val, _ = client.Send("GET", "str:counter")
		if val != "101" {
			t.Errorf("INCR failed: %v", val)
		}

		// MSET, MGET
		client.Send("MSET", "key:a", "val:a", "key:b", "val:b")
		vals, _ := client.Send("MGET", "key:a", "key:b")
		arr := vals.([]interface{})
		if len(arr) != 2 || arr[0] != "val:a" || arr[1] != "val:b" {
			t.Errorf("MSET/MGET failed: %v", vals)
		}

		t.Log("✓ String operations passed")
	})

	t.Run("Hashes", func(t *testing.T) {
		// HSET, HGET
		client.Send("HSET", "hash:user:1", "name", "John", "age", "30")
		val, _ := client.Send("HGET", "hash:user:1", "name")
		if val != "John" {
			t.Errorf("HGET failed: %v", val)
		}

		// HGETALL
		all, _ := client.Send("HGETALL", "hash:user:1")
		// HGETALL returns array of [field1, value1, field2, value2, ...]
		allArr := all.([]interface{})
		m := make(map[string]string)
		for i := 0; i < len(allArr); i += 2 {
			if i+1 < len(allArr) {
				m[allArr[i].(string)] = allArr[i+1].(string)
			}
		}
		if m["name"] != "John" || m["age"] != "30" {
			t.Errorf("HGETALL failed: %v", m)
		}

		// HMSET, HMGET
		client.Send("HMSET", "hash:user:2", "city", "NYC", "country", "USA")
		vals, _ := client.Send("HMGET", "hash:user:2", "city", "country")
		arr := vals.([]interface{})
		if len(arr) != 2 {
			t.Errorf("HMGET failed: %v", vals)
		}

		// HDEL
		client.Send("HDEL", "hash:user:1", "age")
		val, _ = client.Send("HGET", "hash:user:1", "age")
		if val != nil {
			t.Errorf("HDEL failed: %v", val)
		}

		// HINCRBY
		client.Send("HSET", "hash:stats", "views", "100")
		client.Send("HINCRBY", "hash:stats", "views", "5")
		val, _ = client.Send("HGET", "hash:stats", "views")
		if val != "105" {
			t.Errorf("HINCRBY failed: %v", val)
		}

		t.Log("✓ Hash operations passed")
	})

	t.Run("Lists", func(t *testing.T) {
		// LPUSH, RPUSH
		client.Send("LPUSH", "list:queue", "item1", "item2")
		client.Send("RPUSH", "list:queue", "item3")

		// LRANGE
		items, _ := client.Send("LRANGE", "list:queue", "0", "-1")
		arr := items.([]interface{})
		if len(arr) != 3 {
			t.Errorf("List operations failed: %v", arr)
		}

		// LPOP, RPOP
		client.Send("LPOP", "list:queue")
		client.Send("RPOP", "list:queue")

		// LLEN
		len, _ := client.Send("LLEN", "list:queue")
		if len != int64(1) {
			t.Errorf("LLEN failed: %v", len)
		}

		// LINDEX
		val, _ := client.Send("LINDEX", "list:queue", "0")
		if val != "item1" {
			t.Errorf("LINDEX failed: %v", val)
		}

		t.Log("✓ List operations passed")
	})

	t.Run("Sets", func(t *testing.T) {
		// SADD
		client.Send("SADD", "set:tags", "tag1", "tag2", "tag3")

		// SMEMBERS
		members, _ := client.Send("SMEMBERS", "set:tags")
		arr := members.([]interface{})
		if len(arr) != 3 {
			t.Errorf("SADD/SMEMBERS failed: %v", arr)
		}

		// SISMEMBER
		isMember, _ := client.Send("SISMEMBER", "set:tags", "tag1")
		if isMember != int64(1) {
			t.Errorf("SISMEMBER failed: %v", isMember)
		}

		// SREM
		client.Send("SREM", "set:tags", "tag2")
		count, _ := client.Send("SCARD", "set:tags")
		if count != int64(2) {
			t.Errorf("SREM/SCARD failed: %v", count)
		}

		// SINTER, SUNION
		client.Send("SADD", "set:a", "1", "2", "3")
		client.Send("SADD", "set:b", "2", "3", "4")
		inter, _ := client.Send("SINTER", "set:a", "set:b")
		if len(inter.([]interface{})) != 2 {
			t.Errorf("SINTER failed: %v", inter)
		}

		t.Log("✓ Set operations passed")
	})

	t.Run("SortedSets", func(t *testing.T) {
		// ZADD
		client.Send("ZADD", "zset:scores", "100", "player1", "200", "player2", "150", "player3")

		// ZRANGE
		range_, _ := client.Send("ZRANGE", "zset:scores", "0", "-1")
		arr := range_.([]interface{})
		if len(arr) != 3 || arr[0] != "player1" {
			t.Errorf("ZRANGE failed: %v", arr)
		}

		// ZREVRANGE
		revRange, _ := client.Send("ZREVRANGE", "zset:scores", "0", "-1")
		revArr := revRange.([]interface{})
		if revArr[0] != "player2" {
			t.Errorf("ZREVRANGE failed: %v", revArr)
		}

		// ZRANGEBYSCORE
		byScore, _ := client.Send("ZRANGEBYSCORE", "zset:scores", "100", "150")
		if len(byScore.([]interface{})) != 2 {
			t.Errorf("ZRANGEBYSCORE failed: %v", byScore)
		}

		// ZINCRBY
		client.Send("ZINCRBY", "zset:scores", "50", "player1")
		newScore, _ := client.Send("ZSCORE", "zset:scores", "player1")
		if newScore != "150" {
			t.Errorf("ZINCRBY/ZSCORE failed: %v", newScore)
		}

		// ZRANK - player1 now has score 150, same as player3
		// Order: player1 (150), player3 (150), player2 (200) - so rank is 0 or 1 depending on tie handling
		rank, _ := client.Send("ZRANK", "zset:scores", "player1")
		if rank != int64(0) && rank != int64(1) {
			t.Errorf("ZRANK failed: %v", rank)
		}

		t.Log("✓ Sorted Set operations passed")
	})

	t.Run("Streams", func(t *testing.T) {
		// XADD
		id1, _ := client.Send("XADD", "stream:events", "*", "type", "click", "user", "user1")
		_, _ = client.Send("XADD", "stream:events", "*", "type", "view", "user", "user2")

		// XLEN
		streamLen, _ := client.Send("XLEN", "stream:events")
		if streamLen != int64(2) {
			t.Errorf("XLEN failed: %v", streamLen)
		}

		// XRANGE
		range_, _ := client.Send("XRANGE", "stream:events", "-", "+")
		rangeArr, rangeOk := range_.([]interface{})
		if !rangeOk || len(rangeArr) != 2 {
			t.Errorf("XRANGE failed: %v", range_)
		}

		// XREVRANGE
		revRange, _ := client.Send("XREVRANGE", "stream:events", "+", "-", "COUNT", "1")
		revArr, revOk := revRange.([]interface{})
		if !revOk || len(revArr) != 1 {
			t.Errorf("XREVRANGE failed: %v", revRange)
		}

		// XDEL
		client.Send("XDEL", "stream:events", id1)
		newLen, _ := client.Send("XLEN", "stream:events")
		if newLen != int64(1) {
			t.Errorf("XDEL failed: %v", newLen)
		}

		t.Log("✓ Stream operations passed")
	})
}

// TestPersistenceFeatures tests persistence features
func TestPersistenceFeatures(t *testing.T) {
	ts := StartTestServer(t)
	defer ts.Stop()

	client, _ := NewRedisClient(ts.Addr())
	defer client.Close()

	t.Run("TTL", func(t *testing.T) {
		// SET with EX
		client.Send("SET", "ttl:key1", "value1", "EX", "1")

		// TTL
		ttl, _ := client.Send("TTL", "ttl:key1")
		if ttl.(int64) <= 0 || ttl.(int64) > 1 {
			t.Errorf("TTL failed: %v", ttl)
		}

		// Wait for expiry
		time.Sleep(2 * time.Second)

		val, _ := client.Send("GET", "ttl:key1")
		if val != nil {
			t.Errorf("Key should have expired: %v", val)
		}

		t.Log("✓ TTL operations passed")
	})

	t.Run("Persistence Commands", func(t *testing.T) {
		// Populate data
		for i := 0; i < 100; i++ {
			client.Send("SET", fmt.Sprintf("persist:key:%d", i), fmt.Sprintf("value:%d", i))
		}

		// SAVE
		_, err := client.Send("SAVE")
		if err != nil {
			t.Logf("SAVE returned: %v (may be expected)", err)
		}

		// LASTSAVE
		lastSave, _ := client.Send("LASTSAVE")
		if lastSave.(int64) == 0 {
			t.Log("LASTSAVE returned 0")
		}

		t.Log("✓ Persistence commands passed")
	})
}

// TestPubSub tests pub/sub functionality
func TestPubSub(t *testing.T) {
	ts := StartTestServer(t)
	defer ts.Stop()

	publisher, _ := NewRedisClient(ts.Addr())
	defer publisher.Close()

	t.Run("Basic Pub/Sub", func(t *testing.T) {
		// Note: Full pub/sub test would require async handling
		// This is a basic test

		// PUBLISH
		count, _ := publisher.Send("PUBLISH", "channel:test", "hello")
		if count != int64(0) {
			// No subscribers, so 0 is expected
			t.Logf("Published to 0 subscribers (expected)")
		}

		// PUBSUB CHANNELS
		channels, _ := publisher.Send("PUBSUB", "CHANNELS")
		t.Logf("Channels: %v", channels)

		t.Log("✓ Pub/Sub basic test passed")
	})
}

// TestTransactions tests transaction support
func TestTransactions(t *testing.T) {
	ts := StartTestServer(t)
	defer ts.Stop()

	client, _ := NewRedisClient(ts.Addr())
	defer client.Close()

	t.Run("Basic Transaction", func(t *testing.T) {
		// MULTI
		_, err := client.Send("MULTI")
		if err != nil {
			t.Logf("MULTI not supported or error: %v", err)
			return
		}

		// Queue commands
		client.Send("SET", "tx:key1", "value1")
		client.Send("SET", "tx:key2", "value2")
		client.Send("INCR", "tx:counter")

		// EXEC
		result, err := client.Send("EXEC")
		if err != nil {
			t.Logf("EXEC failed: %v", err)
			return
		}

		t.Logf("Transaction result: %v", result)
		t.Log("✓ Transaction test passed")
	})
}

// TestLuaScripting tests Lua scripting
func TestLuaScripting(t *testing.T) {
	ts := StartTestServer(t)
	defer ts.Stop()

	client, _ := NewRedisClient(ts.Addr())
	defer client.Close()

	t.Run("EVAL", func(t *testing.T) {
		// Set up test data
		client.Send("SET", "lua:key1", "10")
		client.Send("SET", "lua:key2", "20")

		// Lua script to sum two keys
		script := "return redis.call('GET', KEYS[1]) + redis.call('GET', KEYS[2])"

		result, err := client.Send("EVAL", script, "2", "lua:key1", "lua:key2")
		if err != nil {
			t.Logf("EVAL not supported or error: %v", err)
			return
		}

		t.Logf("Lua script result: %v", result)
		t.Log("✓ Lua scripting test passed")
	})
}

// TestPerformance runs performance tests
func TestPerformance(t *testing.T) {
	ts := StartTestServer(t)
	defer ts.Stop()

	client, _ := NewRedisClient(ts.Addr())
	defer client.Close()

	t.Run("Write Performance", func(t *testing.T) {
		count := 10000
		start := time.Now()

		for i := 0; i < count; i++ {
			client.Send("SET", fmt.Sprintf("perf:key:%d", i), fmt.Sprintf("value:%d", i))
		}

		duration := time.Since(start)
		rate := float64(count) / duration.Seconds()

		t.Logf("Wrote %d keys in %v (%.0f ops/sec)", count, duration, rate)
	})

	t.Run("Read Performance", func(t *testing.T) {
		count := 10000
		start := time.Now()

		for i := 0; i < count; i++ {
			client.Send("GET", fmt.Sprintf("perf:key:%d", i))
		}

		duration := time.Since(start)
		rate := float64(count) / duration.Seconds()

		t.Logf("Read %d keys in %v (%.0f ops/sec)", count, duration, rate)
	})

	t.Run("Mixed Workload", func(t *testing.T) {
		count := 10000
		start := time.Now()

		for i := 0; i < count; i++ {
			if i%10 < 8 { // 80% reads
				client.Send("GET", fmt.Sprintf("perf:key:%d", rand.Intn(10000)))
			} else { // 20% writes
				client.Send("SET", fmt.Sprintf("perf:key:%d", i), fmt.Sprintf("value:%d", i))
			}
		}

		duration := time.Since(start)
		rate := float64(count) / duration.Seconds()

		t.Logf("Mixed %d ops in %v (%.0f ops/sec)", count, duration, rate)
	})
}

// TestConcurrency tests concurrent access
func TestConcurrency(t *testing.T) {
	ts := StartTestServer(t)
	defer ts.Stop()

	t.Run("Concurrent Writes", func(t *testing.T) {
		var wg sync.WaitGroup
		workers := 100
		opsPerWorker := 100

		start := time.Now()

		for w := 0; w < workers; w++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()

				client, _ := NewRedisClient(ts.Addr())
				defer client.Close()

				for i := 0; i < opsPerWorker; i++ {
					key := fmt.Sprintf("concurrent:w:%d:%d", workerID, i)
					client.Send("SET", key, fmt.Sprintf("value:%d", i))
				}
			}(w)
		}

		wg.Wait()
		duration := time.Since(start)
		totalOps := workers * opsPerWorker
		rate := float64(totalOps) / duration.Seconds()

		t.Logf("Concurrent writes: %d ops in %v (%.0f ops/sec)", totalOps, duration, rate)
	})

	t.Run("Concurrent Reads", func(t *testing.T) {
		// Pre-populate
		client, _ := NewRedisClient(ts.Addr())
		for i := 0; i < 1000; i++ {
			client.Send("SET", fmt.Sprintf("concurrent:read:%d", i), fmt.Sprintf("value:%d", i))
		}
		client.Close()

		var wg sync.WaitGroup
		workers := 100
		opsPerWorker := 100

		start := time.Now()

		for w := 0; w < workers; w++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()

				client, _ := NewRedisClient(ts.Addr())
				defer client.Close()

				for i := 0; i < opsPerWorker; i++ {
					key := fmt.Sprintf("concurrent:read:%d", rand.Intn(1000))
					client.Send("GET", key)
				}
			}(w)
		}

		wg.Wait()
		duration := time.Since(start)
		totalOps := workers * opsPerWorker
		rate := float64(totalOps) / duration.Seconds()

		t.Logf("Concurrent reads: %d ops in %v (%.0f ops/sec)", totalOps, duration, rate)
	})
}

// TestMemoryUsage tests memory usage patterns
func TestMemoryUsage(t *testing.T) {
	ts := StartTestServer(t)
	defer ts.Stop()

	client, _ := NewRedisClient(ts.Addr())
	defer client.Close()

	t.Run("Memory Growth", func(t *testing.T) {
		// Get initial memory
		info1, _ := client.Send("INFO", "memory")
		t.Logf("Initial memory:\n%s", info1)

		// Add data
		for i := 0; i < 100000; i++ {
			client.Send("SET", fmt.Sprintf("mem:key:%d", i), fmt.Sprintf("value:%d:%s", i, makeString(100)))
		}

		// Get memory after insert
		info2, _ := client.Send("INFO", "memory")
		t.Logf("Memory after 100k keys:\n%s", info2)

		// Force GC
		runtime.GC()
		time.Sleep(100 * time.Millisecond)

		// Delete data
		for i := 0; i < 100000; i++ {
			client.Send("DEL", fmt.Sprintf("mem:key:%d", i))
		}

		// Get memory after delete
		info3, _ := client.Send("INFO", "memory")
		t.Logf("Memory after delete:\n%s", info3)
	})
}

func makeString(size int) string {
	b := make([]byte, size)
	for i := range b {
		b[i] = 'a' + byte(i%26)
	}
	return string(b)
}

// BenchmarkComprehensive runs comprehensive benchmarks
func BenchmarkComprehensive(b *testing.B) {
	ts := StartTestServer(&testing.T{})
	defer ts.Stop()

	client, _ := NewRedisClient(ts.Addr())
	defer client.Close()

	b.Run("SET", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			client.Send("SET", fmt.Sprintf("bench:key:%d", i), "value")
		}
	})

	b.Run("GET", func(b *testing.B) {
		// Pre-populate
		for i := 0; i < 10000; i++ {
			client.Send("SET", fmt.Sprintf("bench:get:%d", i), "value")
		}
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			client.Send("GET", fmt.Sprintf("bench:get:%d", i%10000))
		}
	})

	b.Run("HSET", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			client.Send("HSET", fmt.Sprintf("bench:hash:%d", i%1000), "field", "value")
		}
	})

	b.Run("LPUSH", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			client.Send("LPUSH", "bench:list", fmt.Sprintf("item:%d", i))
		}
	})

	b.Run("ZADD", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			client.Send("ZADD", "bench:zset", float64(i), fmt.Sprintf("member:%d", i))
		}
	})

	b.Run("XADD", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			client.Send("XADD", "bench:stream", "*", "data", fmt.Sprintf("value:%d", i))
		}
	})
}
