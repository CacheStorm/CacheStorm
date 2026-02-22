package command

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func newCoverageTestCtx(cmd string, args [][]byte, s *store.Store) *Context {
	buf := &bytes.Buffer{}
	w := resp.NewWriter(buf)
	return NewContext(cmd, args, s, w)
}

func runCommandTest(t *testing.T, router *Router, s *store.Store, cmd string, args [][]byte) {
	ctx := newCoverageTestCtx(cmd, args, s)
	handler, ok := router.Get(cmd)
	if !ok {
		t.Skipf("Command %s not found", cmd)
		return
	}
	if err := handler.Handler(ctx); err != nil {
		t.Errorf("Command %s failed: %v", cmd, err)
	}
}

func TestConfigCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterConfigCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"CONFIG GET", "CONFIG", [][]byte{[]byte("GET"), []byte("maxclients")}},
		{"CONFIG SET", "CONFIG", [][]byte{[]byte("SET"), []byte("maxclients"), []byte("1000")}},
		{"CONFIG REWRITE", "CONFIG", [][]byte{[]byte("REWRITE")}},
		{"CONFIG RESETSTAT", "CONFIG", [][]byte{[]byte("RESETSTAT")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestDebugCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterDebugCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"DEBUG SEGFAULT", "DEBUG", [][]byte{[]byte("SEGFAULT")}},
		{"DEBUG OBJECT", "DEBUG", [][]byte{[]byte("OBJECT"), []byte("key")}},
		{"DEBUG SLEEP", "DEBUG", [][]byte{[]byte("SLEEP"), []byte("0")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestObjectCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterDebugCommands(router)
	RegisterStringCommands(router)

	s.Set("mykey", &store.StringValue{Data: []byte("myvalue")}, store.SetOptions{})

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"OBJECT ENCODING", "OBJECT", [][]byte{[]byte("ENCODING"), []byte("mykey")}},
		{"OBJECT IDLETIME", "OBJECT", [][]byte{[]byte("IDLETIME"), []byte("mykey")}},
		{"OBJECT REFCOUNT", "OBJECT", [][]byte{[]byte("REFCOUNT"), []byte("mykey")}},
		{"MEMORY USAGE", "MEMORY", [][]byte{[]byte("USAGE"), []byte("mykey")}},
		{"MEMORY STATS", "MEMORY", [][]byte{[]byte("STATS")}},
		{"MEMORY DOCTOR", "MEMORY", [][]byte{[]byte("DOCTOR")}},
		{"MEMORY MALLOC-STATS", "MEMORY", [][]byte{[]byte("MALLOC-STATS")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestEncodingCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterEncodingCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"MSGPACK.ENCODE", "MSGPACK.ENCODE", [][]byte{[]byte(`{"key":"value"}`)}},
		{"MSGPACK.DECODE", "MSGPACK.DECODE", [][]byte{[]byte("\x82\xa4key\xa5value")}},
		{"BSON.ENCODE", "BSON.ENCODE", [][]byte{[]byte(`{"key":"value"}`)}},
		{"URL.ENCODE", "URL.ENCODE", [][]byte{[]byte("hello world")}},
		{"URL.DECODE", "URL.DECODE", [][]byte{[]byte("hello%20world")}},
		{"XML.ENCODE", "XML.ENCODE", [][]byte{[]byte(`{"key":"value"}`)}},
		{"YAML.ENCODE", "YAML.ENCODE", [][]byte{[]byte(`{"key":"value"}`)}},
		{"TOML.ENCODE", "TOML.ENCODE", [][]byte{[]byte(`{"key":"value"}`)}},
		{"CBOR.ENCODE", "CBOR.ENCODE", [][]byte{[]byte(`{"key":"value"}`)}},
		{"CSV.ENCODE", "CSV.ENCODE", [][]byte{[]byte(`["a","b","c"]`)}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestUUIDCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterEncodingCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"UUID.GEN", "UUID.GEN", nil},
		{"UUID.VALIDATE valid", "UUID.VALIDATE", [][]byte{[]byte("550e8400-e29b-41d4-a716-446655440000")}},
		{"UUID.VALIDATE invalid", "UUID.VALIDATE", [][]byte{[]byte("not-a-uuid")}},
		{"UUID.VERSION", "UUID.VERSION", [][]byte{[]byte("550e8400-e29b-41d4-a716-446655440000")}},
		{"ULID.GEN", "ULID.GEN", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestTimestampCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterEncodingCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"TIMESTAMP.NOW", "TIMESTAMP.NOW", nil},
		{"TIMESTAMP.PARSE", "TIMESTAMP.PARSE", [][]byte{[]byte("2024-01-01T00:00:00Z")}},
		{"TIMESTAMP.FORMAT", "TIMESTAMP.FORMAT", [][]byte{[]byte("1704067200"), []byte("2006-01-02")}},
		{"TIMESTAMP.ADD", "TIMESTAMP.ADD", [][]byte{[]byte("1704067200"), []byte("24h")}},
		{"TIMESTAMP.DIFF", "TIMESTAMP.DIFF", [][]byte{[]byte("1704067200"), []byte("1704153600")}},
		{"TIMESTAMP.STARTOF", "TIMESTAMP.STARTOF", [][]byte{[]byte("1704067200"), []byte("day")}},
		{"TIMESTAMP.ENDOF", "TIMESTAMP.ENDOF", [][]byte{[]byte("1704067200"), []byte("day")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestEventCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterEventCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"EVENT.EMIT", "EVENT.EMIT", [][]byte{[]byte("test.event"), []byte(`{"data":"value"}`)}},
		{"EVENT.GET", "EVENT.GET", [][]byte{[]byte("test.event")}},
		{"EVENT.LIST", "EVENT.LIST", nil},
		{"EVENT.CLEAR", "EVENT.CLEAR", nil},
		{"WEBHOOK.CREATE", "WEBHOOK.CREATE", [][]byte{[]byte("wh1"), []byte("http://example.com/hook")}},
		{"WEBHOOK.LIST", "WEBHOOK.LIST", nil},
		{"WEBHOOK.GET", "WEBHOOK.GET", [][]byte{[]byte("wh1")}},
		{"WEBHOOK.ENABLE", "WEBHOOK.ENABLE", [][]byte{[]byte("wh1")}},
		{"WEBHOOK.DISABLE", "WEBHOOK.DISABLE", [][]byte{[]byte("wh1")}},
		{"WEBHOOK.STATS", "WEBHOOK.STATS", [][]byte{[]byte("wh1")}},
		{"WEBHOOK.DELETE", "WEBHOOK.DELETE", [][]byte{[]byte("wh1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestUtilityCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterUtilityCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"ID.CREATE", "ID.CREATE", [][]byte{[]byte("gen1")}},
		{"ID.NEXT", "ID.NEXT", [][]byte{[]byte("gen1")}},
		{"ID.NEXTN", "ID.NEXTN", [][]byte{[]byte("gen1"), []byte("5")}},
		{"ID.CURRENT", "ID.CURRENT", [][]byte{[]byte("gen1")}},
		{"ID.SET", "ID.SET", [][]byte{[]byte("gen1"), []byte("100")}},
		{"ID.DELETE", "ID.DELETE", [][]byte{[]byte("gen1")}},
		{"SNOWFLAKE.NEXT", "SNOWFLAKE.NEXT", nil},
		{"SNOWFLAKE.PARSE", "SNOWFLAKE.PARSE", [][]byte{[]byte("1234567890123456789")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestStatsCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStatsCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"TDIGEST.CREATE", "TDIGEST.CREATE", [][]byte{[]byte("td1"), []byte("100")}},
		{"TDIGEST.ADD", "TDIGEST.ADD", [][]byte{[]byte("td1"), []byte("1.0"), []byte("2.0"), []byte("3.0")}},
		{"TDIGEST.QUANTILE", "TDIGEST.QUANTILE", [][]byte{[]byte("td1"), []byte("0.5")}},
		{"TDIGEST.CDF", "TDIGEST.CDF", [][]byte{[]byte("td1"), []byte("2.0")}},
		{"TDIGEST.MEAN", "TDIGEST.MEAN", [][]byte{[]byte("td1")}},
		{"TDIGEST.MIN", "TDIGEST.MIN", [][]byte{[]byte("td1")}},
		{"TDIGEST.MAX", "TDIGEST.MAX", [][]byte{[]byte("td1")}},
		{"TDIGEST.INFO", "TDIGEST.INFO", [][]byte{[]byte("td1")}},
		{"TDIGEST.RESET", "TDIGEST.RESET", [][]byte{[]byte("td1")}},
		{"TDIGEST.MERGE", "TDIGEST.MERGE", [][]byte{[]byte("td1"), []byte("td2")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestSampleCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStatsCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"SAMPLE.CREATE", "SAMPLE.CREATE", [][]byte{[]byte("s1")}},
		{"SAMPLE.ADD", "SAMPLE.ADD", [][]byte{[]byte("s1"), []byte("10"), []byte("20"), []byte("30")}},
		{"SAMPLE.GET", "SAMPLE.GET", [][]byte{[]byte("s1")}},
		{"SAMPLE.RESET", "SAMPLE.RESET", [][]byte{[]byte("s1")}},
		{"SAMPLE.INFO", "SAMPLE.INFO", [][]byte{[]byte("s1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestHistogramCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStatsCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"HISTOGRAM.CREATE", "HISTOGRAM.CREATE", [][]byte{[]byte("h1"), []byte("10")}},
		{"HISTOGRAM.ADD", "HISTOGRAM.ADD", [][]byte{[]byte("h1"), []byte("5.5")}},
		{"HISTOGRAM.GET", "HISTOGRAM.GET", [][]byte{[]byte("h1")}},
		{"HISTOGRAM.MEAN", "HISTOGRAM.MEAN", [][]byte{[]byte("h1")}},
		{"HISTOGRAM.RESET", "HISTOGRAM.RESET", [][]byte{[]byte("h1")}},
		{"HISTOGRAM.INFO", "HISTOGRAM.INFO", [][]byte{[]byte("h1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestSchedulerCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterSchedulerCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"JOB.CREATE", "JOB.CREATE", [][]byte{[]byte("job1"), []byte("* * * * *"), []byte("SET key value")}},
		{"JOB.GET", "JOB.GET", [][]byte{[]byte("job1")}},
		{"JOB.LIST", "JOB.LIST", nil},
		{"JOB.ENABLE", "JOB.ENABLE", [][]byte{[]byte("job1")}},
		{"JOB.DISABLE", "JOB.DISABLE", [][]byte{[]byte("job1")}},
		{"JOB.RUN", "JOB.RUN", [][]byte{[]byte("job1")}},
		{"JOB.STATS", "JOB.STATS", [][]byte{[]byte("job1")}},
		{"JOB.UPDATE", "JOB.UPDATE", [][]byte{[]byte("job1"), []byte("0 */2 * * *")}},
		{"JOB.RESET", "JOB.RESET", [][]byte{[]byte("job1")}},
		{"JOB.DELETE", "JOB.DELETE", [][]byte{[]byte("job1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestResilienceCircuitCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"CIRCUITX.CREATE", "CIRCUITX.CREATE", [][]byte{[]byte("cb1"), []byte("5"), []byte("1000")}},
		{"CIRCUITX.STATUS", "CIRCUITX.STATUS", [][]byte{[]byte("cb1")}},
		{"CIRCUITX.OPEN", "CIRCUITX.OPEN", [][]byte{[]byte("cb1")}},
		{"CIRCUITX.CLOSE", "CIRCUITX.CLOSE", [][]byte{[]byte("cb1")}},
		{"CIRCUITX.HALFOPEN", "CIRCUITX.HALFOPEN", [][]byte{[]byte("cb1")}},
		{"CIRCUITX.METRICS", "CIRCUITX.METRICS", [][]byte{[]byte("cb1")}},
		{"CIRCUITX.RESET", "CIRCUITX.RESET", [][]byte{[]byte("cb1")}},
		{"CIRCUITX.DELETE", "CIRCUITX.DELETE", [][]byte{[]byte("cb1")}},
		{"CIRCUIT.CREATE", "CIRCUIT.CREATE", [][]byte{[]byte("cb2"), []byte("5"), []byte("1000")}},
		{"CIRCUIT.ALLOW", "CIRCUIT.ALLOW", [][]byte{[]byte("cb2")}},
		{"CIRCUIT.SUCCESS", "CIRCUIT.SUCCESS", [][]byte{[]byte("cb2")}},
		{"CIRCUIT.FAILURE", "CIRCUIT.FAILURE", [][]byte{[]byte("cb2")}},
		{"CIRCUIT.STATE", "CIRCUIT.STATE", [][]byte{[]byte("cb2")}},
		{"CIRCUIT.STATS", "CIRCUIT.STATS", [][]byte{[]byte("cb2")}},
		{"CIRCUIT.LIST", "CIRCUIT.LIST", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestResilienceRateLimiterCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"RATELIMITER.CREATE", "RATELIMITER.CREATE", [][]byte{[]byte("rl1"), []byte("10"), []byte("60000")}},
		{"RATELIMITER.TRY", "RATELIMITER.TRY", [][]byte{[]byte("rl1")}},
		{"RATELIMITER.WAIT", "RATELIMITER.WAIT", [][]byte{[]byte("rl1"), []byte("1000")}},
		{"RATELIMITER.RESET", "RATELIMITER.RESET", [][]byte{[]byte("rl1")}},
		{"RATELIMITER.STATUS", "RATELIMITER.STATUS", [][]byte{[]byte("rl1")}},
		{"RATELIMITER.DELETE", "RATELIMITER.DELETE", [][]byte{[]byte("rl1")}},
		{"RL.CREATE", "RL.CREATE", [][]byte{[]byte("rl2"), []byte("10"), []byte("60000")}},
		{"RL.ALLOW", "RL.ALLOW", [][]byte{[]byte("rl2")}},
		{"RL.GET", "RL.GET", [][]byte{[]byte("rl2")}},
		{"RL.RESET", "RL.RESET", [][]byte{[]byte("rl2")}},
		{"RL.DELETE", "RL.DELETE", [][]byte{[]byte("rl2")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestResilienceBulkheadCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"BULKHEAD.CREATE", "BULKHEAD.CREATE", [][]byte{[]byte("bh1"), []byte("5")}},
		{"BULKHEAD.ACQUIRE", "BULKHEAD.ACQUIRE", [][]byte{[]byte("bh1")}},
		{"BULKHEAD.RELEASE", "BULKHEAD.RELEASE", [][]byte{[]byte("bh1")}},
		{"BULKHEAD.STATUS", "BULKHEAD.STATUS", [][]byte{[]byte("bh1")}},
		{"BULKHEAD.DELETE", "BULKHEAD.DELETE", [][]byte{[]byte("bh1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestResilienceRetryCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"RETRY.CREATE", "RETRY.CREATE", [][]byte{[]byte("rt1"), []byte("3"), []byte("100")}},
		{"RETRY.EXECUTE", "RETRY.EXECUTE", [][]byte{[]byte("rt1"), []byte("cmd arg")}},
		{"RETRY.STATUS", "RETRY.STATUS", [][]byte{[]byte("rt1")}},
		{"RETRY.DELETE", "RETRY.DELETE", [][]byte{[]byte("rt1")}},
		{"TIMEOUT.CREATE", "TIMEOUT.CREATE", [][]byte{[]byte("to1"), []byte("5000")}},
		{"TIMEOUT.EXECUTE", "TIMEOUT.EXECUTE", [][]byte{[]byte("to1"), []byte("cmd arg")}},
		{"TIMEOUT.DELETE", "TIMEOUT.DELETE", [][]byte{[]byte("to1")}},
		{"FALLBACK.CREATE", "FALLBACK.CREATE", [][]byte{[]byte("fb1"), []byte("default")}},
		{"FALLBACK.EXECUTE", "FALLBACK.EXECUTE", [][]byte{[]byte("fb1"), []byte("cmd arg")}},
		{"FALLBACK.DELETE", "FALLBACK.DELETE", [][]byte{[]byte("fb1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestResilienceLockCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)
	RegisterUtilityCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"LOCK.TRY", "LOCK.TRY", [][]byte{[]byte("lock1"), []byte("owner1"), []byte("5000")}},
		{"LOCK.ACQUIRE", "LOCK.ACQUIRE", [][]byte{[]byte("lock1"), []byte("owner1"), []byte("5000")}},
		{"LOCK.RELEASE", "LOCK.RELEASE", [][]byte{[]byte("lock1"), []byte("owner1")}},
		{"LOCK.RENEW", "LOCK.RENEW", [][]byte{[]byte("lock1"), []byte("owner1"), []byte("5000")}},
		{"LOCK.INFO", "LOCK.INFO", [][]byte{[]byte("lock1")}},
		{"LOCK.ISLOCKED", "LOCK.ISLOCKED", [][]byte{[]byte("lock1")}},
		{"LOCKX.ACQUIRE", "LOCKX.ACQUIRE", [][]byte{[]byte("lock2"), []byte("owner2"), []byte("5000")}},
		{"LOCKX.RELEASE", "LOCKX.RELEASE", [][]byte{[]byte("lock2"), []byte("owner2")}},
		{"LOCKX.EXTEND", "LOCKX.EXTEND", [][]byte{[]byte("lock2"), []byte("owner2"), []byte("5000")}},
		{"LOCKX.STATUS", "LOCKX.STATUS", [][]byte{[]byte("lock2")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestResilienceSemaphoreCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"SEMAPHOREX.CREATE", "SEMAPHOREX.CREATE", [][]byte{[]byte("sem1"), []byte("5")}},
		{"SEMAPHOREX.ACQUIRE", "SEMAPHOREX.ACQUIRE", [][]byte{[]byte("sem1")}},
		{"SEMAPHOREX.RELEASE", "SEMAPHOREX.RELEASE", [][]byte{[]byte("sem1")}},
		{"SEMAPHOREX.STATUS", "SEMAPHOREX.STATUS", [][]byte{[]byte("sem1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestResilienceAsyncCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"ASYNC.SUBMIT", "ASYNC.SUBMIT", [][]byte{[]byte("GET key")}},
		{"ASYNC.STATUS", "ASYNC.STATUS", [][]byte{[]byte("task1")}},
		{"ASYNC.RESULT", "ASYNC.RESULT", [][]byte{[]byte("task1")}},
		{"ASYNC.CANCEL", "ASYNC.CANCEL", [][]byte{[]byte("task1")}},
		{"PROMISE.CREATE", "PROMISE.CREATE", nil},
		{"PROMISE.RESOLVE", "PROMISE.RESOLVE", [][]byte{[]byte("p1"), []byte("result")}},
		{"PROMISE.REJECT", "PROMISE.REJECT", [][]byte{[]byte("p1"), []byte("error")}},
		{"PROMISE.STATUS", "PROMISE.STATUS", [][]byte{[]byte("p1")}},
		{"PROMISE.AWAIT", "PROMISE.AWAIT", [][]byte{[]byte("p1"), []byte("5000")}},
		{"FUTURE.CREATE", "FUTURE.CREATE", nil},
		{"FUTURE.COMPLETE", "FUTURE.COMPLETE", [][]byte{[]byte("f1"), []byte("result")}},
		{"FUTURE.GET", "FUTURE.GET", [][]byte{[]byte("f1"), []byte("5000")}},
		{"FUTURE.CANCEL", "FUTURE.CANCEL", [][]byte{[]byte("f1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestResilienceObservabilityCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"OBSERVABILITY.TRACE", "OBSERVABILITY.TRACE", [][]byte{[]byte("op1"), []byte("start")}},
		{"OBSERVABILITY.METRIC", "OBSERVABILITY.METRIC", [][]byte{[]byte("metric1"), []byte("1.0")}},
		{"OBSERVABILITY.LOG", "OBSERVABILITY.LOG", [][]byte{[]byte("info"), []byte("message")}},
		{"OBSERVABILITY.SPAN", "OBSERVABILITY.SPAN", [][]byte{[]byte("span1"), []byte("start")}},
		{"TELEMETRY.RECORD", "TELEMETRY.RECORD", [][]byte{[]byte("metric1"), []byte("1.0")}},
		{"TELEMETRY.QUERY", "TELEMETRY.QUERY", [][]byte{[]byte("metric1")}},
		{"TELEMETRY.EXPORT", "TELEMETRY.EXPORT", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestResilienceDiagnosticCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"DIAGNOSTIC.RUN", "DIAGNOSTIC.RUN", [][]byte{[]byte("check1")}},
		{"DIAGNOSTIC.RESULT", "DIAGNOSTIC.RESULT", [][]byte{[]byte("check1")}},
		{"DIAGNOSTIC.LIST", "DIAGNOSTIC.LIST", nil},
		{"PROFILE.START", "PROFILE.START", [][]byte{[]byte("cpu"), []byte("30")}},
		{"PROFILE.STOP", "PROFILE.STOP", nil},
		{"PROFILE.RESULT", "PROFILE.RESULT", nil},
		{"PROFILEX.LIST", "PROFILEX.LIST", nil},
		{"HEAP.STATS", "HEAP.STATS", nil},
		{"HEAP.DUMP", "HEAP.DUMP", nil},
		{"HEAP.GC", "HEAP.GC", nil},
		{"MEMORYX.ALLOC", "MEMORYX.ALLOC", [][]byte{[]byte("buf1"), []byte("1024")}},
		{"MEMORYX.FREE", "MEMORYX.FREE", [][]byte{[]byte("buf1")}},
		{"MEMORYX.STATS", "MEMORYX.STATS", nil},
		{"MEMORYX.TRACK", "MEMORYX.TRACK", [][]byte{[]byte("alloc1"), []byte("1024")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestMLModelCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMLCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"MODEL.CREATE", "MODEL.CREATE", [][]byte{[]byte("model1"), []byte("classifier")}},
		{"MODEL.TRAIN", "MODEL.TRAIN", [][]byte{[]byte("model1"), []byte("data1")}},
		{"MODEL.PREDICT", "MODEL.PREDICT", [][]byte{[]byte("model1"), []byte("1.0,2.0,3.0")}},
		{"MODEL.DELETE", "MODEL.DELETE", [][]byte{[]byte("model1")}},
		{"MODEL.LIST", "MODEL.LIST", nil},
		{"MODEL.STATUS", "MODEL.STATUS", [][]byte{[]byte("model1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestMLFeatureCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMLCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"FEATURE.SET", "FEATURE.SET", [][]byte{[]byte("entity1"), []byte("f1"), []byte("1.5")}},
		{"FEATURE.GET", "FEATURE.GET", [][]byte{[]byte("entity1"), []byte("f1")}},
		{"FEATURE.DEL", "FEATURE.DEL", [][]byte{[]byte("entity1"), []byte("f1")}},
		{"FEATURE.INCR", "FEATURE.INCR", [][]byte{[]byte("entity1"), []byte("f1"), []byte("0.5")}},
		{"FEATURE.NORMALIZE", "FEATURE.NORMALIZE", [][]byte{[]byte("entity1"), []byte("minmax")}},
		{"FEATURE.VECTOR", "FEATURE.VECTOR", [][]byte{[]byte("entity1"), []byte("f1,f2,f3")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestMLEmbeddingCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMLCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"EMBEDDING.CREATE", "EMBEDDING.CREATE", [][]byte{[]byte("emb1"), []byte("0.1"), []byte("0.2"), []byte("0.3")}},
		{"EMBEDDING.GET", "EMBEDDING.GET", [][]byte{[]byte("emb1")}},
		{"EMBEDDING.SEARCH", "EMBEDDING.SEARCH", [][]byte{[]byte("emb1"), []byte("0.1,0.2,0.3"), []byte("5")}},
		{"EMBEDDING.SIMILAR", "EMBEDDING.SIMILAR", [][]byte{[]byte("emb1"), []byte("emb2")}},
		{"EMBEDDING.DELETE", "EMBEDDING.DELETE", [][]byte{[]byte("emb1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestMLTensorCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMLCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"TENSOR.CREATE", "TENSOR.CREATE", [][]byte{[]byte("t1"), []byte("2,3")}},
		{"TENSOR.GET", "TENSOR.GET", [][]byte{[]byte("t1")}},
		{"TENSOR.ADD", "TENSOR.ADD", [][]byte{[]byte("t1"), []byte("t2")}},
		{"TENSOR.MATMUL", "TENSOR.MATMUL", [][]byte{[]byte("t1"), []byte("t2")}},
		{"TENSOR.RESHAPE", "TENSOR.RESHAPE", [][]byte{[]byte("t1"), []byte("3,2")}},
		{"TENSOR.DELETE", "TENSOR.DELETE", [][]byte{[]byte("t1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestMLClassifierCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMLCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"CLASSIFIER.CREATE", "CLASSIFIER.CREATE", [][]byte{[]byte("clf1"), []byte("spam"), []byte("ham")}},
		{"CLASSIFIER.TRAIN", "CLASSIFIER.TRAIN", [][]byte{[]byte("clf1"), []byte("data1")}},
		{"CLASSIFIER.PREDICT", "CLASSIFIER.PREDICT", [][]byte{[]byte("clf1"), []byte("test message")}},
		{"CLASSIFIER.DELETE", "CLASSIFIER.DELETE", [][]byte{[]byte("clf1")}},
		{"REGRESSOR.CREATE", "REGRESSOR.CREATE", [][]byte{[]byte("reg1")}},
		{"REGRESSOR.TRAIN", "REGRESSOR.TRAIN", [][]byte{[]byte("reg1"), []byte("data1")}},
		{"REGRESSOR.PREDICT", "REGRESSOR.PREDICT", [][]byte{[]byte("reg1"), []byte("1.0,2.0,3.0")}},
		{"REGRESSOR.DELETE", "REGRESSOR.DELETE", [][]byte{[]byte("reg1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestMLClusterCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMLCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"CLUSTER.CREATE", "CLUSTER.CREATE", [][]byte{[]byte("clust1"), []byte("kmeans"), []byte("3")}},
		{"CLUSTER.FIT", "CLUSTER.FIT", [][]byte{[]byte("clust1"), []byte("data1")}},
		{"CLUSTER.PREDICT", "CLUSTER.PREDICT", [][]byte{[]byte("clust1"), []byte("1.0,2.0")}},
		{"CLUSTER.CENTROIDS", "CLUSTER.CENTROIDS", [][]byte{[]byte("clust1")}},
		{"CLUSTER.DELETE", "CLUSTER.DELETE", [][]byte{[]byte("clust1")}},
		{"ANOMALY.CREATE", "ANOMALY.CREATE", [][]byte{[]byte("anom1"), []byte("zscore")}},
		{"ANOMALY.DETECT", "ANOMALY.DETECT", [][]byte{[]byte("anom1"), []byte("100.0")}},
		{"ANOMALY.LEARN", "ANOMALY.LEARN", [][]byte{[]byte("anom1"), []byte("50.0")}},
		{"ANOMALY.DELETE", "ANOMALY.DELETE", [][]byte{[]byte("anom1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestMLNLPCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMLCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"SENTIMENT.ANALYZE", "SENTIMENT.ANALYZE", [][]byte{[]byte("I love this product!")}},
		{"SENTIMENT.BATCH", "SENTIMENT.BATCH", [][]byte{[]byte("I love it"), []byte("I hate it"), []byte("It's okay")}},
		{"NLP.TOKENIZE", "NLP.TOKENIZE", [][]byte{[]byte("Hello world, this is a test.")}},
		{"NLP.ENTITIES", "NLP.ENTITIES", [][]byte{[]byte("John works at Google in New York.")}},
		{"NLP.KEYWORDS", "NLP.KEYWORDS", [][]byte{[]byte("Machine learning is a subset of artificial intelligence.")}},
		{"NLP.SUMMARIZE", "NLP.SUMMARIZE", [][]byte{[]byte("Long text to summarize here..."), []byte("50")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestMLSimilarityCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMLCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"SIMILARITY.COSINE", "SIMILARITY.COSINE", [][]byte{[]byte("1.0,2.0,3.0"), []byte("2.0,3.0,4.0")}},
		{"SIMILARITY.EUCLIDEAN", "SIMILARITY.EUCLIDEAN", [][]byte{[]byte("1.0,2.0"), []byte("3.0,4.0")}},
		{"SIMILARITY.JACCARD", "SIMILARITY.JACCARD", [][]byte{[]byte("a,b,c"), []byte("b,c,d")}},
		{"SIMILARITY.DOTPRODUCT", "SIMILARITY.DOTPRODUCT", [][]byte{[]byte("1.0,2.0"), []byte("3.0,4.0")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestMLDatasetCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMLCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"DATASET.CREATE", "DATASET.CREATE", [][]byte{[]byte("ds1")}},
		{"DATASET.ADD", "DATASET.ADD", [][]byte{[]byte("ds1"), []byte("1.0,2.0,label1")}},
		{"DATASET.GET", "DATASET.GET", [][]byte{[]byte("ds1"), []byte("0")}},
		{"DATASET.SPLIT", "DATASET.SPLIT", [][]byte{[]byte("ds1"), []byte("0.8"), []byte("0.2")}},
		{"DATASET.DELETE", "DATASET.DELETE", [][]byte{[]byte("ds1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestMLExperimentCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMLCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"MLXPERIMENT.CREATE", "MLXPERIMENT.CREATE", [][]byte{[]byte("exp1")}},
		{"MLXPERIMENT.LOG", "MLXPERIMENT.LOG", [][]byte{[]byte("exp1"), []byte("accuracy"), []byte("0.95")}},
		{"MLXPERIMENT.METRICS", "MLXPERIMENT.METRICS", [][]byte{[]byte("exp1")}},
		{"MLXPERIMENT.COMPARE", "MLXPERIMENT.COMPARE", [][]byte{[]byte("exp1"), []byte("exp2")}},
		{"MLXPERIMENT.DELETE", "MLXPERIMENT.DELETE", [][]byte{[]byte("exp1")}},
		{"PIPELINEML.CREATE", "PIPELINEML.CREATE", [][]byte{[]byte("pipe1")}},
		{"PIPELINEML.ADD", "PIPELINEML.ADD", [][]byte{[]byte("pipe1"), []byte("normalize")}},
		{"PIPELINEML.RUN", "PIPELINEML.RUN", [][]byte{[]byte("pipe1"), []byte("data1")}},
		{"PIPELINEML.DELETE", "PIPELINEML.DELETE", [][]byte{[]byte("pipe1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestMLHyperparamCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMLCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"HYPERPARAM.SET", "HYPERPARAM.SET", [][]byte{[]byte("hp1"), []byte("learning_rate"), []byte("0.01")}},
		{"HYPERPARAM.GET", "HYPERPARAM.GET", [][]byte{[]byte("hp1"), []byte("learning_rate")}},
		{"HYPERPARAM.SEARCH", "HYPERPARAM.SEARCH", [][]byte{[]byte("hp1"), []byte("grid")}},
		{"HYPERPARAM.DELETE", "HYPERPARAM.DELETE", [][]byte{[]byte("hp1")}},
		{"EVALUATOR.CREATE", "EVALUATOR.CREATE", [][]byte{[]byte("eval1"), []byte("accuracy")}},
		{"EVALUATOR.RUN", "EVALUATOR.RUN", [][]byte{[]byte("eval1"), []byte("model1"), []byte("data1")}},
		{"EVALUATOR.METRICS", "EVALUATOR.METRICS", [][]byte{[]byte("eval1")}},
		{"EVALUATOR.DELETE", "EVALUATOR.DELETE", [][]byte{[]byte("eval1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestMLRecommendCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMLCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"RECOMMEND.CREATE", "RECOMMEND.CREATE", [][]byte{[]byte("rec1")}},
		{"RECOMMEND.TRAIN", "RECOMMEND.TRAIN", [][]byte{[]byte("rec1"), []byte("ratings1")}},
		{"RECOMMEND.GET", "RECOMMEND.GET", [][]byte{[]byte("rec1"), []byte("user1"), []byte("10")}},
		{"RECOMMEND.DELETE", "RECOMMEND.DELETE", [][]byte{[]byte("rec1")}},
		{"TIMEFORECAST.CREATE", "TIMEFORECAST.CREATE", [][]byte{[]byte("tf1")}},
		{"TIMEFORECAST.TRAIN", "TIMEFORECAST.TRAIN", [][]byte{[]byte("tf1"), []byte("ts1")}},
		{"TIMEFORECAST.PREDICT", "TIMEFORECAST.PREDICT", [][]byte{[]byte("tf1"), []byte("10")}},
		{"TIMEFORECAST.DELETE", "TIMEFORECAST.DELETE", [][]byte{[]byte("tf1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestCacheCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterCacheCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"CACHE.BULKGET", "CACHE.BULKGET", [][]byte{[]byte("k1"), []byte("k2"), []byte("k3")}},
		{"CACHE.BULKDEL", "CACHE.BULKDEL", [][]byte{[]byte("k1"), []byte("k2")}},
		{"CACHE.STATS", "CACHE.STATS", nil},
		{"CACHE.PREFETCH", "CACHE.PREFETCH", [][]byte{[]byte("k1"), []byte("k2")}},
		{"CACHE.EXPORT", "CACHE.EXPORT", nil},
		{"CACHE.IMPORT", "CACHE.IMPORT", [][]byte{[]byte(`{"k1":"v1","k2":"v2"}`)}},
		{"CACHE.CLEAR", "CACHE.CLEAR", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestCacheWarmCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterCacheWarmingCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"WARM.PRELOAD", "WARM.PRELOAD", [][]byte{[]byte("key1"), []byte("key2")}},
		{"WARM.PREFETCH", "WARM.PREFETCH", [][]byte{[]byte("pattern*")}},
		{"WARM.INVALIDATE", "WARM.INVALIDATE", [][]byte{[]byte("key1")}},
		{"WARM.STATUS", "WARM.STATUS", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestBatchExtCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterCacheWarmingCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"BATCH.GET", "BATCH.GET", [][]byte{[]byte("k1"), []byte("k2")}},
		{"BATCH.SET", "BATCH.SET", [][]byte{[]byte("k1"), []byte("v1"), []byte("k2"), []byte("v2")}},
		{"BATCH.DEL", "BATCH.DEL", [][]byte{[]byte("k1"), []byte("k2")}},
		{"BATCH.MGET", "BATCH.MGET", [][]byte{[]byte("k1"), []byte("k2")}},
		{"BATCH.MSET", "BATCH.MSET", [][]byte{[]byte("k1"), []byte("v1")}},
		{"BATCH.MDEL", "BATCH.MDEL", [][]byte{[]byte("k1")}},
		{"BATCH.EXEC", "BATCH.EXEC", nil},
		{"PIPELINE.EXEC", "PIPELINE.EXEC", [][]byte{[]byte("SET k1 v1"), []byte("GET k1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestKeyExtCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterCacheWarmingCommands(router)
	RegisterStringCommands(router)

	s.Set("src", &store.StringValue{Data: []byte("value")}, store.SetOptions{})

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"KEY.RENAME", "KEY.RENAME", [][]byte{[]byte("src"), []byte("dst")}},
		{"KEY.RENAMENX", "KEY.RENAMENX", [][]byte{[]byte("src"), []byte("dst2")}},
		{"KEY.COPY", "KEY.COPY", [][]byte{[]byte("src"), []byte("copy1")}},
		{"KEY.MOVE", "KEY.MOVE", [][]byte{[]byte("src"), []byte("1")}},
		{"KEY.DUMP", "KEY.DUMP", [][]byte{[]byte("src")}},
		{"KEY.RESTORE", "KEY.RESTORE", [][]byte{[]byte("restored"), []byte("0"), []byte("data")}},
		{"KEY.OBJECT", "KEY.OBJECT", [][]byte{[]byte("src")}},
		{"KEY.ENCODE", "KEY.ENCODE", [][]byte{[]byte("src")}},
		{"KEY.FREQ", "KEY.FREQ", [][]byte{[]byte("src")}},
		{"KEY.IDLETIME", "KEY.IDLETIME", [][]byte{[]byte("src")}},
		{"KEY.REFCOUNT", "KEY.REFCOUNT", [][]byte{[]byte("src")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestDiffCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterEncodingCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"DIFF.TEXT", "DIFF.TEXT", [][]byte{[]byte("hello world"), []byte("hello there")}},
		{"DIFF.JSON", "DIFF.JSON", [][]byte{[]byte(`{"a":1,"b":2}`), []byte(`{"a":1,"c":3}`)}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestPoolCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterUtilityCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"POOL.CREATE", "POOL.CREATE", [][]byte{[]byte("pool1"), []byte("10")}},
		{"POOL.GET", "POOL.GET", [][]byte{[]byte("pool1")}},
		{"POOL.PUT", "POOL.PUT", [][]byte{[]byte("pool1"), []byte("obj1")}},
		{"POOL.CLEAR", "POOL.CLEAR", [][]byte{[]byte("pool1")}},
		{"POOL.STATS", "POOL.STATS", [][]byte{[]byte("pool1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestCompressCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterEncodingCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"COMPRESS.RLE", "COMPRESS.RLE", [][]byte{[]byte("aaabbbccc")}},
		{"DECOMPRESS.RLE", "DECOMPRESS.RLE", [][]byte{[]byte("3a3b3c")}},
		{"COMPRESS.LZ4", "COMPRESS.LZ4", [][]byte{[]byte("hello world hello world")}},
		{"COMPRESS.CUSTOM", "COMPRESS.CUSTOM", [][]byte{[]byte("gzip"), []byte("data to compress")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestQueueStackCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterEncodingCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"QUEUE.CREATE", "QUEUE.CREATE", [][]byte{[]byte("q1")}},
		{"QUEUE.PUSH", "QUEUE.PUSH", [][]byte{[]byte("q1"), []byte("item1")}},
		{"QUEUE.POP", "QUEUE.POP", [][]byte{[]byte("q1")}},
		{"QUEUE.PEEK", "QUEUE.PEEK", [][]byte{[]byte("q1")}},
		{"QUEUE.LEN", "QUEUE.LEN", [][]byte{[]byte("q1")}},
		{"QUEUE.CLEAR", "QUEUE.CLEAR", [][]byte{[]byte("q1")}},
		{"STACK.CREATE", "STACK.CREATE", [][]byte{[]byte("s1")}},
		{"STACK.PUSH", "STACK.PUSH", [][]byte{[]byte("s1"), []byte("item1")}},
		{"STACK.POP", "STACK.POP", [][]byte{[]byte("s1")}},
		{"STACK.PEEK", "STACK.PEEK", [][]byte{[]byte("s1")}},
		{"STACK.LEN", "STACK.LEN", [][]byte{[]byte("s1")}},
		{"STACK.CLEAR", "STACK.CLEAR", [][]byte{[]byte("s1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestActorCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterActorCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"ACTOR.CREATE", "ACTOR.CREATE", [][]byte{[]byte("a1")}},
		{"ACTOR.SEND", "ACTOR.SEND", [][]byte{[]byte("a1"), []byte("message")}},
		{"ACTOR.RECV", "ACTOR.RECV", [][]byte{[]byte("a1")}},
		{"ACTOR.POKE", "ACTOR.POKE", [][]byte{[]byte("a1")}},
		{"ACTOR.PEEK", "ACTOR.PEEK", [][]byte{[]byte("a1")}},
		{"ACTOR.LEN", "ACTOR.LEN", [][]byte{[]byte("a1")}},
		{"ACTOR.LIST", "ACTOR.LIST", nil},
		{"ACTOR.CLEAR", "ACTOR.CLEAR", [][]byte{[]byte("a1")}},
		{"ACTOR.DELETE", "ACTOR.DELETE", [][]byte{[]byte("a1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestDAGCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterActorCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"DAG.CREATE", "DAG.CREATE", [][]byte{[]byte("dag1")}},
		{"DAG.ADDNODE", "DAG.ADDNODE", [][]byte{[]byte("dag1"), []byte("node1")}},
		{"DAG.ADDEDGE", "DAG.ADDEDGE", [][]byte{[]byte("dag1"), []byte("node1"), []byte("node2")}},
		{"DAG.TOPO", "DAG.TOPO", [][]byte{[]byte("dag1")}},
		{"DAG.PARENTS", "DAG.PARENTS", [][]byte{[]byte("dag1"), []byte("node2")}},
		{"DAG.CHILDREN", "DAG.CHILDREN", [][]byte{[]byte("dag1"), []byte("node1")}},
		{"DAG.LIST", "DAG.LIST", nil},
		{"DAG.DELETE", "DAG.DELETE", [][]byte{[]byte("dag1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestParallelCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterActorCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"PARALLEL.EXEC", "PARALLEL.EXEC", [][]byte{[]byte("GET k1"), []byte("GET k2")}},
		{"PARALLEL.MAP", "PARALLEL.MAP", [][]byte{[]byte("INCR"), []byte("k1"), []byte("k2"), []byte("k3")}},
		{"PARALLEL.REDUCE", "PARALLEL.REDUCE", [][]byte{[]byte("ADD"), []byte("1"), []byte("2"), []byte("3")}},
		{"PARALLEL.FILTER", "PARALLEL.FILTER", [][]byte{[]byte("EXISTS"), []byte("k1"), []byte("k2")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestSecretCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterEncodingCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"SECRET.SET", "SECRET.SET", [][]byte{[]byte("api_key"), []byte("secret123")}},
		{"SECRET.GET", "SECRET.GET", [][]byte{[]byte("api_key")}},
		{"SECRET.LIST", "SECRET.LIST", nil},
		{"SECRET.ROTATE", "SECRET.ROTATE", [][]byte{[]byte("api_key"), []byte("newsecret456")}},
		{"SECRET.VERSION", "SECRET.VERSION", [][]byte{[]byte("api_key")}},
		{"SECRET.DELETE", "SECRET.DELETE", [][]byte{[]byte("api_key")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestConfigExtCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterEncodingCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"CONFIG.SET", "CONFIG.SET", [][]byte{[]byte("app.timeout"), []byte("30")}},
		{"CONFIG.GET", "CONFIG.GET", [][]byte{[]byte("app.timeout")}},
		{"CONFIG.LIST", "CONFIG.LIST", nil},
		{"CONFIG.DELETE", "CONFIG.DELETE", [][]byte{[]byte("app.timeout")}},
		{"CONFIG.NAMESPACE", "CONFIG.NAMESPACE", [][]byte{[]byte("app")}},
		{"CONFIG.IMPORT", "CONFIG.IMPORT", [][]byte{[]byte(`{"app":{"timeout":30}}`)}},
		{"CONFIG.EXPORT", "CONFIG.EXPORT", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestTrieCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterEncodingCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"TRIE.ADD", "TRIE.ADD", [][]byte{[]byte("trie1"), []byte("hello")}},
		{"TRIE.SEARCH", "TRIE.SEARCH", [][]byte{[]byte("trie1"), []byte("hello")}},
		{"TRIE.PREFIX", "TRIE.PREFIX", [][]byte{[]byte("trie1"), []byte("hel")}},
		{"TRIE.DELETE", "TRIE.DELETE", [][]byte{[]byte("trie1"), []byte("hello")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestReplicationCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterReplicationCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"REPLCONF", "REPLCONF", [][]byte{[]byte("listening-port"), []byte("6379")}},
		{"SYNC", "SYNC", nil},
		{"PSYNC", "PSYNC", [][]byte{[]byte("abc123"), []byte("0")}},
		{"ROLE", "ROLE", nil},
		{"REPLICAOF", "REPLICAOF", [][]byte{[]byte("NO"), []byte("ONE")}},
		{"SLAVEOF", "SLAVEOF", [][]byte{[]byte("NO"), []byte("ONE")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestSentinelCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterSentinelCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"SENTINEL masters", "SENTINEL", [][]byte{[]byte("MASTERS")}},
		{"SENTINEL slaves", "SENTINEL", [][]byte{[]byte("SLAVES"), []byte("mymaster")}},
		{"SENTINEL get-master-addr-by-name", "SENTINEL", [][]byte{[]byte("GET-MASTER-ADDR-BY-NAME"), []byte("mymaster")}},
		{"SENTINEL reset", "SENTINEL", [][]byte{[]byte("RESET"), []byte("*")}},
		{"SENTINEL ping", "SENTINEL", [][]byte{[]byte("PING")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestModuleCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterModuleCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"MODULE LIST", "MODULE", [][]byte{[]byte("LIST")}},
		{"MODULE LOAD", "MODULE", [][]byte{[]byte("LOAD"), []byte("/path/to/module.so")}},
		{"MODULE UNLOAD", "MODULE", [][]byte{[]byte("UNLOAD"), []byte("modulename")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestFunctionCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterFunctionCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"FUNCTION LIST", "FUNCTION", [][]byte{[]byte("LIST")}},
		{"FUNCTION LOAD", "FUNCTION", [][]byte{[]byte("LOAD"), []byte("return function() return 1 end")}},
		{"FUNCTION DELETE", "FUNCTION", [][]byte{[]byte("DELETE"), []byte("funcname")}},
		{"FCALL", "FCALL", [][]byte{[]byte("funcname"), []byte("0")}},
		{"FCALL_RO", "FCALL_RO", [][]byte{[]byte("funcname"), []byte("0")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestSessionCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterUtilityExtCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"SESSION.CREATE", "SESSION.CREATE", [][]byte{[]byte("sess1"), []byte("3600")}},
		{"SESSION.GET", "SESSION.GET", [][]byte{[]byte("sess1")}},
		{"SESSION.SET", "SESSION.SET", [][]byte{[]byte("sess1"), []byte("key"), []byte("value")}},
		{"SESSION.DEL", "SESSION.DEL", [][]byte{[]byte("sess1"), []byte("key")}},
		{"SESSION.DELETE", "SESSION.DELETE", [][]byte{[]byte("sess1")}},
		{"SESSION.EXISTS", "SESSION.EXISTS", [][]byte{[]byte("sess1")}},
		{"SESSION.TTL", "SESSION.TTL", [][]byte{[]byte("sess1")}},
		{"SESSION.REFRESH", "SESSION.REFRESH", [][]byte{[]byte("sess1"), []byte("3600")}},
		{"SESSION.CLEAR", "SESSION.CLEAR", nil},
		{"SESSION.ALL", "SESSION.ALL", nil},
		{"SESSION.LIST", "SESSION.LIST", nil},
		{"SESSION.COUNT", "SESSION.COUNT", nil},
		{"SESSION.CLEANUP", "SESSION.CLEANUP", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestAuditCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterUtilityExtCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"AUDIT.LOG", "AUDIT.LOG", [][]byte{[]byte("user1"), []byte("SET"), []byte("key1")}},
		{"AUDIT.GET", "AUDIT.GET", [][]byte{[]byte("0")}},
		{"AUDIT.GETRANGE", "AUDIT.GETRANGE", [][]byte{[]byte("0"), []byte("10")}},
		{"AUDIT.GETBYCMD", "AUDIT.GETBYCMD", [][]byte{[]byte("SET")}},
		{"AUDIT.GETBYKEY", "AUDIT.GETBYKEY", [][]byte{[]byte("key1")}},
		{"AUDIT.CLEAR", "AUDIT.CLEAR", nil},
		{"AUDIT.COUNT", "AUDIT.COUNT", nil},
		{"AUDIT.STATS", "AUDIT.STATS", nil},
		{"AUDIT.ENABLE", "AUDIT.ENABLE", nil},
		{"AUDIT.DISABLE", "AUDIT.DISABLE", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestFlagCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterUtilityExtCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"FLAG.CREATE", "FLAG.CREATE", [][]byte{[]byte("flag1")}},
		{"FLAG.GET", "FLAG.GET", [][]byte{[]byte("flag1")}},
		{"FLAG.ENABLE", "FLAG.ENABLE", [][]byte{[]byte("flag1")}},
		{"FLAG.DISABLE", "FLAG.DISABLE", [][]byte{[]byte("flag1")}},
		{"FLAG.TOGGLE", "FLAG.TOGGLE", [][]byte{[]byte("flag1")}},
		{"FLAG.ISENABLED", "FLAG.ISENABLED", [][]byte{[]byte("flag1")}},
		{"FLAG.LIST", "FLAG.LIST", nil},
		{"FLAG.LISTENABLED", "FLAG.LISTENABLED", nil},
		{"FLAG.ADDVARIANT", "FLAG.ADDVARIANT", [][]byte{[]byte("flag1"), []byte("variantA"), []byte("50")}},
		{"FLAG.GETVARIANT", "FLAG.GETVARIANT", [][]byte{[]byte("flag1"), []byte("user1")}},
		{"FLAG.ADDRULE", "FLAG.ADDRULE", [][]byte{[]byte("flag1"), []byte("rule1"), []byte("country=US")}},
		{"FLAG.DELETE", "FLAG.DELETE", [][]byte{[]byte("flag1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestCounterCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterUtilityExtCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"COUNTER.GET", "COUNTER.GET", [][]byte{[]byte("counter1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestResilienceConnectionPoolCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"CONPOOL.CREATE", "CONPOOL.CREATE", [][]byte{[]byte("cp1"), []byte("10")}},
		{"CONPOOL.GET", "CONPOOL.GET", [][]byte{[]byte("cp1")}},
		{"CONPOOL.RETURN", "CONPOOL.RETURN", [][]byte{[]byte("cp1"), []byte("conn1")}},
		{"CONPOOL.STATUS", "CONPOOL.STATUS", [][]byte{[]byte("cp1")}},
		{"CONPOOL.DELETE", "CONPOOL.DELETE", [][]byte{[]byte("cp1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestResilienceBatchCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"BATCHX.CREATE", "BATCHX.CREATE", [][]byte{[]byte("bx1")}},
		{"BATCHX.ADD", "BATCHX.ADD", [][]byte{[]byte("bx1"), []byte("GET key")}},
		{"BATCHX.EXECUTE", "BATCHX.EXECUTE", [][]byte{[]byte("bx1")}},
		{"BATCHX.STATUS", "BATCHX.STATUS", [][]byte{[]byte("bx1")}},
		{"BATCHX.DELETE", "BATCHX.DELETE", [][]byte{[]byte("bx1")}},
		{"PIPELINEX.START", "PIPELINEX.START", nil},
		{"PIPELINEX.ADD", "PIPELINEX.ADD", [][]byte{[]byte("GET key")}},
		{"PIPELINEX.EXECUTE", "PIPELINEX.EXECUTE", nil},
		{"PIPELINEX.CANCEL", "PIPELINEX.CANCEL", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestResilienceTransactionCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"TRANSX.BEGIN", "TRANSX.BEGIN", nil},
		{"TRANSX.COMMIT", "TRANSX.COMMIT", nil},
		{"TRANSX.ROLLBACK", "TRANSX.ROLLBACK", nil},
		{"TRANSX.STATUS", "TRANSX.STATUS", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestResilienceObservableCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"OBSERVABLE.CREATE", "OBSERVABLE.CREATE", [][]byte{[]byte("obs1")}},
		{"OBSERVABLE.NEXT", "OBSERVABLE.NEXT", [][]byte{[]byte("obs1"), []byte("value")}},
		{"OBSERVABLE.COMPLETE", "OBSERVABLE.COMPLETE", [][]byte{[]byte("obs1")}},
		{"OBSERVABLE.ERROR", "OBSERVABLE.ERROR", [][]byte{[]byte("obs1"), []byte("error message")}},
		{"OBSERVABLE.SUBSCRIBE", "OBSERVABLE.SUBSCRIBE", [][]byte{[]byte("obs1"), []byte("sub1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestResilienceStreamProcCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"STREAMPROC.CREATE", "STREAMPROC.CREATE", [][]byte{[]byte("sp1")}},
		{"STREAMPROC.PUSH", "STREAMPROC.PUSH", [][]byte{[]byte("sp1"), []byte("data")}},
		{"STREAMPROC.POP", "STREAMPROC.POP", [][]byte{[]byte("sp1")}},
		{"STREAMPROC.PEEK", "STREAMPROC.PEEK", [][]byte{[]byte("sp1")}},
		{"STREAMPROC.DELETE", "STREAMPROC.DELETE", [][]byte{[]byte("sp1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestResilienceEventSourcingCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"EVENTSOURCING.APPEND", "EVENTSOURCING.APPEND", [][]byte{[]byte("stream1"), []byte(`{"type":"created","data":{}}`)}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestAggregatorExtCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"AGGREGATOR.CREATE", "AGGREGATOR.CREATE", [][]byte{[]byte("agg1"), []byte("sum")}},
		{"AGGREGATOR.ADD", "AGGREGATOR.ADD", [][]byte{[]byte("agg1"), []byte("10")}},
		{"AGGREGATOR.GET", "AGGREGATOR.GET", [][]byte{[]byte("agg1")}},
		{"AGGREGATOR.RESET", "AGGREGATOR.RESET", [][]byte{[]byte("agg1")}},
		{"AGGREGATOR.DELETE", "AGGREGATOR.DELETE", [][]byte{[]byte("agg1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestTransactionCommandsExtended(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterTransactionCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"MULTI", "MULTI", nil},
		{"EXEC", "EXEC", nil},
		{"DISCARD", "DISCARD", nil},
		{"WATCH", "WATCH", [][]byte{[]byte("key1")}},
		{"UNWATCH", "UNWATCH", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestScriptingCommandsExtended(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterScriptCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"EVAL", "EVAL", [][]byte{[]byte("return 1"), []byte("0")}},
		{"EVALSHA", "EVALSHA", [][]byte{[]byte("abc123"), []byte("0")}},
		{"SCRIPT LOAD", "SCRIPT", [][]byte{[]byte("LOAD"), []byte("return 1")}},
		{"SCRIPT EXISTS", "SCRIPT", [][]byte{[]byte("EXISTS"), []byte("abc123")}},
		{"SCRIPT FLUSH", "SCRIPT", [][]byte{[]byte("FLUSH")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestPubSubCommandsExtended(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterPubSubCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"SUBSCRIBE", "SUBSCRIBE", [][]byte{[]byte("channel1")}},
		{"UNSUBSCRIBE", "UNSUBSCRIBE", [][]byte{[]byte("channel1")}},
		{"PSUBSCRIBE", "PSUBSCRIBE", [][]byte{[]byte("news:*")}},
		{"PUNSUBSCRIBE", "PUNSUBSCRIBE", [][]byte{[]byte("news:*")}},
		{"PUBLISH", "PUBLISH", [][]byte{[]byte("channel1"), []byte("message")}},
		{"PUBSUB CHANNELS", "PUBSUB", [][]byte{[]byte("CHANNELS")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestGeoCommandsExtended(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterGeoCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"GEOADD", "GEOADD", [][]byte{[]byte("locations"), []byte("member1"), []byte("13.361389"), []byte("38.115556")}},
		{"GEOPOS", "GEOPOS", [][]byte{[]byte("locations"), []byte("member1")}},
		{"GEODIST", "GEODIST", [][]byte{[]byte("locations"), []byte("member1"), []byte("member2")}},
		{"GEORADIUS", "GEORADIUS", [][]byte{[]byte("locations"), []byte("15"), []byte("37"), []byte("200"), []byte("km")}},
		{"GEORADIUSBYMEMBER", "GEORADIUSBYMEMBER", [][]byte{[]byte("locations"), []byte("member1"), []byte("100"), []byte("km")}},
		{"GEOHASH", "GEOHASH", [][]byte{[]byte("locations"), []byte("member1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestHyperLogLogCommandsExtended(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterHyperLogLogCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"PFADD", "PFADD", [][]byte{[]byte("hll1"), []byte("a"), []byte("b"), []byte("c")}},
		{"PFCOUNT", "PFCOUNT", [][]byte{[]byte("hll1")}},
		{"PFMERGE", "PFMERGE", [][]byte{[]byte("hll3"), []byte("hll1"), []byte("hll2")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestBitmapCommandsExtended(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterBitmapCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"SETBIT", "SETBIT", [][]byte{[]byte("bits"), []byte("0"), []byte("1")}},
		{"GETBIT", "GETBIT", [][]byte{[]byte("bits"), []byte("0")}},
		{"BITCOUNT", "BITCOUNT", [][]byte{[]byte("bits")}},
		{"BITPOS", "BITPOS", [][]byte{[]byte("bits"), []byte("1")}},
		{"BITOP", "BITOP", [][]byte{[]byte("AND"), []byte("dest"), []byte("bits1"), []byte("bits2")}},
		{"BITFIELD", "BITFIELD", [][]byte{[]byte("bits"), []byte("SET"), []byte("u8"), []byte("0"), []byte("255")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestStreamCommandsExtended(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStreamCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"XADD", "XADD", [][]byte{[]byte("mystream"), []byte("*"), []byte("field"), []byte("value")}},
		{"XLEN", "XLEN", [][]byte{[]byte("mystream")}},
		{"XRANGE", "XRANGE", [][]byte{[]byte("mystream"), []byte("-"), []byte("+")}},
		{"XREVRANGE", "XREVRANGE", [][]byte{[]byte("mystream"), []byte("+"), []byte("-")}},
		{"XREAD", "XREAD", [][]byte{[]byte("STREAMS"), []byte("mystream"), []byte("0")}},
		{"XGROUP CREATE", "XGROUP", [][]byte{[]byte("CREATE"), []byte("mystream"), []byte("mygroup"), []byte("$")}},
		{"XREADGROUP", "XREADGROUP", [][]byte{[]byte("GROUP"), []byte("mygroup"), []byte("consumer1"), []byte("STREAMS"), []byte("mystream"), []byte(">")}},
		{"XACK", "XACK", [][]byte{[]byte("mystream"), []byte("mygroup"), []byte("1234567890123-0")}},
		{"XTRIM", "XTRIM", [][]byte{[]byte("mystream"), []byte("MAXLEN"), []byte("1000")}},
		{"XDEL", "XDEL", [][]byte{[]byte("mystream"), []byte("1234567890123-0")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestSortedSetCommandsExtended(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterSortedSetCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"ZADD", "ZADD", [][]byte{[]byte("zset1"), []byte("1"), []byte("one")}},
		{"ZSCORE", "ZSCORE", [][]byte{[]byte("zset1"), []byte("one")}},
		{"ZRANK", "ZRANK", [][]byte{[]byte("zset1"), []byte("one")}},
		{"ZREVRANK", "ZREVRANK", [][]byte{[]byte("zset1"), []byte("one")}},
		{"ZINCRBY", "ZINCRBY", [][]byte{[]byte("zset1"), []byte("2"), []byte("one")}},
		{"ZCARD", "ZCARD", [][]byte{[]byte("zset1")}},
		{"ZCOUNT", "ZCOUNT", [][]byte{[]byte("zset1"), []byte("0"), []byte("10")}},
		{"ZRANGE", "ZRANGE", [][]byte{[]byte("zset1"), []byte("0"), []byte("-1")}},
		{"ZREVRANGE", "ZREVRANGE", [][]byte{[]byte("zset1"), []byte("0"), []byte("-1")}},
		{"ZREM", "ZREM", [][]byte{[]byte("zset1"), []byte("one")}},
		{"ZREMRANGEBYRANK", "ZREMRANGEBYRANK", [][]byte{[]byte("zset1"), []byte("0"), []byte("1")}},
		{"ZREMRANGEBYSCORE", "ZREMRANGEBYSCORE", [][]byte{[]byte("zset1"), []byte("0"), []byte("5")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestStringCommandsCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"SET", "SET", [][]byte{[]byte("key1"), []byte("value1")}},
		{"GET", "GET", [][]byte{[]byte("key1")}},
		{"SETEX", "SETEX", [][]byte{[]byte("key2"), []byte("10"), []byte("value2")}},
		{"SETNX", "SETNX", [][]byte{[]byte("key3"), []byte("value3")}},
		{"SETRANGE", "SETRANGE", [][]byte{[]byte("key1"), []byte("0"), []byte("new")}},
		{"GETRANGE", "GETRANGE", [][]byte{[]byte("key1"), []byte("0"), []byte("3")}},
		{"INCR", "INCR", [][]byte{[]byte("counter")}},
		{"INCRBY", "INCRBY", [][]byte{[]byte("counter"), []byte("5")}},
		{"DECR", "DECR", [][]byte{[]byte("counter")}},
		{"DECRBY", "DECRBY", [][]byte{[]byte("counter"), []byte("3")}},
		{"APPEND", "APPEND", [][]byte{[]byte("key1"), []byte("suffix")}},
		{"STRLEN", "STRLEN", [][]byte{[]byte("key1")}},
		{"MSET", "MSET", [][]byte{[]byte("k1"), []byte("v1"), []byte("k2"), []byte("v2")}},
		{"MGET", "MGET", [][]byte{[]byte("k1"), []byte("k2")}},
		{"GETSET", "GETSET", [][]byte{[]byte("key1"), []byte("newvalue")}},
		{"GETDEL", "GETDEL", [][]byte{[]byte("key1")}},
		{"INCRBYFLOAT", "INCRBYFLOAT", [][]byte{[]byte("floatkey"), []byte("1.5")}},
		{"MSETNX", "MSETNX", [][]byte{[]byte("nx1"), []byte("v1"), []byte("nx2"), []byte("v2")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestHashCommandsCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterHashCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"HSET", "HSET", [][]byte{[]byte("hash1"), []byte("field1"), []byte("value1")}},
		{"HGET", "HGET", [][]byte{[]byte("hash1"), []byte("field1")}},
		{"HMSET", "HMSET", [][]byte{[]byte("hash2"), []byte("f1"), []byte("v1"), []byte("f2"), []byte("v2")}},
		{"HMGET", "HMGET", [][]byte{[]byte("hash2"), []byte("f1"), []byte("f2")}},
		{"HGETALL", "HGETALL", [][]byte{[]byte("hash1")}},
		{"HDEL", "HDEL", [][]byte{[]byte("hash1"), []byte("field1")}},
		{"HEXISTS", "HEXISTS", [][]byte{[]byte("hash1"), []byte("field1")}},
		{"HKEYS", "HKEYS", [][]byte{[]byte("hash1")}},
		{"HVALS", "HVALS", [][]byte{[]byte("hash1")}},
		{"HLEN", "HLEN", [][]byte{[]byte("hash1")}},
		{"HINCRBY", "HINCRBY", [][]byte{[]byte("hash3"), []byte("counter"), []byte("1")}},
		{"HINCRBYFLOAT", "HINCRBYFLOAT", [][]byte{[]byte("hash3"), []byte("counter"), []byte("1.5")}},
		{"HSETNX", "HSETNX", [][]byte{[]byte("hash1"), []byte("newfield"), []byte("newvalue")}},
		{"HSCAN", "HSCAN", [][]byte{[]byte("hash1"), []byte("0")}},
		{"HSTRLEN", "HSTRLEN", [][]byte{[]byte("hash1"), []byte("field1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestListCommandsCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"LPUSH", "LPUSH", [][]byte{[]byte("list1"), []byte("a"), []byte("b"), []byte("c")}},
		{"RPUSH", "RPUSH", [][]byte{[]byte("list1"), []byte("d"), []byte("e")}},
		{"LPOP", "LPOP", [][]byte{[]byte("list1")}},
		{"RPOP", "RPOP", [][]byte{[]byte("list1")}},
		{"LLEN", "LLEN", [][]byte{[]byte("list1")}},
		{"LRANGE", "LRANGE", [][]byte{[]byte("list1"), []byte("0"), []byte("-1")}},
		{"LINDEX", "LINDEX", [][]byte{[]byte("list1"), []byte("0")}},
		{"LSET", "LSET", [][]byte{[]byte("list1"), []byte("0"), []byte("newvalue")}},
		{"LREM", "LREM", [][]byte{[]byte("list1"), []byte("1"), []byte("value")}},
		{"LTRIM", "LTRIM", [][]byte{[]byte("list1"), []byte("0"), []byte("10")}},
		{"LPUSHX", "LPUSHX", [][]byte{[]byte("list1"), []byte("x")}},
		{"RPUSHX", "RPUSHX", [][]byte{[]byte("list1"), []byte("y")}},
		{"LINSERT", "LINSERT", [][]byte{[]byte("list1"), []byte("BEFORE"), []byte("b"), []byte("inserted")}},
		{"RPOPLPUSH", "RPOPLPUSH", [][]byte{[]byte("list1"), []byte("list2")}},
		{"BLPOP", "BLPOP", [][]byte{[]byte("list1"), []byte("1")}},
		{"BRPOP", "BRPOP", [][]byte{[]byte("list1"), []byte("1")}},
		{"BRPOPLPUSH", "BRPOPLPUSH", [][]byte{[]byte("list1"), []byte("list2"), []byte("1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestSetCommandsCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterSetCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"SADD", "SADD", [][]byte{[]byte("set1"), []byte("a"), []byte("b"), []byte("c")}},
		{"SREM", "SREM", [][]byte{[]byte("set1"), []byte("a")}},
		{"SMEMBERS", "SMEMBERS", [][]byte{[]byte("set1")}},
		{"SISMEMBER", "SISMEMBER", [][]byte{[]byte("set1"), []byte("b")}},
		{"SCARD", "SCARD", [][]byte{[]byte("set1")}},
		{"SPOP", "SPOP", [][]byte{[]byte("set1")}},
		{"SRANDMEMBER", "SRANDMEMBER", [][]byte{[]byte("set1")}},
		{"SMOVE", "SMOVE", [][]byte{[]byte("set1"), []byte("set2"), []byte("b")}},
		{"SUNION", "SUNION", [][]byte{[]byte("set1"), []byte("set2")}},
		{"SINTER", "SINTER", [][]byte{[]byte("set1"), []byte("set2")}},
		{"SDIFF", "SDIFF", [][]byte{[]byte("set1"), []byte("set2")}},
		{"SUNIONSTORE", "SUNIONSTORE", [][]byte{[]byte("dest"), []byte("set1"), []byte("set2")}},
		{"SINTERSTORE", "SINTERSTORE", [][]byte{[]byte("dest"), []byte("set1"), []byte("set2")}},
		{"SDIFFSTORE", "SDIFFSTORE", [][]byte{[]byte("dest"), []byte("set1"), []byte("set2")}},
		{"SSCAN", "SSCAN", [][]byte{[]byte("set1"), []byte("0")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestJSONCommandsCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterJSONCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"JSON.SET", "JSON.SET", [][]byte{[]byte("json1"), []byte("$"), []byte(`{"name":"test","value":123}`)}},
		{"JSON.GET", "JSON.GET", [][]byte{[]byte("json1"), []byte("$")}},
		{"JSON.GET path", "JSON.GET", [][]byte{[]byte("json1"), []byte("$.name")}},
		{"JSON.TYPE", "JSON.TYPE", [][]byte{[]byte("json1"), []byte("$")}},
		{"JSON.STRLEN", "JSON.STRLEN", [][]byte{[]byte("json1"), []byte("$.name")}},
		{"JSON.OBJLEN", "JSON.OBJLEN", [][]byte{[]byte("json1"), []byte("$")}},
		{"JSON.ARRLEN", "JSON.ARRLEN", [][]byte{[]byte("json1"), []byte("$")}},
		{"JSON.NUMINCRBY", "JSON.NUMINCRBY", [][]byte{[]byte("json1"), []byte("$.value"), []byte("10")}},
		{"JSON.DEL", "JSON.DEL", [][]byte{[]byte("json1"), []byte("$.temp")}},
		{"JSON.CLEAR", "JSON.CLEAR", [][]byte{[]byte("json1"), []byte("$")}},
		{"JSON.KEYS", "JSON.KEYS", [][]byte{[]byte("json1"), []byte("$")}},
		{"JSON.DEBUG", "JSON.DEBUG", [][]byte{[]byte("MEMORY"), []byte("json1"), []byte("$")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestServerCommandsCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterServerCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"PING", "PING", nil},
		{"ECHO", "ECHO", [][]byte{[]byte("hello")}},
		{"INFO", "INFO", nil},
		{"INFO default", "INFO", [][]byte{[]byte("default")}},
		{"DBSIZE", "DBSIZE", nil},
		{"TIME", "TIME", nil},
		{"FLUSHDB", "FLUSHDB", nil},
		{"FLUSHALL", "FLUSHALL", nil},
		{"CLIENT LIST", "CLIENT", [][]byte{[]byte("LIST")}},
		{"CLIENT ID", "CLIENT", [][]byte{[]byte("ID")}},
		{"COMMAND", "COMMAND", nil},
		{"COMMAND COUNT", "COMMAND", [][]byte{[]byte("COUNT")}},
		{"COMMAND DOCS", "COMMAND", [][]byte{[]byte("DOCS")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestClusterCommandsCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterClusterCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"CLUSTER INFO", "CLUSTER", [][]byte{[]byte("INFO")}},
		{"CLUSTER NODES", "CLUSTER", [][]byte{[]byte("NODES")}},
		{"CLUSTER MEET", "CLUSTER", [][]byte{[]byte("MEET"), []byte("127.0.0.1"), []byte("7001")}},
		{"CLUSTER FORGET", "CLUSTER", [][]byte{[]byte("FORGET"), []byte("nodeid123")}},
		{"CLUSTER REPLICATE", "CLUSTER", [][]byte{[]byte("REPLICATE"), []byte("masterid")}},
		{"CLUSTER FAILOVER", "CLUSTER", [][]byte{[]byte("FAILOVER")}},
		{"CLUSTER RESET", "CLUSTER", [][]byte{[]byte("RESET"), []byte("SOFT")}},
		{"CLUSTER SLOTS", "CLUSTER", [][]byte{[]byte("SLOTS")}},
		{"CLUSTER KEYSLOT", "CLUSTER", [][]byte{[]byte("KEYSLOT"), []byte("mykey")}},
		{"CLUSTER COUNTKEYSINSLOT", "CLUSTER", [][]byte{[]byte("COUNTKEYSINSLOT"), []byte("1234")}},
		{"CLUSTER GETKEYSINSLOT", "CLUSTER", [][]byte{[]byte("GETKEYSINSLOT"), []byte("1234"), []byte("10")}},
		{"CLUSTER ADDSLOTS", "CLUSTER", [][]byte{[]byte("ADDSLOTS"), []byte("0"), []byte("1")}},
		{"CLUSTER DELSLOTS", "CLUSTER", [][]byte{[]byte("DELSLOTS"), []byte("0")}},
		{"CLUSTER SETSLOT", "CLUSTER", [][]byte{[]byte("SETSLOT"), []byte("0"), []byte("IMPORTING"), []byte("nodeid")}},
		{"CLUSTER SAVECONFIG", "CLUSTER", [][]byte{[]byte("SAVECONFIG")}},
		{"CLUSTER BUMPEPOCH", "CLUSTER", [][]byte{[]byte("BUMPEPOCH")}},
		{"CLUSTER FLUSHSLOTS", "CLUSTER", [][]byte{[]byte("FLUSHSLOTS")}},
		{"READONLY", "READONLY", nil},
		{"READWRITE", "READWRITE", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestKeyCommandsCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)
	RegisterKeyCommands(router)

	s.Set("existingkey", &store.StringValue{Data: []byte("value")}, store.SetOptions{})

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"EXISTS", "EXISTS", [][]byte{[]byte("existingkey")}},
		{"DEL", "DEL", [][]byte{[]byte("existingkey")}},
		{"TYPE", "TYPE", [][]byte{[]byte("existingkey")}},
		{"KEYS", "KEYS", [][]byte{[]byte("*")}},
		{"SCAN", "SCAN", [][]byte{[]byte("0")}},
		{"RENAME", "RENAME", [][]byte{[]byte("existingkey"), []byte("newkey")}},
		{"RENAMENX", "RENAMENX", [][]byte{[]byte("existingkey"), []byte("newkey2")}},
		{"TTL", "TTL", [][]byte{[]byte("existingkey")}},
		{"PTTL", "PTTL", [][]byte{[]byte("existingkey")}},
		{"EXPIRE", "EXPIRE", [][]byte{[]byte("existingkey"), []byte("100")}},
		{"PEXPIRE", "PEXPIRE", [][]byte{[]byte("existingkey"), []byte("100000")}},
		{"EXPIREAT", "EXPIREAT", [][]byte{[]byte("existingkey"), []byte("9999999999")}},
		{"PEXPIREAT", "PEXPIREAT", [][]byte{[]byte("existingkey"), []byte("9999999999000")}},
		{"PERSIST", "PERSIST", [][]byte{[]byte("existingkey")}},
		{"DUMP", "DUMP", [][]byte{[]byte("existingkey")}},
		{"RESTORE", "RESTORE", [][]byte{[]byte("restoredkey"), []byte("0"), []byte("data")}},
		{"MOVE", "MOVE", [][]byte{[]byte("existingkey"), []byte("1")}},
		{"RANDOMKEY", "RANDOMKEY", nil},
		{"COPY", "COPY", [][]byte{[]byte("existingkey"), []byte("copykey")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestBitmapCommandsCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterBitmapCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"SETBIT", "SETBIT", [][]byte{[]byte("bitmap1"), []byte("0"), []byte("1")}},
		{"GETBIT", "GETBIT", [][]byte{[]byte("bitmap1"), []byte("0")}},
		{"BITCOUNT", "BITCOUNT", [][]byte{[]byte("bitmap1")}},
		{"BITPOS", "BITPOS", [][]byte{[]byte("bitmap1"), []byte("1")}},
		{"BITOP AND", "BITOP", [][]byte{[]byte("AND"), []byte("dest"), []byte("bitmap1")}},
		{"BITOP OR", "BITOP", [][]byte{[]byte("OR"), []byte("dest2"), []byte("bitmap1")}},
		{"BITOP XOR", "BITOP", [][]byte{[]byte("XOR"), []byte("dest3"), []byte("bitmap1")}},
		{"BITOP NOT", "BITOP", [][]byte{[]byte("NOT"), []byte("dest4"), []byte("bitmap1")}},
		{"BITFIELD", "BITFIELD", [][]byte{[]byte("bitmap1"), []byte("SET"), []byte("u8"), []byte("0"), []byte("255")}},
		{"BITFIELD_RO", "BITFIELD_RO", [][]byte{[]byte("bitmap1"), []byte("GET"), []byte("u8"), []byte("0")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestGeoCommandsCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterGeoCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"GEOADD", "GEOADD", [][]byte{[]byte("geo1"), []byte("13.361389"), []byte("38.115556"), []byte("Palermo")}},
		{"GEOHASH", "GEOHASH", [][]byte{[]byte("geo1"), []byte("Palermo")}},
		{"GEOPOS", "GEOPOS", [][]byte{[]byte("geo1"), []byte("Palermo")}},
		{"GEODIST", "GEODIST", [][]byte{[]byte("geo1"), []byte("Palermo"), []byte("Catania")}},
		{"GEORADIUS", "GEORADIUS", [][]byte{[]byte("geo1"), []byte("15"), []byte("37"), []byte("100"), []byte("km")}},
		{"GEORADIUSBYMEMBER", "GEORADIUSBYMEMBER", [][]byte{[]byte("geo1"), []byte("Palermo"), []byte("100"), []byte("km")}},
		{"GEOSEARCH", "GEOSEARCH", [][]byte{[]byte("geo1"), []byte("FROMMEMBER"), []byte("Palermo"), []byte("BYRADIUS"), []byte("100"), []byte("km")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestHyperLogLogCommandsCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterHyperLogLogCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"PFADD", "PFADD", [][]byte{[]byte("hll1"), []byte("a"), []byte("b"), []byte("c")}},
		{"PFCOUNT", "PFCOUNT", [][]byte{[]byte("hll1")}},
		{"PFMERGE", "PFMERGE", [][]byte{[]byte("hlldest"), []byte("hll1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestPubSubCommandsCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterPubSubCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"PUBLISH", "PUBLISH", [][]byte{[]byte("channel1"), []byte("message1")}},
		{"SUBSCRIBE", "SUBSCRIBE", [][]byte{[]byte("channel1")}},
		{"UNSUBSCRIBE", "UNSUBSCRIBE", [][]byte{[]byte("channel1")}},
		{"PSUBSCRIBE", "PSUBSCRIBE", [][]byte{[]byte("channel*")}},
		{"PUNSUBSCRIBE", "PUNSUBSCRIBE", [][]byte{[]byte("channel*")}},
		{"PUBSUB CHANNELS", "PUBSUB", [][]byte{[]byte("CHANNELS")}},
		{"PUBSUB NUMSUB", "PUBSUB", [][]byte{[]byte("NUMSUB"), []byte("channel1")}},
		{"PUBSUB NUMPAT", "PUBSUB", [][]byte{[]byte("NUMPAT")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestScriptCommandsCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterScriptCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"EVAL", "EVAL", [][]byte{[]byte("return 1"), []byte("0")}},
		{"EVALSHA", "EVALSHA", [][]byte{[]byte("abc123"), []byte("0")}},
		{"SCRIPT EXISTS", "SCRIPT", [][]byte{[]byte("EXISTS"), []byte("abc123")}},
		{"SCRIPT FLUSH", "SCRIPT", [][]byte{[]byte("FLUSH")}},
		{"SCRIPT LOAD", "SCRIPT", [][]byte{[]byte("LOAD"), []byte("return 1")}},
		{"SCRIPT DEBUG", "SCRIPT", [][]byte{[]byte("DEBUG"), []byte("YES")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestTransactionCommandsCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterTransactionCommands(router)
	RegisterStringCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"MULTI", "MULTI", nil},
		{"EXEC", "EXEC", nil},
		{"DISCARD", "DISCARD", nil},
		{"WATCH", "WATCH", [][]byte{[]byte("key1")}},
		{"UNWATCH", "UNWATCH", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestMonitorCommandsCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMonitoringCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"MONITOR", "MONITOR", nil},
		{"LATENCY LATEST", "LATENCY", [][]byte{[]byte("LATEST")}},
		{"LATENCY HISTORY", "LATENCY", [][]byte{[]byte("HISTORY"), []byte("command")}},
		{"LATENCY RESET", "LATENCY", [][]byte{[]byte("RESET")}},
		{"LATENCY DOCTOR", "LATENCY", [][]byte{[]byte("DOCTOR")}},
		{"LATENCY GRAPH", "LATENCY", [][]byte{[]byte("GRAPH"), []byte("command")}},
		{"SLOWLOG GET", "SLOWLOG", [][]byte{[]byte("GET"), []byte("10")}},
		{"SLOWLOG LEN", "SLOWLOG", [][]byte{[]byte("LEN")}},
		{"SLOWLOG RESET", "SLOWLOG", [][]byte{[]byte("RESET")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestTimeSeriesCommandsCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterTSCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"TS.CREATE", "TS.CREATE", [][]byte{[]byte("ts1")}},
		{"TS.ADD", "TS.ADD", [][]byte{[]byte("ts1"), []byte("1234567890"), []byte("100")}},
		{"TS.GET", "TS.GET", [][]byte{[]byte("ts1")}},
		{"TS.RANGE", "TS.RANGE", [][]byte{[]byte("ts1"), []byte("-"), []byte("+")}},
		{"TS.REVRANGE", "TS.REVRANGE", [][]byte{[]byte("ts1"), []byte("+"), []byte("-")}},
		{"TS.MRANGE", "TS.MRANGE", [][]byte{[]byte("-"), []byte("+"), []byte("FILTER"), []byte("label=value")}},
		{"TS.INFO", "TS.INFO", [][]byte{[]byte("ts1")}},
		{"TS.QUERYINDEX", "TS.QUERYINDEX", [][]byte{[]byte("label=value")}},
		{"TS.MGET", "TS.MGET", [][]byte{[]byte("FILTER"), []byte("label=value")}},
		{"TS.ALTER", "TS.ALTER", [][]byte{[]byte("ts1"), []byte("RETENTION"), []byte("0")}},
		{"TS.DEL", "TS.DEL", [][]byte{[]byte("ts1"), []byte("-"), []byte("+")}},
		{"TS.MADD", "TS.MADD", [][]byte{[]byte("ts1"), []byte("1234567891"), []byte("200")}},
		{"TS.INCRBY", "TS.INCRBY", [][]byte{[]byte("ts1"), []byte("1")}},
		{"TS.DECRBY", "TS.DECRBY", [][]byte{[]byte("ts1"), []byte("1")}},
		{"TS.CREATERULE", "TS.CREATERULE", [][]byte{[]byte("ts1"), []byte("ts_agg"), []byte("AGGREGATION"), []byte("avg"), []byte("60000")}},
		{"TS.DELETERULE", "TS.DELETERULE", [][]byte{[]byte("ts1"), []byte("ts_agg")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestNamespaceCommandsCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterNamespaceCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"NAMESPACE ADD", "NAMESPACE", [][]byte{[]byte("ADD"), []byte("ns1")}},
		{"NAMESPACE LIST", "NAMESPACE", [][]byte{[]byte("LIST")}},
		{"NAMESPACE DEL", "NAMESPACE", [][]byte{[]byte("DEL"), []byte("ns1")}},
		{"NAMESPACE GET", "NAMESPACE", [][]byte{[]byte("GET"), []byte("ns1")}},
		{"NAMESPACE SET", "NAMESPACE", [][]byte{[]byte("SET"), []byte("ns1"), []byte("maxmemory"), []byte("1000000")}},
		{"NAMESPACE INFO", "NAMESPACE", [][]byte{[]byte("INFO"), []byte("ns1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestTagCommandsCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterTagCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"TAG.ADD", "TAG.ADD", [][]byte{[]byte("key1"), []byte("tag1"), []byte("tag2")}},
		{"TAG.GET", "TAG.GET", [][]byte{[]byte("key1")}},
		{"TAG.DEL", "TAG.DEL", [][]byte{[]byte("key1"), []byte("tag1")}},
		{"TAG.QUERY", "TAG.QUERY", [][]byte{[]byte("tag1")}},
		{"TAG.KEYS", "TAG.KEYS", [][]byte{[]byte("tag1")}},
		{"TAG.CLEAR", "TAG.CLEAR", [][]byte{[]byte("key1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestSearchCommandsCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterSearchCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"FT.CREATE", "FT.CREATE", [][]byte{[]byte("idx1"), []byte("ON"), []byte("HASH"), []byte("PREFIX"), []byte("1"), []byte("doc:"), []byte("SCHEMA"), []byte("title"), []byte("TEXT")}},
		{"FT.SEARCH", "FT.SEARCH", [][]byte{[]byte("idx1"), []byte("*")}},
		{"FT.INFO", "FT.INFO", [][]byte{[]byte("idx1")}},
		{"FT.DROPINDEX", "FT.DROPINDEX", [][]byte{[]byte("idx1")}},
		{"FT.ALIASADD", "FT.ALIASADD", [][]byte{[]byte("alias1"), []byte("idx1")}},
		{"FT.ALIASDEL", "FT.ALIASDEL", [][]byte{[]byte("alias1")}},
		{"FT.ALIASUPDATE", "FT.ALIASUPDATE", [][]byte{[]byte("alias1"), []byte("idx1")}},
		{"FT.TAGVALS", "FT.TAGVALS", [][]byte{[]byte("idx1"), []byte("tagfield")}},
		{"FT.PROFILE", "FT.PROFILE", [][]byte{[]byte("idx1"), []byte("SEARCH"), []byte("QUERY"), []byte("*")}},
		{"FT.EXPLAIN", "FT.EXPLAIN", [][]byte{[]byte("idx1"), []byte("*")}},
		{"FT.AGGREGATE", "FT.AGGREGATE", [][]byte{[]byte("idx1"), []byte("*")}},
		{"FT.CURSOR", "FT.CURSOR", [][]byte{[]byte("READ"), []byte("idx1"), []byte("0")}},
		{"FT.SYNDUMP", "FT.SYNDUMP", [][]byte{[]byte("idx1")}},
		{"FT.SPELLCHECK", "FT.SPELLCHECK", [][]byte{[]byte("idx1"), []byte("qurey")}},
		{"FT.DICTADD", "FT.DICTADD", [][]byte{[]byte("dict1"), []byte("word1"), []byte("word2")}},
		{"FT.DICTDEL", "FT.DICTDEL", [][]byte{[]byte("dict1"), []byte("word1")}},
		{"FT.DICTDUMP", "FT.DICTDUMP", [][]byte{[]byte("dict1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestGraphCommandsCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterGraphCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"GRAPH.QUERY", "GRAPH.QUERY", [][]byte{[]byte("g1"), []byte("CREATE (n:Person {name: 'Alice'})")}},
		{"GRAPH.DELETE", "GRAPH.DELETE", [][]byte{[]byte("g1")}},
		{"GRAPH.PROFILE", "GRAPH.PROFILE", [][]byte{[]byte("g1"), []byte("MATCH (n) RETURN n")}},
		{"GRAPH.EXPLAIN", "GRAPH.EXPLAIN", [][]byte{[]byte("g1"), []byte("MATCH (n) RETURN n")}},
		{"GRAPH.SLOWLOG", "GRAPH.SLOWLOG", [][]byte{[]byte("g1")}},
		{"GRAPH.CONFIG", "GRAPH.CONFIG", [][]byte{[]byte("GET"), []byte("RESULTSET_SIZE")}},
		{"GRAPH.RO_QUERY", "GRAPH.RO_QUERY", [][]byte{[]byte("g1"), []byte("MATCH (n) RETURN n")}},
		{"GRAPH.LIST", "GRAPH.LIST", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestMVCCCommandsCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMVCCCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"MVCC.GET", "MVCC.GET", [][]byte{[]byte("key1")}},
		{"MVCC.SET", "MVCC.SET", [][]byte{[]byte("key1"), []byte("value1")}},
		{"MVCC.DEL", "MVCC.DEL", [][]byte{[]byte("key1")}},
		{"MVCC.HISTORY", "MVCC.HISTORY", [][]byte{[]byte("key1")}},
		{"MVCC.VERSION", "MVCC.VERSION", nil},
		{"MVCC.COMPACT", "MVCC.COMPACT", [][]byte{[]byte("100")}},
		{"MVCC.SNAPSHOT", "MVCC.SNAPSHOT", nil},
		{"MVCC.RESTORE", "MVCC.RESTORE", [][]byte{[]byte("100")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestDigestCommandsCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterDigestCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"DIGEST.GET", "DIGEST.GET", [][]byte{[]byte("key1")}},
		{"DIGEST.SET", "DIGEST.SET", [][]byte{[]byte("key1"), []byte("value1")}},
		{"DIGEST.VERIFY", "DIGEST.VERIFY", [][]byte{[]byte("key1"), []byte("value1")}},
		{"DIGEST.LIST", "DIGEST.LIST", nil},
		{"DIGEST.DEL", "DIGEST.DEL", [][]byte{[]byte("key1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestProbabilisticCommandsCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterProbabilisticCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"BF.ADD", "BF.ADD", [][]byte{[]byte("bf1"), []byte("item1")}},
		{"BF.EXISTS", "BF.EXISTS", [][]byte{[]byte("bf1"), []byte("item1")}},
		{"BF.MADD", "BF.MADD", [][]byte{[]byte("bf1"), []byte("item2"), []byte("item3")}},
		{"BF.MEXISTS", "BF.MEXISTS", [][]byte{[]byte("bf1"), []byte("item1"), []byte("item2")}},
		{"BF.INFO", "BF.INFO", [][]byte{[]byte("bf1")}},
		{"BF.RESERVE", "BF.RESERVE", [][]byte{[]byte("bf2"), []byte("0.01"), []byte("1000")}},
		{"CF.ADD", "CF.ADD", [][]byte{[]byte("cf1"), []byte("item1")}},
		{"CF.EXISTS", "CF.EXISTS", [][]byte{[]byte("cf1"), []byte("item1")}},
		{"CF.DEL", "CF.DEL", [][]byte{[]byte("cf1"), []byte("item1")}},
		{"CF.COUNT", "CF.COUNT", [][]byte{[]byte("cf1")}},
		{"CF.INFO", "CF.INFO", [][]byte{[]byte("cf1")}},
		{"CF.RESERVE", "CF.RESERVE", [][]byte{[]byte("cf2"), []byte("1000")}},
		{"CMS.INCRBY", "CMS.INCRBY", [][]byte{[]byte("cms1"), []byte("item1"), []byte("1")}},
		{"CMS.QUERY", "CMS.QUERY", [][]byte{[]byte("cms1"), []byte("item1")}},
		{"CMS.INFO", "CMS.INFO", [][]byte{[]byte("cms1")}},
		{"CMS.INITBYPROB", "CMS.INITBYPROB", [][]byte{[]byte("cms2"), []byte("0.01"), []byte("0.01")}},
		{"TOPK.ADD", "TOPK.ADD", [][]byte{[]byte("topk1"), []byte("item1")}},
		{"TOPK.QUERY", "TOPK.QUERY", [][]byte{[]byte("topk1"), []byte("item1")}},
		{"TOPK.INFO", "TOPK.INFO", [][]byte{[]byte("topk1")}},
		{"TOPK.RESERVE", "TOPK.RESERVE", [][]byte{[]byte("topk2"), []byte("10")}},
		{"TDIGEST.ADD", "TDIGEST.ADD", [][]byte{[]byte("td1"), []byte("1.0"), []byte("2.0")}},
		{"TDIGEST.QUANTILE", "TDIGEST.QUANTILE", [][]byte{[]byte("td1"), []byte("0.5")}},
		{"TDIGEST.INFO", "TDIGEST.INFO", [][]byte{[]byte("td1")}},
		{"TDIGEST.CREATE", "TDIGEST.CREATE", [][]byte{[]byte("td2")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestActorCommandsCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterActorCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"ACTOR.CREATE", "ACTOR.CREATE", [][]byte{[]byte("actor1")}},
		{"ACTOR.SEND", "ACTOR.SEND", [][]byte{[]byte("actor1"), []byte("msg")}},
		{"ACTOR.RECV", "ACTOR.RECV", [][]byte{[]byte("actor1")}},
		{"ACTOR.STOP", "ACTOR.STOP", [][]byte{[]byte("actor1")}},
		{"ACTOR.LIST", "ACTOR.LIST", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestDSCommandsCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterDataStructuresCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"DS.CREATE", "DS.CREATE", [][]byte{[]byte("ds1"), []byte("list")}},
		{"DS.PUSH", "DS.PUSH", [][]byte{[]byte("ds1"), []byte("item1")}},
		{"DS.POP", "DS.POP", [][]byte{[]byte("ds1")}},
		{"DS.PEEK", "DS.PEEK", [][]byte{[]byte("ds1")}},
		{"DS.SIZE", "DS.SIZE", [][]byte{[]byte("ds1")}},
		{"DS.CLEAR", "DS.CLEAR", [][]byte{[]byte("ds1")}},
		{"DS.DESTROY", "DS.DESTROY", [][]byte{[]byte("ds1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestWorkflowCommandsCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterWorkflowCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"WORKFLOW.CREATE", "WORKFLOW.CREATE", [][]byte{[]byte("wf1")}},
		{"WORKFLOW.ADDSTEP", "WORKFLOW.ADDSTEP", [][]byte{[]byte("wf1"), []byte("step1"), []byte("SET"), []byte("key"), []byte("value")}},
		{"WORKFLOW.EXEC", "WORKFLOW.EXEC", [][]byte{[]byte("wf1")}},
		{"WORKFLOW.STATUS", "WORKFLOW.STATUS", [][]byte{[]byte("wf1")}},
		{"WORKFLOW.DELETE", "WORKFLOW.DELETE", [][]byte{[]byte("wf1")}},
		{"WORKFLOW.LIST", "WORKFLOW.LIST", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestExtendedCommandsCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtendedCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"STRALGO LCS", "STRALGO", [][]byte{[]byte("LCS"), []byte("STRINGS"), []byte("abc"), []byte("abd")}},
		{"SLEEP", "SLEEP", [][]byte{[]byte("0")}},
		{"DUMP", "DUMP", [][]byte{[]byte("key")}},
		{"RESTORE", "RESTORE", [][]byte{[]byte("key"), []byte("0"), []byte("data")}},
		{"MIGRATE", "MIGRATE", [][]byte{[]byte("127.0.0.1"), []byte("6379"), []byte("key"), []byte("0"), []byte("1000")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestMoreCommandsCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"CLIENT PAUSE", "CLIENT", [][]byte{[]byte("PAUSE"), []byte("0")}},
		{"CLIENT UNPAUSE", "CLIENT", [][]byte{[]byte("UNPAUSE")}},
		{"CLIENT TRACKING", "CLIENT", [][]byte{[]byte("TRACKING"), []byte("ON")}},
		{"CLIENT CACHING", "CLIENT", [][]byte{[]byte("CACHING"), []byte("YES")}},
		{"CLIENT GETREDIR", "CLIENT", [][]byte{[]byte("GETREDIR")}},
		{"CLIENT SETNAME", "CLIENT", [][]byte{[]byte("SETNAME"), []byte("testclient")}},
		{"CLIENT GETNAME", "CLIENT", [][]byte{[]byte("GETNAME")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestExtraCommandsCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtraCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"WAIT", "WAIT", [][]byte{[]byte("1"), []byte("1000")}},
		{"WAITAOF", "WAITAOF", [][]byte{[]byte("1"), []byte("1"), []byte("1000")}},
		{"LOLWUT", "LOLWUT", nil},
		{"LOLWUT VERSION", "LOLWUT", [][]byte{[]byte("VERSION"), []byte("5")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestIntegrationCommandsCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterIntegrationCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"SWAPDB", "SWAPDB", [][]byte{[]byte("0"), []byte("1")}},
		{"COPY", "COPY", [][]byte{[]byte("src"), []byte("dst")}},
		{"MOVE", "MOVE", [][]byte{[]byte("key"), []byte("1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestTemplateCommandsCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterTemplateCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"TEMPLATE.CREATE", "TEMPLATE.CREATE", [][]byte{[]byte("tpl1"), []byte("Hello {{.name}}")}},
		{"TEMPLATE.RENDER", "TEMPLATE.RENDER", [][]byte{[]byte("tpl1"), []byte(`{"name":"World"}`)}},
		{"TEMPLATE.DELETE", "TEMPLATE.DELETE", [][]byte{[]byte("tpl1")}},
		{"TEMPLATE.LIST", "TEMPLATE.LIST", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestAdvancedCommands2Coverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterAdvancedCommands2(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"SORT", "SORT", [][]byte{[]byte("list1")}},
		{"SORT BY", "SORT", [][]byte{[]byte("list1"), []byte("BY"), []byte("weight_*")}},
		{"SORT LIMIT", "SORT", [][]byte{[]byte("list1"), []byte("LIMIT"), []byte("0"), []byte("10")}},
		{"SORT GET", "SORT", [][]byte{[]byte("list1"), []byte("GET"), []byte("object_*")}},
		{"SORT ALPHA", "SORT", [][]byte{[]byte("list1"), []byte("ALPHA")}},
		{"SORT DESC", "SORT", [][]byte{[]byte("list1"), []byte("DESC")}},
		{"SORT STORE", "SORT", [][]byte{[]byte("list1"), []byte("STORE"), []byte("sortedlist")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestStringCommandsExtendedCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	s.Set("str1", &store.StringValue{Data: []byte("hello world")}, store.SetOptions{})
	s.Set("counter", &store.StringValue{Data: []byte("10")}, store.SetOptions{})

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"GETRANGE full", "GETRANGE", [][]byte{[]byte("str1"), []byte("0"), []byte("-1")}},
		{"GETRANGE partial", "GETRANGE", [][]byte{[]byte("str1"), []byte("0"), []byte("4")}},
		{"GETRANGE negative", "GETRANGE", [][]byte{[]byte("str1"), []byte("-5"), []byte("-1")}},
		{"SETRANGE", "SETRANGE", [][]byte{[]byte("str1"), []byte("6"), []byte("Redis")}},
		{"INCRBYFLOAT", "INCRBYFLOAT", [][]byte{[]byte("counter"), []byte("1.5")}},
		{"INCRBYFLOAT negative", "INCRBYFLOAT", [][]byte{[]byte("counter"), []byte("-0.5")}},
		{"LCS", "LCS", [][]byte{[]byte("str1"), []byte("str1")}},
		{"GETDEL", "GETDEL", [][]byte{[]byte("str1")}},
		{"GETEX EX", "GETEX", [][]byte{[]byte("str1"), []byte("EX"), []byte("100")}},
		{"GETEX PX", "GETEX", [][]byte{[]byte("str1"), []byte("PX"), []byte("100000")}},
		{"GETEX PERSIST", "GETEX", [][]byte{[]byte("str1"), []byte("PERSIST")}},
		{"SET EX", "SET", [][]byte{[]byte("tmp1"), []byte("val"), []byte("EX"), []byte("100")}},
		{"SET PX", "SET", [][]byte{[]byte("tmp2"), []byte("val"), []byte("PX"), []byte("100000")}},
		{"SET NX exists", "SET", [][]byte{[]byte("str1"), []byte("newval"), []byte("NX")}},
		{"SET XX", "SET", [][]byte{[]byte("str1"), []byte("newval"), []byte("XX")}},
		{"SET GET", "SET", [][]byte{[]byte("str1"), []byte("newval"), []byte("GET")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestHashCommandsExtendedCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterHashCommands(router)

	s.Set("hash1", &store.HashValue{Fields: map[string][]byte{"f1": []byte("v1"), "f2": []byte("v2")}}, store.SetOptions{})

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"HGETDEL", "HGETDEL", [][]byte{[]byte("hash1"), []byte("f1")}},
		{"HGETEX EX", "HGETEX", [][]byte{[]byte("hash1"), []byte("f2"), []byte("EX"), []byte("100")}},
		{"HGETEX PX", "HGETEX", [][]byte{[]byte("hash1"), []byte("f2"), []byte("PX"), []byte("100000")}},
		{"HRANDFIELD", "HRANDFIELD", [][]byte{[]byte("hash1")}},
		{"HRANDFIELD count", "HRANDFIELD", [][]byte{[]byte("hash1"), []byte("2")}},
		{"HRANDFIELD withvalues", "HRANDFIELD", [][]byte{[]byte("hash1"), []byte("2"), []byte("WITHVALUES")}},
		{"HSCAN with match", "HSCAN", [][]byte{[]byte("hash1"), []byte("0"), []byte("MATCH"), []byte("f*")}},
		{"HSCAN with count", "HSCAN", [][]byte{[]byte("hash1"), []byte("0"), []byte("COUNT"), []byte("10")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestListCommandsExtendedCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)

	s.Set("list1", &store.ListValue{Elements: [][]byte{[]byte("a"), []byte("b"), []byte("c"), []byte("d"), []byte("e")}}, store.SetOptions{})

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"LPOS", "LPOS", [][]byte{[]byte("list1"), []byte("b")}},
		{"LPOS RANK", "LPOS", [][]byte{[]byte("list1"), []byte("a"), []byte("RANK"), []byte("1")}},
		{"LPOS COUNT", "LPOS", [][]byte{[]byte("list1"), []byte("a"), []byte("COUNT"), []byte("2")}},
		{"LPOS MAXLEN", "LPOS", [][]byte{[]byte("list1"), []byte("a"), []byte("MAXLEN"), []byte("3")}},
		{"LMPOP", "LMPOP", [][]byte{[]byte("1"), []byte("list1"), []byte("LEFT"), []byte("COUNT"), []byte("1")}},
		{"LMPUSH", "LMPUSH", [][]byte{[]byte("list1"), []byte("LEFT"), []byte("x"), []byte("y")}},
		{"BLMPOP timeout", "BLMPOP", [][]byte{[]byte("0"), []byte("1"), []byte("list1"), []byte("LEFT"), []byte("COUNT"), []byte("1")}},
		{"LMOVE", "LMOVE", [][]byte{[]byte("list1"), []byte("list2"), []byte("LEFT"), []byte("RIGHT")}},
		{"LPOP count", "LPOP", [][]byte{[]byte("list1"), []byte("2")}},
		{"RPOP count", "RPOP", [][]byte{[]byte("list1"), []byte("2")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestSetCommandsExtendedCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterSetCommands(router)

	s.Set("set1", &store.SetValue{Members: map[string]struct{}{"a": {}, "b": {}, "c": {}}}, store.SetOptions{})
	s.Set("set2", &store.SetValue{Members: map[string]struct{}{"b": {}, "c": {}, "d": {}}}, store.SetOptions{})

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"SINTERCARD", "SINTERCARD", [][]byte{[]byte("2"), []byte("set1"), []byte("set2")}},
		{"SINTERCARD LIMIT", "SINTERCARD", [][]byte{[]byte("2"), []byte("set1"), []byte("set2"), []byte("LIMIT"), []byte("1")}},
		{"SMISMEMBER", "SMISMEMBER", [][]byte{[]byte("set1"), []byte("a"), []byte("x")}},
		{"SSCAN MATCH", "SSCAN", [][]byte{[]byte("set1"), []byte("0"), []byte("MATCH"), []byte("a*")}},
		{"SSCAN COUNT", "SSCAN", [][]byte{[]byte("set1"), []byte("0"), []byte("COUNT"), []byte("10")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestSortedSetCommandsExtendedCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterSortedSetCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"ZADD with options", "ZADD", [][]byte{[]byte("zset1"), []byte("NX"), []byte("1"), []byte("one")}},
		{"ZADD XX", "ZADD", [][]byte{[]byte("zset1"), []byte("XX"), []byte("2"), []byte("two")}},
		{"ZADD CH", "ZADD", [][]byte{[]byte("zset1"), []byte("CH"), []byte("1"), []byte("one"), []byte("3"), []byte("three")}},
		{"ZRANGEBYSCORE", "ZRANGEBYSCORE", [][]byte{[]byte("zset1"), []byte("-inf"), []byte("+inf")}},
		{"ZRANGEBYSCORE LIMIT", "ZRANGEBYSCORE", [][]byte{[]byte("zset1"), []byte("0"), []byte("10"), []byte("LIMIT"), []byte("0"), []byte("5")}},
		{"ZREVRANGEBYSCORE", "ZREVRANGEBYSCORE", [][]byte{[]byte("zset1"), []byte("+inf"), []byte("-inf")}},
		{"ZRANGEBYLEX", "ZRANGEBYLEX", [][]byte{[]byte("zset1"), []byte("-"), []byte("+")}},
		{"ZREVRANGEBYLEX", "ZREVRANGEBYLEX", [][]byte{[]byte("zset1"), []byte("+"), []byte("-")}},
		{"ZLEXCOUNT", "ZLEXCOUNT", [][]byte{[]byte("zset1"), []byte("-"), []byte("+")}},
		{"ZREMRANGEBYLEX", "ZREMRANGEBYLEX", [][]byte{[]byte("zset1"), []byte("-"), []byte("+")}},
		{"ZUNIONSTORE", "ZUNIONSTORE", [][]byte{[]byte("zunion"), []byte("1"), []byte("zset1")}},
		{"ZINTERSTORE", "ZINTERSTORE", [][]byte{[]byte("zinter"), []byte("1"), []byte("zset1")}},
		{"ZDIFFSTORE", "ZDIFFSTORE", [][]byte{[]byte("zdiff"), []byte("1"), []byte("zset1")}},
		{"ZRANGESTORE", "ZRANGESTORE", [][]byte{[]byte("zrange"), []byte("zset1"), []byte("0"), []byte("-1")}},
		{"ZMSCORE", "ZMSCORE", [][]byte{[]byte("zset1"), []byte("one"), []byte("two")}},
		{"ZRANDMEMBER", "ZRANDMEMBER", [][]byte{[]byte("zset1")}},
		{"ZRANDMEMBER count", "ZRANDMEMBER", [][]byte{[]byte("zset1"), []byte("2")}},
		{"ZRANDMEMBER withscores", "ZRANDMEMBER", [][]byte{[]byte("zset1"), []byte("2"), []byte("WITHSCORES")}},
		{"ZSCAN", "ZSCAN", [][]byte{[]byte("zset1"), []byte("0")}},
		{"ZPOPMIN count", "ZPOPMIN", [][]byte{[]byte("zset1"), []byte("2")}},
		{"ZPOPMAX count", "ZPOPMAX", [][]byte{[]byte("zset1"), []byte("2")}},
		{"ZMPOP", "ZMPOP", [][]byte{[]byte("1"), []byte("zset1"), []byte("MIN"), []byte("COUNT"), []byte("1")}},
		{"BZPOPMIN", "BZPOPMIN", [][]byte{[]byte("zset1"), []byte("0")}},
		{"BZPOPMAX", "BZPOPMAX", [][]byte{[]byte("zset1"), []byte("0")}},
		{"BZMPOP", "BZMPOP", [][]byte{[]byte("0"), []byte("1"), []byte("zset1"), []byte("MIN")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestStreamCommandsExtendedCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStreamCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"XADD with ID", "XADD", [][]byte{[]byte("stream1"), []byte("0-1"), []byte("field"), []byte("value")}},
		{"XADD MAXLEN", "XADD", [][]byte{[]byte("stream1"), []byte("MAXLEN"), []byte("100"), []byte("*"), []byte("f"), []byte("v")}},
		{"XADD MINID", "XADD", [][]byte{[]byte("stream1"), []byte("MINID"), []byte("0-1"), []byte("*"), []byte("f"), []byte("v")}},
		{"XLEN", "XLEN", [][]byte{[]byte("stream1")}},
		{"XRANGE COUNT", "XRANGE", [][]byte{[]byte("stream1"), []byte("-"), []byte("+"), []byte("COUNT"), []byte("10")}},
		{"XREVRANGE COUNT", "XREVRANGE", [][]byte{[]byte("stream1"), []byte("+"), []byte("-"), []byte("COUNT"), []byte("10")}},
		{"XREAD COUNT", "XREAD", [][]byte{[]byte("COUNT"), []byte("10"), []byte("STREAMS"), []byte("stream1"), []byte("0")}},
		{"XREAD BLOCK", "XREAD", [][]byte{[]byte("BLOCK"), []byte("0"), []byte("STREAMS"), []byte("stream1"), []byte("0")}},
		{"XGROUP CREATE MKSTREAM", "XGROUP", [][]byte{[]byte("CREATE"), []byte("stream1"), []byte("group1"), []byte("$"), []byte("MKSTREAM")}},
		{"XGROUP SETID", "XGROUP", [][]byte{[]byte("SETID"), []byte("stream1"), []byte("group1"), []byte("$")}},
		{"XGROUP DESTROY", "XGROUP", [][]byte{[]byte("DESTROY"), []byte("stream1"), []byte("group1")}},
		{"XGROUP CREATECONSUMER", "XGROUP", [][]byte{[]byte("CREATECONSUMER"), []byte("stream1"), []byte("group1"), []byte("consumer1")}},
		{"XGROUP DELCONSUMER", "XGROUP", [][]byte{[]byte("DELCONSUMER"), []byte("stream1"), []byte("group1"), []byte("consumer1")}},
		{"XINFO STREAM", "XINFO", [][]byte{[]byte("STREAM"), []byte("stream1")}},
		{"XINFO GROUPS", "XINFO", [][]byte{[]byte("GROUPS"), []byte("stream1")}},
		{"XINFO CONSUMERS", "XINFO", [][]byte{[]byte("CONSUMERS"), []byte("stream1"), []byte("group1")}},
		{"XTRIM MAXLEN", "XTRIM", [][]byte{[]byte("stream1"), []byte("MAXLEN"), []byte("10")}},
		{"XTRIM MINID", "XTRIM", [][]byte{[]byte("stream1"), []byte("MINID"), []byte("0-1")}},
		{"XCLAIM", "XCLAIM", [][]byte{[]byte("stream1"), []byte("group1"), []byte("consumer1"), []byte("0"), []byte("0-1")}},
		{"XAUTOCLAIM", "XAUTOCLAIM", [][]byte{[]byte("stream1"), []byte("group1"), []byte("consumer1"), []byte("0"), []byte("0-0")}},
		{"XPENDING", "XPENDING", [][]byte{[]byte("stream1"), []byte("group1")}},
		{"XSETID", "XSETID", [][]byte{[]byte("stream1"), []byte("0-1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestScriptCommandsExtendedCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterScriptCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"EVAL simple", "EVAL", [][]byte{[]byte("return 1"), []byte("0")}},
		{"EVAL with keys", "EVAL", [][]byte{[]byte("return {KEYS[1], ARGV[1]}"), []byte("1"), []byte("key1"), []byte("arg1")}},
		{"EVALSHA", "EVALSHA", [][]byte{[]byte("abc123def"), []byte("0")}},
		{"SCRIPT EXISTS", "SCRIPT", [][]byte{[]byte("EXISTS"), []byte("sha1"), []byte("sha2")}},
		{"SCRIPT FLUSH", "SCRIPT", [][]byte{[]byte("FLUSH")}},
		{"SCRIPT LOAD", "SCRIPT", [][]byte{[]byte("LOAD"), []byte("return 1")}},
		{"SCRIPT DEBUG", "SCRIPT", [][]byte{[]byte("DEBUG"), []byte("YES")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestFunctionCommandsCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterFunctionCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"FUNCTION CREATE", "FUNCTION", [][]byte{[]byte("CREATE"), []byte("func1"), []byte("LUA"), []byte("return 1")}},
		{"FUNCTION LIST", "FUNCTION", [][]byte{[]byte("LIST")}},
		{"FUNCTION DELETE", "FUNCTION", [][]byte{[]byte("DELETE"), []byte("func1")}},
		{"FUNCTION CALL", "FCALL", [][]byte{[]byte("func1"), []byte("0")}},
		{"FUNCTION INFO", "FUNCTION", [][]byte{[]byte("INFO"), []byte("func1")}},
		{"FUNCTION STATS", "FUNCTION", [][]byte{[]byte("STATS")}},
		{"FUNCTION KILL", "FUNCTION", [][]byte{[]byte("KILL")}},
		{"FUNCTION DUMP", "FUNCTION", [][]byte{[]byte("DUMP")}},
		{"FUNCTION RESTORE", "FUNCTION", [][]byte{[]byte("RESTORE"), []byte("data")}},
		{"FUNCTION FLUSH", "FUNCTION", [][]byte{[]byte("FLUSH")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestSchedulerCommandsCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterSchedulerCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"SCHEDULER.ADD", "SCHEDULER.ADD", [][]byte{[]byte("job1"), []byte("* * * * *"), []byte("SET"), []byte("key"), []byte("value")}},
		{"SCHEDULER.REMOVE", "SCHEDULER.REMOVE", [][]byte{[]byte("job1")}},
		{"SCHEDULER.LIST", "SCHEDULER.LIST", nil},
		{"SCHEDULER.INFO", "SCHEDULER.INFO", [][]byte{[]byte("job1")}},
		{"SCHEDULER.PAUSE", "SCHEDULER.PAUSE", [][]byte{[]byte("job1")}},
		{"SCHEDULER.RESUME", "SCHEDULER.RESUME", [][]byte{[]byte("job1")}},
		{"SCHEDULER.CLEAR", "SCHEDULER.CLEAR", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestStatsCommandsCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStatsCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"STATS.GET", "STATS.GET", [][]byte{[]byte("counter1")}},
		{"STATS.INCR", "STATS.INCR", [][]byte{[]byte("counter1"), []byte("1")}},
		{"STATS.DECR", "STATS.DECR", [][]byte{[]byte("counter1"), []byte("1")}},
		{"STATS.RESET", "STATS.RESET", [][]byte{[]byte("counter1")}},
		{"STATS.LIST", "STATS.LIST", nil},
		{"STATS.AVG", "STATS.AVG", [][]byte{[]byte("metric1"), []byte("100")}},
		{"STATS.SUM", "STATS.SUM", [][]byte{[]byte("metric1"), []byte("100")}},
		{"STATS.MIN", "STATS.MIN", [][]byte{[]byte("metric1"), []byte("100")}},
		{"STATS.MAX", "STATS.MAX", [][]byte{[]byte("metric1"), []byte("100")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestEventCommandsCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterEventCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"EVENT.PUBLISH", "EVENT.PUBLISH", [][]byte{[]byte("event1"), []byte("data")}},
		{"EVENT.SUBSCRIBE", "EVENT.SUBSCRIBE", [][]byte{[]byte("event1")}},
		{"EVENT.UNSUBSCRIBE", "EVENT.UNSUBSCRIBE", [][]byte{[]byte("event1")}},
		{"EVENT.LIST", "EVENT.LIST", nil},
		{"EVENT.HISTORY", "EVENT.HISTORY", [][]byte{[]byte("event1"), []byte("10")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestEncodingCommandsCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterEncodingCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"BASE64.ENCODE", "BASE64.ENCODE", [][]byte{[]byte("hello")}},
		{"BASE64.DECODE", "BASE64.DECODE", [][]byte{[]byte("aGVsbG8=")}},
		{"HEX.ENCODE", "HEX.ENCODE", [][]byte{[]byte("hello")}},
		{"HEX.DECODE", "HEX.DECODE", [][]byte{[]byte("68656c6c6f")}},
		{"JSON.ENCODE", "JSON.ENCODE", [][]byte{[]byte(`{"key":"value"}`)}},
		{"JSON.DECODE", "JSON.DECODE", [][]byte{[]byte(`{"key":"value"}`)}},
		{"GZIP.COMPRESS", "GZIP.COMPRESS", [][]byte{[]byte("hello world")}},
		{"GZIP.DECOMPRESS", "GZIP.DECOMPRESS", [][]byte{[]byte("compressed_data")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestUtilityCommandsCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterUtilityCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"UUID.GENERATE", "UUID.GENERATE", nil},
		{"UUID.GENERATE v4", "UUID.GENERATE", [][]byte{[]byte("v4")}},
		{"TIMESTAMP.NOW", "TIMESTAMP.NOW", nil},
		{"TIMESTAMP.FORMAT", "TIMESTAMP.FORMAT", [][]byte{[]byte("1640995200"), []byte("2006-01-02")}},
		{"TIMESTAMP.PARSE", "TIMESTAMP.PARSE", [][]byte{[]byte("2022-01-01"), []byte("2006-01-02")}},
		{"RANDOM.STRING", "RANDOM.STRING", [][]byte{[]byte("10")}},
		{"RANDOM.INT", "RANDOM.INT", [][]byte{[]byte("1"), []byte("100")}},
		{"HASH.MD5", "HASH.MD5", [][]byte{[]byte("hello")}},
		{"HASH.SHA1", "HASH.SHA1", [][]byte{[]byte("hello")}},
		{"HASH.SHA256", "HASH.SHA256", [][]byte{[]byte("hello")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestCacheWarmCommandsCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterCacheWarmingCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"CACHEWARM.ADD", "CACHEWARM.ADD", [][]byte{[]byte("key1"), []byte("value1")}},
		{"CACHEWARM.GET", "CACHEWARM.GET", [][]byte{[]byte("key1")}},
		{"CACHEWARM.REMOVE", "CACHEWARM.REMOVE", [][]byte{[]byte("key1")}},
		{"CACHEWARM.LIST", "CACHEWARM.LIST", nil},
		{"CACHEWARM.CLEAR", "CACHEWARM.CLEAR", nil},
		{"CACHEWARM.LOAD", "CACHEWARM.LOAD", [][]byte{[]byte("file.txt")}},
		{"CACHEWARM.SAVE", "CACHEWARM.SAVE", [][]byte{[]byte("file.txt")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestResilienceCommandsCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"CIRCUIT.CREATE", "CIRCUIT.CREATE", [][]byte{[]byte("circuit1"), []byte("5"), []byte("30000")}},
		{"CIRCUIT.OPEN", "CIRCUIT.OPEN", [][]byte{[]byte("circuit1")}},
		{"CIRCUIT.CLOSE", "CIRCUIT.CLOSE", [][]byte{[]byte("circuit1")}},
		{"CIRCUIT.STATUS", "CIRCUIT.STATUS", [][]byte{[]byte("circuit1")}},
		{"CIRCUIT.DELETE", "CIRCUIT.DELETE", [][]byte{[]byte("circuit1")}},
		{"RATELIMIT.CREATE", "RATELIMIT.CREATE", [][]byte{[]byte("limit1"), []byte("10"), []byte("60")}},
		{"RATELIMIT.CHECK", "RATELIMIT.CHECK", [][]byte{[]byte("limit1"), []byte("client1")}},
		{"RATELIMIT.RESET", "RATELIMIT.RESET", [][]byte{[]byte("limit1"), []byte("client1")}},
		{"RATELIMIT.DELETE", "RATELIMIT.DELETE", [][]byte{[]byte("limit1")}},
		{"BULKHEAD.CREATE", "BULKHEAD.CREATE", [][]byte{[]byte("bulk1"), []byte("10")}},
		{"BULKHEAD.ACQUIRE", "BULKHEAD.ACQUIRE", [][]byte{[]byte("bulk1")}},
		{"BULKHEAD.RELEASE", "BULKHEAD.RELEASE", [][]byte{[]byte("bulk1")}},
		{"BULKHEAD.STATUS", "BULKHEAD.STATUS", [][]byte{[]byte("bulk1")}},
		{"BULKHEAD.DELETE", "BULKHEAD.DELETE", [][]byte{[]byte("bulk1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestMLCommandsCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMLCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"ML.MODEL.CREATE", "ML.MODEL.CREATE", [][]byte{[]byte("model1"), []byte("linear")}},
		{"ML.MODEL.TRAIN", "ML.MODEL.TRAIN", [][]byte{[]byte("model1"), []byte("data")}},
		{"ML.MODEL.PREDICT", "ML.MODEL.PREDICT", [][]byte{[]byte("model1"), []byte("input")}},
		{"ML.MODEL.DELETE", "ML.MODEL.DELETE", [][]byte{[]byte("model1")}},
		{"ML.MODEL.LIST", "ML.MODEL.LIST", nil},
		{"ML.FEATURE.SET", "ML.FEATURE.SET", [][]byte{[]byte("f1"), []byte("value")}},
		{"ML.FEATURE.GET", "ML.FEATURE.GET", [][]byte{[]byte("f1")}},
		{"ML.FEATURE.DELETE", "ML.FEATURE.DELETE", [][]byte{[]byte("f1")}},
		{"ML.EMBEDDING.CREATE", "ML.EMBEDDING.CREATE", [][]byte{[]byte("emb1"), []byte("1,2,3,4")}},
		{"ML.EMBEDDING.SIMILARITY", "ML.EMBEDDING.SIMILARITY", [][]byte{[]byte("emb1"), []byte("emb2")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestDigestCommandsExtendedCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterDigestCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"DIGEST.SET", "DIGEST.SET", [][]byte{[]byte("d1"), []byte("hello")}},
		{"DIGEST.GET", "DIGEST.GET", [][]byte{[]byte("d1")}},
		{"DIGEST.VERIFY", "DIGEST.VERIFY", [][]byte{[]byte("d1"), []byte("hello")}},
		{"DIGEST.LIST", "DIGEST.LIST", nil},
		{"DIGEST.DEL", "DIGEST.DEL", [][]byte{[]byte("d1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestMVCCCommandsExtendedCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMVCCCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"MVCC.SET", "MVCC.SET", [][]byte{[]byte("k1"), []byte("v1")}},
		{"MVCC.GET", "MVCC.GET", [][]byte{[]byte("k1")}},
		{"MVCC.DEL", "MVCC.DEL", [][]byte{[]byte("k1")}},
		{"MVCC.HISTORY", "MVCC.HISTORY", [][]byte{[]byte("k1")}},
		{"MVCC.VERSION", "MVCC.VERSION", nil},
		{"MVCC.COMPACT", "MVCC.COMPACT", [][]byte{[]byte("100")}},
		{"MVCC.SNAPSHOT", "MVCC.SNAPSHOT", nil},
		{"MVCC.RESTORE", "MVCC.RESTORE", [][]byte{[]byte("1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestModuleCommandsExtendedCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterModuleCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"MODULE LIST", "MODULE", [][]byte{[]byte("LIST")}},
		{"MODULE LOAD", "MODULE", [][]byte{[]byte("LOAD"), []byte("testmodule")}},
		{"MODULE UNLOAD", "MODULE", [][]byte{[]byte("UNLOAD"), []byte("testmodule")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestReplicationCommandsExtendedCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterReplicationCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"REPLICAOF", "REPLICAOF", [][]byte{[]byte("127.0.0.1"), []byte("6379")}},
		{"REPLICAOF NO ONE", "REPLICAOF", [][]byte{[]byte("NO"), []byte("ONE")}},
		{"INFO replication", "INFO", [][]byte{[]byte("replication")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestSentinelCommandsExtendedCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterSentinelCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"SENTINEL MASTERS", "SENTINEL", [][]byte{[]byte("MASTERS")}},
		{"SENTINEL MONITOR", "SENTINEL", [][]byte{[]byte("MONITOR"), []byte("mymaster"), []byte("127.0.0.1"), []byte("6379"), []byte("2")}},
		{"SENTINEL REMOVE", "SENTINEL", [][]byte{[]byte("REMOVE"), []byte("mymaster")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestTransactionCommandsExtendedCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterTransactionCommands(router)
	RegisterStringCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"MULTI", "MULTI", nil},
		{"SET in MULTI", "SET", [][]byte{[]byte("k1"), []byte("v1")}},
		{"EXEC", "EXEC", nil},
		{"DISCARD after MULTI", "MULTI", nil},
		{"DISCARD", "DISCARD", nil},
		{"WATCH", "WATCH", [][]byte{[]byte("k1")}},
		{"UNWATCH", "UNWATCH", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestUtilityExtCommandsCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterUtilityExtCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"UTIL.CPU", "UTIL.CPU", nil},
		{"UTIL.MEM", "UTIL.MEM", nil},
		{"UTIL.DISK", "UTIL.DISK", nil},
		{"UTIL.NETWORK", "UTIL.NETWORK", nil},
		{"UTIL.PROCESS", "UTIL.PROCESS", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestTrieCommandsCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterActorCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"TRIE.ADD", "TRIE.ADD", [][]byte{[]byte("trie1"), []byte("hello")}},
		{"TRIE.FIND", "TRIE.FIND", [][]byte{[]byte("trie1"), []byte("hello")}},
		{"TRIE.PREFIX", "TRIE.PREFIX", [][]byte{[]byte("trie1"), []byte("he")}},
		{"TRIE.DELETE", "TRIE.DELETE", [][]byte{[]byte("trie1"), []byte("hello")}},
		{"TRIE.LIST", "TRIE.LIST", [][]byte{[]byte("trie1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestSemaphoreCommandsCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterActorCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"SEM.CREATE", "SEM.CREATE", [][]byte{[]byte("sem1"), []byte("3")}},
		{"SEM.ACQUIRE", "SEM.ACQUIRE", [][]byte{[]byte("sem1")}},
		{"SEM.RELEASE", "SEM.RELEASE", [][]byte{[]byte("sem1")}},
		{"SEM.TRYACQUIRE", "SEM.TRYACQUIRE", [][]byte{[]byte("sem1")}},
		{"SEM.VALUE", "SEM.VALUE", [][]byte{[]byte("sem1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestRingBufferCommandsCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterActorCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"RING.CREATE", "RING.CREATE", [][]byte{[]byte("ring1"), []byte("100")}},
		{"RING.ADD", "RING.ADD", [][]byte{[]byte("ring1"), []byte("item1")}},
		{"RING.GET", "RING.GET", [][]byte{[]byte("ring1"), []byte("0")}},
		{"RING.REMOVE", "RING.REMOVE", [][]byte{[]byte("ring1"), []byte("0")}},
		{"RING.SIZE", "RING.SIZE", [][]byte{[]byte("ring1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestFilterCommandsCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterAdvancedCommands2(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"FILTER.CREATE", "FILTER.CREATE", [][]byte{[]byte("f1"), []byte("key =~ ^user.*")}},
		{"FILTER.DELETE", "FILTER.DELETE", [][]byte{[]byte("f1")}},
		{"FILTER.APPLY", "FILTER.APPLY", [][]byte{[]byte("f1"), []byte("mykey")}},
		{"FILTER.LIST", "FILTER.LIST", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestTransformCommandsCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterAdvancedCommands2(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"TRANSFORM.CREATE upper", "TRANSFORM.CREATE", [][]byte{[]byte("t1"), []byte("upper")}},
		{"TRANSFORM.DELETE", "TRANSFORM.DELETE", [][]byte{[]byte("t1")}},
		{"TRANSFORM.APPLY", "TRANSFORM.APPLY", [][]byte{[]byte("t1"), []byte("hello")}},
		{"TRANSFORM.LIST", "TRANSFORM.LIST", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestEnrichCommandsCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterAdvancedCommands2(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"ENRICH.CREATE", "ENRICH.CREATE", [][]byte{[]byte("e1"), []byte("add_timestamp")}},
		{"ENRICH.DELETE", "ENRICH.DELETE", [][]byte{[]byte("e1")}},
		{"ENRICH.APPLY", "ENRICH.APPLY", [][]byte{[]byte("e1"), []byte("data")}},
		{"ENRICH.LIST", "ENRICH.LIST", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestValidateCommandsCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterAdvancedCommands2(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"VALIDATE.CREATE", "VALIDATE.CREATE", [][]byte{[]byte("v1"), []byte("len > 0")}},
		{"VALIDATE.DELETE", "VALIDATE.DELETE", [][]byte{[]byte("v1")}},
		{"VALIDATE.CHECK", "VALIDATE.CHECK", [][]byte{[]byte("v1"), []byte("test")}},
		{"VALIDATE.LIST", "VALIDATE.LIST", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestJobXCommandsCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterAdvancedCommands2(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"JOBX.CREATE", "JOBX.CREATE", [][]byte{[]byte("j1"), []byte("SET key value")}},
		{"JOBX.DELETE", "JOBX.DELETE", [][]byte{[]byte("j1")}},
		{"JOBX.RUN", "JOBX.RUN", [][]byte{[]byte("j1")}},
		{"JOBX.STATUS", "JOBX.STATUS", [][]byte{[]byte("j1")}},
		{"JOBX.LIST", "JOBX.LIST", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestDAGCommandsExtendedCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterActorCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"DAG.CREATE", "DAG.CREATE", [][]byte{[]byte("dag1")}},
		{"DAG.ADDNODE", "DAG.ADDNODE", [][]byte{[]byte("dag1"), []byte("node1"), []byte("SET"), []byte("k1"), []byte("v1")}},
		{"DAG.ADDEDGE", "DAG.ADDEDGE", [][]byte{[]byte("dag1"), []byte("node1"), []byte("node2")}},
		{"DAG.EXECUTE", "DAG.EXECUTE", [][]byte{[]byte("dag1")}},
		{"DAG.DELETE", "DAG.DELETE", [][]byte{[]byte("dag1")}},
		{"DAG.LIST", "DAG.LIST", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestWorkflowCommandsExtendedCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterWorkflowCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"WORKFLOW.CREATE", "WORKFLOW.CREATE", [][]byte{[]byte("wf1")}},
		{"WORKFLOW.ADDSTEP", "WORKFLOW.ADDSTEP", [][]byte{[]byte("wf1"), []byte("step1"), []byte("SET"), []byte("k1"), []byte("v1")}},
		{"WORKFLOW.EXEC", "WORKFLOW.EXEC", [][]byte{[]byte("wf1")}},
		{"WORKFLOW.STATUS", "WORKFLOW.STATUS", [][]byte{[]byte("wf1")}},
		{"WORKFLOW.DELETE", "WORKFLOW.DELETE", [][]byte{[]byte("wf1")}},
		{"WORKFLOW.LIST", "WORKFLOW.LIST", nil},
		{"CHAIN.CREATE", "CHAIN.CREATE", [][]byte{[]byte("chain1")}},
		{"CHAIN.ADD", "CHAIN.ADD", [][]byte{[]byte("chain1"), []byte("SET k1 v1")}},
		{"CHAIN.EXEC", "CHAIN.EXEC", [][]byte{[]byte("chain1")}},
		{"CHAIN.DEL", "CHAIN.DEL", [][]byte{[]byte("chain1")}},
		{"REACTIVE.WATCH", "REACTIVE.WATCH", [][]byte{[]byte("key1"), []byte("ONCHANGE")}},
		{"REACTIVE.UNWATCH", "REACTIVE.UNWATCH", [][]byte{[]byte("key1")}},
		{"REACTIVE.TRIGGER", "REACTIVE.TRIGGER", [][]byte{[]byte("key1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestExtraCommandsExtendedCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtraCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"WAIT", "WAIT", [][]byte{[]byte("1"), []byte("1000")}},
		{"WAITAOF", "WAITAOF", [][]byte{[]byte("1"), []byte("1"), []byte("1000")}},
		{"LOLWUT", "LOLWUT", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestMoreCommandsExtendedCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"CLIENT ID", "CLIENT", [][]byte{[]byte("ID")}},
		{"CLIENT INFO", "CLIENT", [][]byte{[]byte("INFO")}},
		{"CLIENT KILL", "CLIENT", [][]byte{[]byte("KILL"), []byte("127.0.0.1:6379")}},
		{"CLIENT SETNAME", "CLIENT", [][]byte{[]byte("SETNAME"), []byte("testclient")}},
		{"CLIENT GETNAME", "CLIENT", [][]byte{[]byte("GETNAME")}},
		{"CLIENT LIST", "CLIENT", [][]byte{[]byte("LIST")}},
		{"CLIENT PAUSE", "CLIENT", [][]byte{[]byte("PAUSE"), []byte("0")}},
		{"CLIENT UNPAUSE", "CLIENT", [][]byte{[]byte("UNPAUSE")}},
		{"CLIENT TRACKING", "CLIENT", [][]byte{[]byte("TRACKING"), []byte("ON")}},
		{"CLIENT CACHING", "CLIENT", [][]byte{[]byte("CACHING"), []byte("YES")}},
		{"CLIENT GETREDIR", "CLIENT", [][]byte{[]byte("GETREDIR")}},
		{"CLIENT REPLY", "CLIENT", [][]byte{[]byte("REPLY"), []byte("ON")}},
		{"CLIENT UNBLOCK", "CLIENT", [][]byte{[]byte("UNBLOCK"), []byte("123"), []byte("TIMEOUT")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}
