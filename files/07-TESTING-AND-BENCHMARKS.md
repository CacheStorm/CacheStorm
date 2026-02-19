# CacheStorm — Testing & Benchmarks Specification

## 1. Test Strategy

### 1.1 Test Layers

```
Layer 1: Unit Tests
  - Every package has _test.go files
  - Test individual functions and methods
  - Mock dependencies where needed
  - Target: 80%+ coverage

Layer 2: Integration Tests
  - Start actual CacheStorm server in-process
  - Connect with real TCP client
  - Test full command flow: TCP → RESP → Router → Store → Response
  - Test multi-command sequences (SET → GET → DEL → GET)
  - Located in: internal/server/integration_test.go

Layer 3: Compatibility Tests
  - Use real Redis client libraries (go-redis) against CacheStorm
  - Verify behavior matches Redis for supported commands
  - Located in: tests/compat/

Layer 4: Cluster Tests
  - Spin up 3-node cluster in-process
  - Test slot routing, MOVED responses
  - Test tag invalidation propagation
  - Test node join/leave
  - Located in: internal/cluster/cluster_integration_test.go

Layer 5: Benchmark Tests
  - Go benchmark tests for hot paths
  - Located in: benchmarks/
```

### 1.2 Test Helpers

```go
// tests/helpers.go

package tests

import (
    "net"
    "testing"
    "time"
    "github.com/cachestorm/cachestorm/internal/server"
    "github.com/cachestorm/cachestorm/internal/config"
    "github.com/cachestorm/cachestorm/internal/resp"
)

// TestServer starts a CacheStorm server on a random port for testing.
type TestServer struct {
    Server *server.Server
    Port   int
    t      *testing.T
}

func NewTestServer(t *testing.T) *TestServer {
    t.Helper()
    cfg := config.DefaultConfig()
    cfg.Server.Port = 0 // let OS pick a free port
    cfg.Logging.Level = "error" // quiet during tests

    srv := server.New(cfg)
    go srv.Start(context.Background())

    // Wait for server to be ready
    port := srv.Port() // method returns actual bound port
    waitForPort(t, port, 5*time.Second)

    return &TestServer{Server: srv, Port: port, t: t}
}

func (ts *TestServer) Close() {
    ts.Server.Stop(context.Background())
}

// Dial connects a raw RESP client to the test server.
func (ts *TestServer) Dial() *TestClient {
    conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", ts.Port), 5*time.Second)
    if err != nil {
        ts.t.Fatalf("dial failed: %v", err)
    }
    return &TestClient{
        conn:   conn,
        reader: resp.NewReader(conn),
        writer: resp.NewWriter(conn),
    }
}

// TestClient is a simple RESP client for testing.
type TestClient struct {
    conn   net.Conn
    reader *resp.Reader
    writer *resp.Writer
}

func (tc *TestClient) Do(args ...string) resp.Value {
    // Write RESP array command
    tc.writer.WriteArray(len(args))
    for _, arg := range args {
        tc.writer.WriteBulkStringBytes([]byte(arg))
    }
    tc.writer.Flush()
    // Read response
    val, err := tc.reader.ReadValue()
    if err != nil {
        panic(err)
    }
    return val
}

func (tc *TestClient) Close() {
    tc.conn.Close()
}

// Assert helpers
func AssertOK(t *testing.T, val resp.Value) {
    t.Helper()
    if val.Type != resp.TypeSimpleString || val.Str != "OK" {
        t.Fatalf("expected +OK, got %v", val)
    }
}

func AssertBulk(t *testing.T, val resp.Value, expected string) {
    t.Helper()
    if val.Type != resp.TypeBulkString || string(val.Bulk) != expected {
        t.Fatalf("expected $%s, got %v", expected, val)
    }
}

func AssertInteger(t *testing.T, val resp.Value, expected int64) {
    t.Helper()
    if val.Type != resp.TypeInteger || val.Int != expected {
        t.Fatalf("expected :%d, got %v", expected, val)
    }
}

func AssertNull(t *testing.T, val resp.Value) {
    t.Helper()
    if !val.IsNull {
        t.Fatalf("expected null, got %v", val)
    }
}

func AssertError(t *testing.T, val resp.Value, contains string) {
    t.Helper()
    if val.Type != resp.TypeError {
        t.Fatalf("expected error, got %v", val)
    }
    if !strings.Contains(val.Str, contains) {
        t.Fatalf("expected error containing %q, got %q", contains, val.Str)
    }
}
```

