package server

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// RL, RATELIMIT, TOKENBUCKET, LEAKYBUCKET and THROTTLE are hash keys: a
// limiter keeps what it spent across a restart, through an AOF holding only
// hash commands. They were process-wide maps, so a restart handed every
// client a full allowance again.
func TestLimitersAreKeys(t *testing.T) {
	cfg := aofTxConfig(t)
	s := startToggleServer(t, cfg)
	c := dialRepl(t, s)
	ok := func(args ...string) {
		t.Helper()
		if v := c.do(args...); v.Type == '-' {
			t.Fatalf("%v: %v", args, v.Err)
		}
	}
	ok("RL.CREATE", "rl", "5", "1", "3600000")
	ok("RL.ALLOW", "rl", "3")
	ok("RATELIMIT.CREATE", "fw", "2", "3600000")
	ok("RATELIMIT.CHECK", "fw", "alice")
	ok("RATELIMIT.CHECK", "fw", "alice")
	ok("TOKENBUCKET.CREATE", "tb", "10", "0")
	ok("TOKENBUCKET.CONSUME", "tb", "7.5")
	ok("LEAKYBUCKET.CREATE", "lb", "4", "0")
	ok("LEAKYBUCKET.ADD", "lb", "3")
	ok("THROTTLE.SET", "th", "3600000")
	ok("THROTTLE.CALL", "th")
	ok("SET", "plain", "v")
	for _, args := range [][]string{
		{"RL.ALLOW", "plain", "1"}, {"RATELIMIT.CHECK", "plain", "x"}, {"TOKENBUCKET.CONSUME", "plain", "1"},
		{"LEAKYBUCKET.ADD", "plain", "1"}, {"THROTTLE.CALL", "plain"}, {"RL.CREATE", "plain", "1", "1", "1"},
		{"TOKENBUCKET.CONSUME", "rl", "1"}, {"THROTTLE.CALL", "tb"},
	} {
		if got := c.do(args...); got.Type != '-' || !strings.HasPrefix(got.Err, "WRONGTYPE") {
			t.Fatalf("%v: %v", args, got)
		}
	}
	for _, args := range [][]string{{"RL.ALLOW", "rl", "-1"}, {"TOKENBUCKET.CREATE", "x", "nan", "1"}, {"LEAKYBUCKET.ADD", "lb", "x"}} {
		if got := c.do(args...); got.Type != '-' {
			t.Fatalf("%v accepted: %v", args, got)
		}
	}
	stopReplServer(s)

	data, err := os.ReadFile(filepath.Join(cfg.Persistence.DataDir, "appendonly.aof"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"RL.", "RATELIMIT.", "TOKENBUCKET.", "LEAKYBUCKET.", "THROTTLE."} {
		if strings.Contains(string(data), name) {
			t.Fatalf("the AOF records %s commands", name)
		}
	}

	s = startToggleServer(t, cfg)
	defer stopReplServer(s)
	c = dialRepl(t, s)
	if got := c.do("RL.ALLOW", "rl", "3"); len(got.Array) != 3 || got.Array[0].Int != 0 || got.Array[1].Int != 2 {
		t.Fatalf("RL.ALLOW after restart: %+v", got)
	}
	if got := c.do("RATELIMIT.CHECK", "fw", "alice"); len(got.Array) != 3 || got.Array[0].Int != 0 {
		t.Fatalf("RATELIMIT.CHECK past the limit after restart: %+v", got)
	}
	if got := c.do("RATELIMIT.CHECK", "fw", "bob"); len(got.Array) != 3 || got.Array[0].Int != 1 || got.Array[1].Int != 1 {
		t.Fatalf("RATELIMIT.CHECK for another id: %+v", got)
	}
	if got := c.do("TOKENBUCKET.AVAILABLE", "tb"); string(got.Bulk) != "2.50" {
		t.Fatalf("TOKENBUCKET.AVAILABLE after restart: %+v", got)
	}
	if got := c.do("LEAKYBUCKET.AVAILABLE", "lb"); got.Int != 1 {
		t.Fatalf("LEAKYBUCKET.AVAILABLE after restart: %+v", got)
	}
	if got := c.do("THROTTLE.CALL", "th"); len(got.Array) != 2 || got.Array[0].Int != 0 {
		t.Fatalf("THROTTLE.CALL after restart: %+v", got)
	}
	if got := c.do("TYPE", "tb"); got.Str != "hash" {
		t.Fatalf("TYPE tb: %v", got)
	}
	for _, args := range [][]string{{"RL.RESET", "rl"}, {"TOKENBUCKET.RESET", "tb"}, {"THROTTLE.RESET", "th"}, {"RATELIMIT.RESET", "fw"}} {
		if got := c.do(args...); got.Type == '-' {
			t.Fatalf("%v: %v", args, got.Err)
		}
	}
	if got := c.do("RL.ALLOW", "rl", "5"); got.Array[0].Int != 1 {
		t.Fatalf("RL.ALLOW after RESET: %+v", got)
	}
	if got := c.do("THROTTLE.CALL", "th"); got.Array[0].Int != 1 {
		t.Fatalf("THROTTLE.CALL after RESET: %+v", got)
	}
	for _, args := range [][]string{{"RL.DELETE", "rl"}, {"RATELIMIT.DELETE", "fw"}, {"TOKENBUCKET.DELETE", "tb"}, {"LEAKYBUCKET.DELETE", "lb"}, {"THROTTLE.DELETE", "th"}} {
		if got := c.do(args...); got.Int != 1 {
			t.Fatalf("%v: %v", args, got)
		}
	}
	if got := c.do("DBSIZE"); got.Int != 1 {
		t.Fatalf("DBSIZE after the deletes: %v", got)
	}
}

// Concurrent checks of one limiter are counted exactly: each is a
// read-modify-write of the key, serialized by the write gate.
func TestRateLimitCountsConcurrentChecks(t *testing.T) {
	cfg := aofTxConfig(t)
	s := startToggleServer(t, cfg)
	defer stopReplServer(s)
	c := dialRepl(t, s)
	c.do("RATELIMIT.CREATE", "api", "300", "3600000")
	var allowed atomic.Int64
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := dialRepl(t, s)
			for range 50 {
				if v := w.do("RATELIMIT.CHECK", "api", "x"); len(v.Array) == 3 && v.Array[0].Int == 1 {
					allowed.Add(1)
				}
			}
		}()
	}
	wg.Wait()
	if got := allowed.Load(); got != 300 {
		t.Fatalf("%d of 400 concurrent checks allowed under a limit of 300", got)
	}
}

