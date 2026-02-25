package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestResilienceCommandsExtensive(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"CIRCUITBREAKER.CALL no args", "CIRCUITBREAKER.CALL", nil},
		{"CIRCUITBREAKER.CALL not found", "CIRCUITBREAKER.CALL", [][]byte{[]byte("notfound"), []byte("cmd")}},
		{"CIRCUITBREAKER.RESET no args", "CIRCUITBREAKER.RESET", nil},
		{"CIRCUITBREAKER.RESET not found", "CIRCUITBREAKER.RESET", [][]byte{[]byte("notfound")}},
		{"CIRCUITBREAKER.STATUS no args", "CIRCUITBREAKER.STATUS", nil},
		{"CIRCUITBREAKER.STATUS not found", "CIRCUITBREAKER.STATUS", [][]byte{[]byte("notfound")}},
		{"CIRCUITBREAKER.METRICS no args", "CIRCUITBREAKER.METRICS", nil},
		{"CIRCUITBREAKER.METRICS not found", "CIRCUITBREAKER.METRICS", [][]byte{[]byte("notfound")}},
		{"BULKHEAD.CREATE no args", "BULKHEAD.CREATE", nil},
		{"BULKHEAD.CREATE bh", "BULKHEAD.CREATE", [][]byte{[]byte("bh1"), []byte("10")}},
		{"BULKHEAD.ACQUIRE no args", "BULKHEAD.ACQUIRE", nil},
		{"BULKHEAD.ACQUIRE not found", "BULKHEAD.ACQUIRE", [][]byte{[]byte("notfound")}},
		{"BULKHEAD.RELEASE no args", "BULKHEAD.RELEASE", nil},
		{"BULKHEAD.RELEASE not found", "BULKHEAD.RELEASE", [][]byte{[]byte("notfound")}},
		{"BULKHEAD.STATUS no args", "BULKHEAD.STATUS", nil},
		{"BULKHEAD.STATUS not found", "BULKHEAD.STATUS", [][]byte{[]byte("notfound")}},
		{"RATELIMITER.CREATE no args", "RATELIMITER.CREATE", nil},
		{"RATELIMITER.CREATE rl", "RATELIMITER.CREATE", [][]byte{[]byte("rl1"), []byte("100"), []byte("60")}},
		{"RATELIMITER.ALLOW no args", "RATELIMITER.ALLOW", nil},
		{"RATELIMITER.ALLOW not found", "RATELIMITER.ALLOW", [][]byte{[]byte("notfound")}},
		{"RATELIMITER.STATUS no args", "RATELIMITER.STATUS", nil},
		{"RATELIMITER.STATUS not found", "RATELIMITER.STATUS", [][]byte{[]byte("notfound")}},
		{"RETRY.CREATE no args", "RETRY.CREATE", nil},
		{"RETRY.CREATE retry", "RETRY.CREATE", [][]byte{[]byte("retry1"), []byte("3")}},
		{"RETRY.EXEC no args", "RETRY.EXEC", nil},
		{"RETRY.EXEC not found", "RETRY.EXEC", [][]byte{[]byte("notfound"), []byte("cmd")}},
		{"TIMEOUT.CREATE no args", "TIMEOUT.CREATE", nil},
		{"TIMEOUT.CREATE timeout", "TIMEOUT.CREATE", [][]byte{[]byte("timeout1"), []byte("5000")}},
		{"TIMEOUT.EXEC no args", "TIMEOUT.EXEC", nil},
		{"TIMEOUT.EXEC not found", "TIMEOUT.EXEC", [][]byte{[]byte("notfound"), []byte("cmd")}},
		{"CACHEX.CREATE no args", "CACHEX.CREATE", nil},
		{"CACHEX.CREATE cache", "CACHEX.CREATE", [][]byte{[]byte("cache1"), []byte("100")}},
		{"CACHEX.GET no args", "CACHEX.GET", nil},
		{"CACHEX.GET not found", "CACHEX.GET", [][]byte{[]byte("notfound"), []byte("key")}},
		{"CACHEX.SET no args", "CACHEX.SET", nil},
		{"CACHEX.SET not found", "CACHEX.SET", [][]byte{[]byte("notfound"), []byte("key"), []byte("value")}},
		{"CACHEX.EVICT no args", "CACHEX.EVICT", nil},
		{"CACHEX.EVICT not found", "CACHEX.EVICT", [][]byte{[]byte("notfound")}},
		{"CACHEX.STATS no args", "CACHEX.STATS", nil},
		{"CACHEX.STATS not found", "CACHEX.STATS", [][]byte{[]byte("notfound")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestResilienceCommandsAdvanced(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"MEMORYX.STATS", "MEMORYX.STATS", nil},
		{"MEMORYX.FRAGMENTATION", "MEMORYX.FRAGMENTATION", nil},
		{"MEMORYX.DEFRAG", "MEMORYX.DEFRAG", nil},
		{"OBSERVABILITY.METRICS", "OBSERVABILITY.METRICS", nil},
		{"OBSERVABILITY.TRACE no args", "OBSERVABILITY.TRACE", nil},
		{"OBSERVABILITY.TRACE start", "OBSERVABILITY.TRACE", [][]byte{[]byte("start")}},
		{"DIAGNOSTIC.HEALTH", "DIAGNOSTIC.HEALTH", nil},
		{"DIAGNOSTIC.REPORT", "DIAGNOSTIC.REPORT", nil},
		{"EVENTSOURCING.SAVE no args", "EVENTSOURCING.SAVE", nil},
		{"EVENTSOURCING.SAVE event", "EVENTSOURCING.SAVE", [][]byte{[]byte("event1"), []byte("data")}},
		{"EVENTSOURCING.LOAD no args", "EVENTSOURCING.LOAD", nil},
		{"EVENTSOURCING.LOAD id", "EVENTSOURCING.LOAD", [][]byte{[]byte("event1")}},
		{"BACKPRESSURE.CREATE no args", "BACKPRESSURE.CREATE", nil},
		{"BACKPRESSURE.CREATE bp", "BACKPRESSURE.CREATE", [][]byte{[]byte("bp1"), []byte("100")}},
		{"BACKPRESSURE.RELEASE no args", "BACKPRESSURE.RELEASE", nil},
		{"BACKPRESSURE.RELEASE not found", "BACKPRESSURE.RELEASE", [][]byte{[]byte("notfound")}},
		{"DEBOUNCEX.CREATE no args", "DEBOUNCEX.CREATE", nil},
		{"DEBOUNCEX.CREATE debounce", "DEBOUNCEX.CREATE", [][]byte{[]byte("debounce1"), []byte("1000")}},
		{"DEBOUNCEX.CALL no args", "DEBOUNCEX.CALL", nil},
		{"DEBOUNCEX.CALL not found", "DEBOUNCEX.CALL", [][]byte{[]byte("notfound"), []byte("cmd")}},
		{"AGGREGATOR.CREATE no args", "AGGREGATOR.CREATE", nil},
		{"AGGREGATOR.CREATE agg", "AGGREGATOR.CREATE", [][]byte{[]byte("agg1"), []byte("sum")}},
		{"AGGREGATOR.ADD no args", "AGGREGATOR.ADD", nil},
		{"AGGREGATOR.ADD not found", "AGGREGATOR.ADD", [][]byte{[]byte("notfound"), []byte("100")}},
		{"AGGREGATOR.GET no args", "AGGREGATOR.GET", nil},
		{"AGGREGATOR.GET not found", "AGGREGATOR.GET", [][]byte{[]byte("notfound")}},
		{"JOINX.CREATE no args", "JOINX.CREATE", nil},
		{"JOINX.CREATE join", "JOINX.CREATE", [][]byte{[]byte("join1")}},
		{"JOINX.ADD no args", "JOINX.ADD", nil},
		{"JOINX.ADD not found", "JOINX.ADD", [][]byte{[]byte("notfound"), []byte("stream1"), []byte("data")}},
		{"JOINX.EXEC no args", "JOINX.EXEC", nil},
		{"JOINX.EXEC not found", "JOINX.EXEC", [][]byte{[]byte("notfound")}},
		{"PARTITIONX.CREATE no args", "PARTITIONX.CREATE", nil},
		{"PARTITIONX.CREATE part", "PARTITIONX.CREATE", [][]byte{[]byte("part1"), []byte("4")}},
		{"PARTITIONX.ROUTE no args", "PARTITIONX.ROUTE", nil},
		{"PARTITIONX.ROUTE not found", "PARTITIONX.ROUTE", [][]byte{[]byte("notfound"), []byte("key")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}