## 2. Unit Test Specifications

### 2.1 RESP Parser Tests (`internal/resp/reader_test.go`)

```
TestReadSimpleString          — "+OK\r\n" → SimpleString("OK")
TestReadError                 — "-ERR msg\r\n" → Error("ERR msg")
TestReadInteger               — ":1000\r\n" → Integer(1000)
TestReadIntegerNegative       — ":-50\r\n" → Integer(-50)
TestReadIntegerZero           — ":0\r\n" → Integer(0)
TestReadBulkString            — "$5\r\nhello\r\n" → BulkString("hello")
TestReadBulkStringEmpty       — "$0\r\n\r\n" → BulkString("")
TestReadBulkStringNull        — "$-1\r\n" → Null
TestReadBulkStringBinary      — binary data with \r\n inside
TestReadArray                 — "*2\r\n$3\r\nfoo\r\n$3\r\nbar\r\n" → Array[foo, bar]
TestReadArrayEmpty            — "*0\r\n" → Array[]
TestReadArrayNull             — "*-1\r\n" → Null
TestReadArrayNested           — array containing array
TestReadCommand               — parse SET command from array
TestReadCommandPING           — inline PING → cmd="PING"
TestReadInvalid               — garbage input → error
TestReadIncomplete            — truncated input → error or block
TestReadLargeBulkString       — 1MB bulk string
```

### 2.2 RESP Writer Tests (`internal/resp/writer_test.go`)

```
TestWriteSimpleString         — "OK" → "+OK\r\n"
TestWriteError                — "ERR" → "-ERR\r\n"
TestWriteInteger              — 42 → ":42\r\n"
TestWriteBulkString           — "hello" → "$5\r\nhello\r\n"
TestWriteNull                 — → "$-1\r\n"
TestWriteArray                — 2 elements → "*2\r\n..."
TestWriteNullArray            — → "*-1\r\n"
TestRoundTrip                 — write various types → read back → compare
```

### 2.3 Store/Shard Tests (`internal/store/shard_test.go`)

```
TestShardSetGet               — basic set then get
TestShardSetOverwrite         — set same key twice, get returns latest
TestShardGetMissing           — get non-existent key → nil
TestShardDel                  — set then delete → get returns nil
TestShardDelMissing           — delete non-existent → false
TestShardExists               — exists after set, not after del
TestShardSetWithTTL           — set with TTL, get immediately → value
TestShardSetExpired           — set with 1ms TTL, sleep, get → nil (lazy expiry)
TestShardConcurrentReadWrite  — 100 goroutines SET/GET same keys → no race
TestShardConcurrentDifferent  — 100 goroutines SET/GET different keys → all correct
TestShardMemoryTracking       — set N keys, verify memUsage matches
TestShardFNV32aDistribution   — hash 10000 random keys → verify even distribution across shards
```

### 2.4 Tag Index Tests (`internal/store/tag_index_test.go`)

```
TestTagAddAndGet              — add tags to key, get tags → matches
TestTagAddMultiple            — key with 5 tags, verify all present
TestTagRemove                 — remove 1 tag from key with 3 tags → 2 remain
TestTagRemoveKey              — remove key from all tags (on DEL)
TestTagGetKeys                — 100 keys with same tag → GetKeys returns all 100
TestTagInvalidate             — 100 keys with tag "t1" → Invalidate("t1") → all gone
TestTagInvalidateMultiple     — keys with different tags → Invalidate only removes targeted
TestTagInvalidateCleanup      — after invalidation, tag entry itself is removed
TestTagCrossTag               — key in tag A and B, invalidate A → key gone, also removed from B
TestTagCount                  — add 50 keys → Count returns 50
TestTagHierarchy              — TAGLINK parent→child, TAGCHILDREN returns child
TestTagCascadeInvalidation    — parent has children → cascade invalidate → all gone
TestTagConcurrent             — parallel AddTags + Invalidate → no race, no leak
TestTagLargeScale             — 100K keys, 1000 tags → invalidate 1 tag → verify speed + correctness
```

### 2.5 Timing Wheel Tests (`internal/store/timing_wheel_test.go`)