// Every command of a keyspace-backed extension namespace is classified: a
// write must be in writeCommands, which gives it the write gate (so its
// read-modify-write is atomic per key) and ordering with its propagation.
func TestKeyspaceExtensionWritesAreGated(t *testing.T) {
	reads := map[string]bool{
		"QUEUE.CREATE": true, "QUEUE.PEEK": true, "QUEUE.LEN": true,
		"STACK.CREATE": true, "STACK.PEEK": true, "STACK.LEN": true,
		"LOCK.INFO": true, "LOCK.ISLOCKED": true,
		"RATELIMITER.STATUS": true,
		"SESSION.GET":        true, "SESSION.EXISTS": true, "SESSION.TTL": true, "SESSION.ALL": true,
		"SESSION.LIST": true, "SESSION.COUNT": true, "SESSION.CLEANUP": true,
		"CIRCUIT.STATE": true, "CIRCUIT.STATS": true, "CIRCUIT.LIST": true,
		"MSGQUEUE.DEADLETTER": true, "MSGQUEUE.STATS": true,
		"RL.GET": true, "TOKENBUCKET.AVAILABLE": true, "LEAKYBUCKET.AVAILABLE": true,
		"LOCKX.STATUS": true, "LEADER.CURRENT": true, "LEADER.HISTORY": true,
		"CIRCUITX.STATUS": true, "CIRCUITX.METRICS": true,
		"SEM.VALUE": true, "SEMAPHOREX.STATUS": true, "SESSIONX.GET": true, "SESSIONX.LIST": true,
		"SERVICE.DISCOVER": true, "SERVICE.LIST": true, "SERVICE.HEALTHY": true,
		"DOC.FIND": true, "DOC.FINDONE": true, "DOC.COUNT": true, "DOC.DISTINCT": true, "DOC.AGGREGATE": true,
		"AGG.SUM": true, "AGG.AVG": true, "AGG.MIN": true, "AGG.MAX": true, "AGG.COUNT": true,
		"CRDT.LWW.GET": true, "CRDT.GCOUNTER.GET": true, "CRDT.PNCOUNTER.GET": true, "CRDT.GSET.GET": true,
		"CRDT.ORSET.GET": true, "CRDT.ORSET.TAGS": true,
		"DEDUP.CHECK": true, "DEDUP.EXPIRE": true, "VECTOR_CLOCK.COMPARE": true, "VECTOR_CLOCK.GET": true,
		"THRESHOLD.CHECK": true, "THRESHOLD.LIST": true, "SWITCH.STATE": true, "SWITCH.LIST": true,
		"BOOKMARK.GET": true, "BOOKMARK.LIST": true, "DEADLINE.CHECK": true, "DEADLINE.LIST": true,
		"ROUTE.MATCH": true, "ROUTE.LIST": true, "GATEWAY.LIST": true, "GRID.GET": true, "GRID.QUERY": true,
		"GHOST.READ": true, "TAPE.READ": true, "SLICE.GET": true, "ROLLUPX.GET": true,
		"SECRET.GET": true, "SECRET.LIST": true, "SECRET.VERSION": true, "CONFIG.GET": true, "CONFIG.LIST": true,
		"CONFIG.EXPORT": true, "TRIE.SEARCH": true, "TRIE.PREFIX": true, "TRIE.AUTOCOMPLETE": true, "RING.GET": true,
		"RING.NODES": true, "ACTOR.POKE": true, "ACTOR.PEEK": true, "ACTOR.LEN": true, "ACTOR.LIST": true,
		"DAG.TOPO": true, "DAG.PARENTS": true, "DAG.CHILDREN": true, "DAG.LIST": true,
		"VALIDATE.CHECK": true, "VALIDATE.LIST": true, "VALIDATE.ALPHA": true, "VALIDATE.ALPHANUM": true,
		"VALIDATE.EMAIL": true, "VALIDATE.FLOAT": true, "VALIDATE.INT": true, "VALIDATE.IP": true, "VALIDATE.JSON": true,
		"VALIDATE.LENGTH": true, "VALIDATE.RANGE": true, "VALIDATE.URL": true, "VIEW.GET": true, "VIEW.LIST": true, "STAGE.LIST": true,
		"CONTEXT.GET": true, "CONTEXT.LIST": true, "CACHEX.GET": true, "CACHEX.LIST": true, "STOREX.GET": true,
		"STOREX.LIST": true, "RECORD.GET": true, "ENTITY.GET": true, "ENTITY.LIST": true, "PROFILE.GET": true,
		"PROFILE.LIST": true, "COUNTERX2.GET": true, "COUNTERX2.LIST": true, "LEVEL.GET": true, "LEVEL.LIST": true,
		"PERMIT.CHECK": true, "PERMIT.LIST": true, "GRANT.CHECK": true, "GRANT.LIST": true, "RELATION.GET": true,
		"RELATION.LIST": true, "TIMER.STATUS": true, "TIMER.LIST": true, "TOKEN.VALIDATE": true, "TOKEN.LIST": true,
		"POOLX.STATUS": true, "BUFFERX.READ": true, "STREAMX.READ": true, "AUDITX.GET": true, "AUDITX.SEARCH": true,
		"AUDITX.LIST": true, "INDEX.SEARCH": true, "INDEX.LIST": true,
		"PQ.PEEK": true, "PQ.LEN": true, "PQ.GETALL": true, "LRU.KEYS": true, "LRU.STATS": true,
		"SLIDINGWINDOW.COUNT": true, "DEBOUNCE.GET": true,
		"SLIDING.STATS":   true,
		"IDEMPOTENCY.GET": true, "IDEMPOTENCY.CHECK": true, "IDEMPOTENCY.LIST": true,
		"EXPERIMENT.RESULTS": true, "EXPERIMENT.LIST": true,
		"ROLLOUT.CHECK": true, "ROLLOUT.LIST": true,
		"ALERT.LIST": true, "ALERT.HISTORY": true,
		"COUNTERX.GET": true,
		"GAUGE.GET":    true,
		"TRACE.GET":    true, "TRACE.LIST": true,
		"LOGX.READ": true, "LOGX.SEARCH": true, "LOGX.STATS": true,
		"APIKEY.LIST": true, "APIKEY.USAGE": true,
		"QUOTAX.USAGE": true,
		"METER.GET":    true,
		"TENANT.GET":   true, "TENANT.LIST": true,
		"LEASE.GET": true, "LEASE.LIST": true,
		"HEAP.PEEK": true, "HEAP.SIZE": true, "HEAP.STATS": true, "HEAP.GC": true,
		"BLOOMX.CHECK": true, "BLOOMX.INFO": true,
		"SKETCH.QUERY":    true,
		"RINGBUFFER.READ": true, "RINGBUFFER.SIZE": true,
		"WINDOW.GET": true, "WINDOW.AGGREGATE": true,
		"FREQ.COUNT": true, "FREQ.TOP": true,
		"PARTITION.GET": true, "PARTITION.LIST": true,
		"BULKHEAD.STATUS": true,
		"TELEMETRY.QUERY": true,
		"PROMISE.STATUS":  true, "PROMISE.AWAIT": true,
		"FUTURE.GET":           true,
		"STREAMPROC.PEEK":      true,
		"EVENTSOURCING.REPLAY": true, "EVENTSOURCING.GET": true,
		"BACKPRESSURE.STATUS": true,
		"THROTTLEX.STATUS":    true,
		"COALESCE.GET":        true,
		"AGGREGATOR.GET":      true,
		"WINDOWX.GET":         true, "WINDOWX.AGGREGATE": true,
		"JOINX.GET":      true,
		"PARTITIONX.GET": true,
		"PROFILEX.LIST":  true,
		"VECTOR.GET":     true, "VECTOR.SEARCH": true, "VECTOR.SIMILARITY": true, "VECTOR.DIMENSIONS": true, "VECTOR.STATS": true,
	}
	namespaces := []string{"QUEUE", "STACK", "LOCK", "RATELIMITER", "SESSION", "CIRCUIT", "MSGQUEUE", "RL", "RATELIMIT", "TOKENBUCKET", "LEAKYBUCKET", "THROTTLE", "LOCKX", "LEADER", "CIRCUITX", "CIRCUITBREAKER", "SEM", "SEMAPHOREX", "SESSIONX", "SERVICE", "DOC", "AGG", "VECTOR", "CRDT", "DEDUP", "VECTOR_CLOCK", "THRESHOLD", "SWITCH", "BOOKMARK", "DEADLINE", "ROUTE", "GATEWAY", "GRID", "GHOST", "TAPE", "SLICE", "ROLLUPX", "SECRET", "CONFIG", "TRIE", "RING", "ACTOR", "DAG", "VALIDATE", "VIEW", "STAGE", "CONTEXT", "CACHEX", "STOREX", "RECORD", "ENTITY", "PROFILE", "COUNTERX2", "LEVEL", "PERMIT", "GRANT", "RELATION", "TIMER", "TOKEN", "POOLX", "BUFFERX", "STREAMX", "AUDITX", "INDEX", "PQ", "LRU", "SLIDINGWINDOW", "DEBOUNCE", "SLIDING", "BUCKETX", "IDEMPOTENCY", "EXPERIMENT", "ROLLOUT", "ALERT", "COUNTERX", "GAUGE", "TRACE", "LOGX", "APIKEY", "QUOTAX", "METER", "TENANT", "LEASE", "HEAP", "BLOOMX", "SKETCH", "RINGBUFFER", "WINDOW", "FREQ", "PARTITION", "BULKHEAD", "TELEMETRY", "PROMISE", "FUTURE", "STREAMPROC", "EVENTSOURCING", "BACKPRESSURE", "THROTTLEX", "DEBOUNCEX", "COALESCE", "AGGREGATOR", "WINDOWX", "JOINX", "SHUFFLE", "PARTITIONX", "PROFILEX"}
	cfg := aofTxConfig(t)
	cfg.Server.ExtensionCommands = "all"
	s := startToggleServer(t, cfg)
	defer stopReplServer(s)
	seen := 0
	for name := range s.router.Commands() {
		ns, _, dotted := strings.Cut(name, ".")
		if !dotted {
			continue
		}
		for _, want := range namespaces {
			if ns != want {
				continue
			}
			seen++
			if !writeCommands[name] && !reads[name] {
				t.Errorf("%s is neither in writeCommands nor a known read", name)
			}
			if writeCommands[name] && reads[name] {
				t.Errorf("%s is listed as both a write and a read", name)
			}
		}
	}
	if seen < 60 {
		t.Fatalf("only %d commands of the keyspace namespaces are registered", seen)
	}
}