```
TestTimingWheelAdd            — add key with 5s TTL → not expired at 4s, expired at 6s
TestTimingWheelRemove         — add then remove → never expires
TestTimingWheelReschedule     — add with 2s, extend to 10s → survives 5s mark
TestTimingWheelShortTTL       — 100ms TTL → expires within 200ms
TestTimingWheelLongTTL        — 1 hour TTL → correct wheel level assignment
TestTimingWheelVeryLongTTL    — 400 day TTL → far future bucket
TestTimingWheelManyKeys       — 10000 keys with random TTLs → all expire correctly
TestTimingWheelConcurrent     — parallel Add/Remove → no race
```

### 2.6 Eviction Tests (`internal/store/eviction_test.go`)

```
TestEvictionLRUBasic          — set 3 keys, access 2, evict 1 → least recent evicted
TestEvictionLRUOrder          — set 5 keys with known access order → correct eviction sequence
TestEvictionMemoryPressure    — max_memory=1KB, fill beyond → keys evicted to stay under
TestEvictionNoEviction        — policy=noeviction, fill beyond → write rejected with error
TestEvictionVolatileLRU       — mix TTL and no-TTL keys → only TTL keys evicted
TestEvictionRandom            — policy=random → keys evicted (any valid key)
TestEvictionWithTags          — evicted keys properly cleaned from tag index
TestEvictionHook              — OnEvict hook fires for each eviction
```

### 2.7 Command Tests (per command file)

Each command needs at minimum:
- Success case
- Missing key case
- Wrong type case (WRONGTYPE error)
- Wrong argument count
- Edge cases specific to the command

Example for SET:
```
TestSETBasic                  — SET key value → OK, GET key → value
TestSETOverwrite              — SET key v1, SET key v2 → GET returns v2
TestSETWithEX                 — SET key v EX 1 → TTL = 1, wait 2s → nil
TestSETWithPX                 — SET key v PX 500 → PTTL ~500, wait 600ms → nil
TestSETWithNXNew              — SET key v NX → OK (new key)
TestSETWithNXExists           — SET key v, SET key v2 NX → nil (not set)
TestSETWithXXExists           — SET key v, SET key v2 XX → OK
TestSETWithXXMissing          — SET key v XX → nil (key doesn't exist)
TestSETWithGET                — SET key v1, SET key v2 GET → returns v1
TestSETWithKEEPTTL            — SET key v EX 100, SET key v2 KEEPTTL → TTL preserved
TestSETSyntaxError            — SET key → wrong arg count error
TestSETInvalidOption          — SET key v INVALID → syntax error
```

## 3. Integration Tests

### 3.1 Basic Flow (`internal/server/integration_test.go`)

```
TestIntegrationPing           — connect, PING → PONG
TestIntegrationSetGet         — SET foo bar → GET foo → "bar"
TestIntegrationMultiClient    — 10 clients, each SET/GET own keys → all correct
TestIntegrationPipeline       — send 100 commands without reading, then read 100 responses
TestIntegrationLargeValue     — SET key with 1MB value → GET returns correctly
TestIntegrationConnectionClose — set key, disconnect, reconnect, GET → still there
TestIntegrationMaxConnections — exceed max_connections → new connection rejected
TestIntegrationGracefulShutdown — server stopping → in-flight commands complete
```

### 3.2 Tag Integration

```
TestIntegrationSettag         — SETTAG key val TAG t1 t2 → TAGS key → [t1, t2]
TestIntegrationInvalidate     — SETTAG 100 keys with "group1" → INVALIDATE group1 → all 100 gone
TestIntegrationCascade        — TAGLINK parent child → SETTAG keys with child → TAGINVALIDATE parent CASCADE → all gone
TestIntegrationTagCleanup     — SETTAG key val TAG t1, DEL key → TAGKEYS t1 → empty
```

### 3.3 Namespace Integration

```
TestIntegrationNamespace      — NAMESPACE "test" → SET key val → NAMESPACE "default" → GET key → nil
TestIntegrationNamespaceIsolation — keys, tags in ns1 isolated from ns2
TestIntegrationSelectCompat   — SELECT 0 = default, SELECT 1 = db1
TestIntegrationFlushDB        — only current namespace flushed
```

### 3.4 Redis Client Compatibility (`tests/compat/`)

Using `go-redis/redis/v9`:
```
TestCompatGoRedisBasic        — go-redis client: Set, Get, Del
TestCompatGoRedisHash         — go-redis: HSet, HGet, HGetAll
TestCompatGoRedisList         — go-redis: LPush, LRange, RPop
TestCompatGoRedisSet          — go-redis: SAdd, SMembers, SCard
TestCompatGoRedisTTL          — go-redis: Set with expiration, TTL check
TestCompatGoRedisPipeline     — go-redis: pipeline multiple commands
TestCompatGoRedisCustomCmd    — go-redis: send custom SETTAG/INVALIDATE via Do()
```

## 4. Benchmark Specifications

### 4.1 RESP Benchmarks (`benchmarks/resp_bench_test.go`)

```go
func BenchmarkRESPReadSimpleString(b *testing.B)
func BenchmarkRESPReadBulkString100(b *testing.B)   // 100 byte value
func BenchmarkRESPReadBulkString1KB(b *testing.B)
func BenchmarkRESPReadBulkString10KB(b *testing.B)
func BenchmarkRESPReadArray10(b *testing.B)           // 10 element array
func BenchmarkRESPWriteSimpleString(b *testing.B)
func BenchmarkRESPWriteBulkString100(b *testing.B)
func BenchmarkRESPWriteBulkString1KB(b *testing.B)
```

### 4.2 Store Benchmarks (`benchmarks/store_bench_test.go`)

```go
func BenchmarkStoreSet(b *testing.B)                  // sequential SET
func BenchmarkStoreGet(b *testing.B)                  // sequential GET (existing key)
func BenchmarkStoreGetMissing(b *testing.B)           // GET non-existent key
func BenchmarkStoreSetParallel(b *testing.B)          // b.RunParallel SET
func BenchmarkStoreGetParallel(b *testing.B)          // b.RunParallel GET
func BenchmarkStoreMixedParallel(b *testing.B)        // 80% GET + 20% SET
func BenchmarkStoreSet100B(b *testing.B)              // SET with 100 byte value
func BenchmarkStoreSet1KB(b *testing.B)               // SET with 1KB value
func BenchmarkStoreSet10KB(b *testing.B)              // SET with 10KB value
func BenchmarkStoreHashSet(b *testing.B)              // HSET
func BenchmarkStoreHashGet(b *testing.B)              // HGET
func BenchmarkStoreListPush(b *testing.B)             // LPUSH
func BenchmarkStoreListRange(b *testing.B)            // LRANGE
func BenchmarkStoreSetAdd(b *testing.B)               // SADD
func BenchmarkStoreSetMembers(b *testing.B)           // SMEMBERS
func BenchmarkStoreFNV32a(b *testing.B)               // hash function speed
func BenchmarkStoreShardLookup(b *testing.B)          // shard selection
```

### 4.3 Tag Benchmarks (`benchmarks/tag_bench_test.go`)

```go
func BenchmarkTagAdd(b *testing.B)                    // add single tag
func BenchmarkTagAdd5(b *testing.B)                   // add 5 tags at once
func BenchmarkTagGetKeys100(b *testing.B)             // get keys for tag with 100 members
func BenchmarkTagGetKeys10K(b *testing.B)             // get keys for tag with 10K members
func BenchmarkTagInvalidate100(b *testing.B)          // invalidate tag with 100 keys
func BenchmarkTagInvalidate1K(b *testing.B)           // invalidate tag with 1K keys
func BenchmarkTagInvalidate10K(b *testing.B)          // invalidate tag with 10K keys
func BenchmarkTagInvalidate100K(b *testing.B)         // invalidate tag with 100K keys
func BenchmarkTagParallelInvalidate(b *testing.B)     // concurrent invalidation
```

### 4.4 End-to-End Benchmarks (`benchmarks/e2e_bench_test.go`)

```go
// These start a real server and use TCP connections

func BenchmarkE2ESET(b *testing.B)                    // full SET command over TCP
func BenchmarkE2EGET(b *testing.B)                    // full GET command over TCP
func BenchmarkE2EPipeline10(b *testing.B)             // 10 pipelined commands
func BenchmarkE2EPipeline100(b *testing.B)            // 100 pipelined commands
func BenchmarkE2EConcurrent100(b *testing.B)          // 100 concurrent clients
func BenchmarkE2EConcurrent1000(b *testing.B)         // 1000 concurrent clients
func BenchmarkE2ESETTAG(b *testing.B)                 // SETTAG with 3 tags
func BenchmarkE2EInvalidate(b *testing.B)             // INVALIDATE over TCP
```

### 4.5 Comparison Script (`scripts/benchmark.sh`)

```bash
#!/bin/bash
# Compare CacheStorm vs Redis performance

echo "=== CacheStorm Benchmark ==="
# Start CacheStorm
./bin/cachestorm --port 6380 &
CS_PID=$!
sleep 1

redis-benchmark -p 6380 -t set,get -n 100000 -c 50 -q
redis-benchmark -p 6380 -t set,get -n 100000 -c 50 -d 1024 -q  # 1KB values

kill $CS_PID

echo ""
echo "=== Redis Benchmark (for comparison) ==="
redis-benchmark -p 6379 -t set,get -n 100000 -c 50 -q
redis-benchmark -p 6379 -t set,get -n 100000 -c 50 -d 1024 -q

echo ""
echo "=== CacheStorm Tag Benchmark (Redis has no equivalent) ==="
# Custom benchmark for SETTAG + INVALIDATE
go test ./benchmarks/ -bench=BenchmarkTag -benchmem -count=3
```

## 5. CI/CD Pipeline

### 5.1 GitHub Actions CI (`.github/workflows/ci.yml`)

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        go-version: ['1.22', '1.23']
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: ${{ matrix.go-version }}

      - name: Download dependencies
        run: go mod download

      - name: Lint
        uses: golangci/golangci-lint-action@v4
        with:
          version: latest

      - name: Test
        run: go test ./... -race -count=1 -coverprofile=coverage.out

      - name: Coverage
        run: |
          go tool cover -func=coverage.out
          COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | sed 's/%//')
          echo "Total coverage: ${COVERAGE}%"
          if (( $(echo "$COVERAGE < 80" | bc -l) )); then
            echo "Coverage below 80%!"
            exit 1
          fi

      - name: Benchmark
        run: go test ./benchmarks/ -bench=. -benchmem -count=1 -timeout=300s

      - name: Build
        run: go build -o bin/cachestorm ./cmd/cachestorm

  docker:
    needs: test
    runs-on: ubuntu-latest
    if: github.ref == 'refs/heads/main'
    steps:
      - uses: actions/checkout@v4
      - name: Build Docker image
        run: docker build -t cachestorm:test -f docker/Dockerfile .
      - name: Test Docker image
        run: |
          docker run -d --name cs-test -p 6380:6380 cachestorm:test
          sleep 2
          redis-cli -p 6380 PING | grep PONG
          docker stop cs-test
```

### 5.2 Release Pipeline (`.github/workflows/release.yml`)

```yaml
name: Release

on:
  push:
    tags: ['v*']

permissions:
  contents: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      - name: Run GoReleaser
        uses: goreleaser/goreleaser-action@v5
        with:
          version: latest
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

## 6. Race Condition Testing

All tests must pass with `-race` flag:
```bash
go test ./... -race -count=1
```

Additionally, specific concurrency stress tests:
```go
func TestRaceConditionSetGet(t *testing.T) {
    store := newTestStore()
    var wg sync.WaitGroup
    for i := 0; i < 100; i++ {
        wg.Add(2)
        key := fmt.Sprintf("key:%d", i)
        go func() {
            defer wg.Done()
            for j := 0; j < 1000; j++ {
                store.Set("default", key, &StringValue{Data: []byte("value")}, 0, nil)
            }
        }()
        go func() {
            defer wg.Done()
            for j := 0; j < 1000; j++ {
                store.Get("default", key)
            }
        }()
    }
    wg.Wait()
}

func TestRaceConditionTagInvalidation(t *testing.T) {
    // 50 goroutines adding keys with tags
    // 50 goroutines invalidating tags
    // Must not panic, crash, or leak memory
}
```

## 7. Memory Leak Detection

```go
func TestNoMemoryLeak(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping memory leak test in short mode")
    }

    store := newTestStore()

    // Baseline
    runtime.GC()
    var m1 runtime.MemStats
    runtime.ReadMemStats(&m1)

    // Write and delete 100K keys 10 times
    for round := 0; round < 10; round++ {
        for i := 0; i < 100000; i++ {
            key := fmt.Sprintf("key:%d", i)
            store.Set("default", key, &StringValue{Data: make([]byte, 100)}, 0, []string{"tag1"})
        }
        store.FlushNamespace("default")
    }

    // After
    runtime.GC()
    time.Sleep(100 * time.Millisecond)
    runtime.GC()
    var m2 runtime.MemStats
    runtime.ReadMemStats(&m2)

    // Allow 10MB variance for runtime overhead
    leaked := int64(m2.HeapInuse) - int64(m1.HeapInuse)
    if leaked > 10*1024*1024 {
        t.Fatalf("possible memory leak: %d bytes not freed after flush", leaked)
    }
}
```
