package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestResilienceCommandsMEMORYXFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"MEMORYX.ALLOC", "MEMORYX.ALLOC", [][]byte{[]byte("1000")}},
		{"MEMORYX.ALLOC no args", "MEMORYX.ALLOC", nil},
		{"MEMORYX.FREE", "MEMORYX.FREE", [][]byte{[]byte("ptr1")}},
		{"MEMORYX.FREE not found", "MEMORYX.FREE", [][]byte{[]byte("notfound")}},
		{"MEMORYX.FREE no args", "MEMORYX.FREE", nil},
		{"MEMORYX.STATS", "MEMORYX.STATS", nil},
		{"MEMORYX.TRACK start", "MEMORYX.TRACK", [][]byte{[]byte("START")}},
		{"MEMORYX.TRACK stop", "MEMORYX.TRACK", [][]byte{[]byte("STOP")}},
		{"MEMORYX.TRACK no args", "MEMORYX.TRACK", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestResilienceCommandsOBSERVABILITYFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"OBSERVABILITY.START", "OBSERVABILITY.START", nil},
		{"OBSERVABILITY.STOP", "OBSERVABILITY.STOP", nil},
		{"OBSERVABILITY.METRIC", "OBSERVABILITY.METRIC", [][]byte{[]byte("metric1"), []byte("100")}},
		{"OBSERVABILITY.METRIC no args", "OBSERVABILITY.METRIC", nil},
		{"OBSERVABILITY.TRACE", "OBSERVABILITY.TRACE", [][]byte{[]byte("trace1")}},
		{"OBSERVABILITY.TRACE no args", "OBSERVABILITY.TRACE", nil},
		{"OBSERVABILITY.LOG", "OBSERVABILITY.LOG", [][]byte{[]byte("INFO"), []byte("test message")}},
		{"OBSERVABILITY.LOG no args", "OBSERVABILITY.LOG", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestResilienceCommandsDIAGNOSTICFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"DIAGNOSTIC.RUN", "DIAGNOSTIC.RUN", nil},
		{"DIAGNOSTIC.STATUS", "DIAGNOSTIC.STATUS", nil},
		{"DIAGNOSTIC.REPORT", "DIAGNOSTIC.REPORT", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestResilienceCommandsEVENTSOURCINGFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"EVENTSOURCING.CREATE", "EVENTSOURCING.CREATE", [][]byte{[]byte("stream1")}},
		{"EVENTSOURCING.CREATE no args", "EVENTSOURCING.CREATE", nil},
		{"EVENTSOURCING.APPEND", "EVENTSOURCING.APPEND", [][]byte{[]byte("stream1"), []byte("event1"), []byte("data")}},
		{"EVENTSOURCING.APPEND no args", "EVENTSOURCING.APPEND", nil},
		{"EVENTSOURCING.GET", "EVENTSOURCING.GET", [][]byte{[]byte("stream1"), []byte("0")}},
		{"EVENTSOURCING.GET not found", "EVENTSOURCING.GET", [][]byte{[]byte("notfound"), []byte("0")}},
		{"EVENTSOURCING.GET no args", "EVENTSOURCING.GET", nil},
		{"EVENTSOURCING.SNAPSHOT", "EVENTSOURCING.SNAPSHOT", [][]byte{[]byte("stream1")}},
		{"EVENTSOURCING.SNAPSHOT no args", "EVENTSOURCING.SNAPSHOT", nil},
		{"EVENTSOURCING.RESTORE", "EVENTSOURCING.RESTORE", [][]byte{[]byte("stream1"), []byte("snapshot1")}},
		{"EVENTSOURCING.RESTORE no args", "EVENTSOURCING.RESTORE", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestResilienceCommandsBACKPRESSUREFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"BACKPRESSURE.ENABLE", "BACKPRESSURE.ENABLE", [][]byte{[]byte("queue1")}},
		{"BACKPRESSURE.ENABLE no args", "BACKPRESSURE.ENABLE", nil},
		{"BACKPRESSURE.DISABLE", "BACKPRESSURE.DISABLE", [][]byte{[]byte("queue1")}},
		{"BACKPRESSURE.DISABLE no args", "BACKPRESSURE.DISABLE", nil},
		{"BACKPRESSURE.CHECK", "BACKPRESSURE.CHECK", [][]byte{[]byte("queue1")}},
		{"BACKPRESSURE.CHECK not found", "BACKPRESSURE.CHECK", [][]byte{[]byte("notfound")}},
		{"BACKPRESSURE.CHECK no args", "BACKPRESSURE.CHECK", nil},
		{"BACKPRESSURE.ADJUST", "BACKPRESSURE.ADJUST", [][]byte{[]byte("queue1"), []byte("100")}},
		{"BACKPRESSURE.ADJUST no args", "BACKPRESSURE.ADJUST", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestResilienceCommandsDEBOUNCEXFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"DEBOUNCEX.CREATE", "DEBOUNCEX.CREATE", [][]byte{[]byte("deb1"), []byte("1000")}},
		{"DEBOUNCEX.CREATE no args", "DEBOUNCEX.CREATE", nil},
		{"DEBOUNCEX.CALL", "DEBOUNCEX.CALL", [][]byte{[]byte("deb1"), []byte("key1")}},
		{"DEBOUNCEX.CALL not found", "DEBOUNCEX.CALL", [][]byte{[]byte("notfound"), []byte("key1")}},
		{"DEBOUNCEX.CALL no args", "DEBOUNCEX.CALL", nil},
		{"DEBOUNCEX.RESET", "DEBOUNCEX.RESET", [][]byte{[]byte("deb1")}},
		{"DEBOUNCEX.RESET no args", "DEBOUNCEX.RESET", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestResilienceCommandsAGGREGATORFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"AGGREGATOR.CREATE", "AGGREGATOR.CREATE", [][]byte{[]byte("agg1"), []byte("SUM")}},
		{"AGGREGATOR.CREATE no args", "AGGREGATOR.CREATE", nil},
		{"AGGREGATOR.ADD", "AGGREGATOR.ADD", [][]byte{[]byte("agg1"), []byte("100")}},
		{"AGGREGATOR.ADD no args", "AGGREGATOR.ADD", nil},
		{"AGGREGATOR.GET", "AGGREGATOR.GET", [][]byte{[]byte("agg1")}},
		{"AGGREGATOR.GET not found", "AGGREGATOR.GET", [][]byte{[]byte("notfound")}},
		{"AGGREGATOR.GET no args", "AGGREGATOR.GET", nil},
		{"AGGREGATOR.RESET", "AGGREGATOR.RESET", [][]byte{[]byte("agg1")}},
		{"AGGREGATOR.RESET no args", "AGGREGATOR.RESET", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestResilienceCommandsJOINXFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"JOINX.CREATE", "JOINX.CREATE", [][]byte{[]byte("join1"), []byte("2")}},
		{"JOINX.CREATE no args", "JOINX.CREATE", nil},
		{"JOINX.ADD", "JOINX.ADD", [][]byte{[]byte("join1"), []byte("stream1"), []byte("data")}},
		{"JOINX.ADD no args", "JOINX.ADD", nil},
		{"JOINX.GET", "JOINX.GET", [][]byte{[]byte("join1")}},
		{"JOINX.GET no args", "JOINX.GET", nil},
		{"JOINX.CLEAR", "JOINX.CLEAR", [][]byte{[]byte("join1")}},
		{"JOINX.CLEAR no args", "JOINX.CLEAR", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestResilienceCommandsPARTITIONXFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"PARTITIONX.CREATE", "PARTITIONX.CREATE", [][]byte{[]byte("part1"), []byte("4")}},
		{"PARTITIONX.CREATE no args", "PARTITIONX.CREATE", nil},
		{"PARTITIONX.ADD", "PARTITIONX.ADD", [][]byte{[]byte("part1"), []byte("key1"), []byte("data")}},
		{"PARTITIONX.ADD no args", "PARTITIONX.ADD", nil},
		{"PARTITIONX.GET", "PARTITIONX.GET", [][]byte{[]byte("part1"), []byte("key1")}},
		{"PARTITIONX.GET not found", "PARTITIONX.GET", [][]byte{[]byte("notfound"), []byte("key1")}},
		{"PARTITIONX.GET no args", "PARTITIONX.GET", nil},
		{"PARTITIONX.REMOVE", "PARTITIONX.REMOVE", [][]byte{[]byte("part1"), []byte("key1")}},
		{"PARTITIONX.REMOVE no args", "PARTITIONX.REMOVE", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestResilienceCommandsWINDOWXFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"WINDOWX.CREATE", "WINDOWX.CREATE", [][]byte{[]byte("win1"), []byte("10"), []byte("60000")}},
		{"WINDOWX.CREATE no args", "WINDOWX.CREATE", nil},
		{"WINDOWX.ADD", "WINDOWX.ADD", [][]byte{[]byte("win1"), []byte("item1")}},
		{"WINDOWX.ADD no args", "WINDOWX.ADD", nil},
		{"WINDOWX.COUNT", "WINDOWX.COUNT", [][]byte{[]byte("win1")}},
		{"WINDOWX.COUNT no args", "WINDOWX.COUNT", nil},
		{"WINDOWX.CLEAR", "WINDOWX.CLEAR", [][]byte{[]byte("win1")}},
		{"WINDOWX.CLEAR no args", "WINDOWX.CLEAR", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestResilienceCommandsSTREAMXFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"STREAMX.CREATE", "STREAMX.CREATE", [][]byte{[]byte("stream1")}},
		{"STREAMX.CREATE no args", "STREAMX.CREATE", nil},
		{"STREAMX.PUBLISH", "STREAMX.PUBLISH", [][]byte{[]byte("stream1"), []byte("data")}},
		{"STREAMX.PUBLISH no args", "STREAMX.PUBLISH", nil},
		{"STREAMX.SUBSCRIBE", "STREAMX.SUBSCRIBE", [][]byte{[]byte("stream1"), []byte("sub1")}},
		{"STREAMX.SUBSCRIBE no args", "STREAMX.SUBSCRIBE", nil},
		{"STREAMX.UNSUBSCRIBE", "STREAMX.UNSUBSCRIBE", [][]byte{[]byte("stream1"), []byte("sub1")}},
		{"STREAMX.UNSUBSCRIBE no args", "STREAMX.UNSUBSCRIBE", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestResilienceCommandsCIRCUITBREAKERFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"CIRCUITBREAKER.CREATE", "CIRCUITBREAKER.CREATE", [][]byte{[]byte("cb1"), []byte("5"), []byte("60000")}},
		{"CIRCUITBREAKER.CREATE no args", "CIRCUITBREAKER.CREATE", nil},
		{"CIRCUITBREAKER.CALL", "CIRCUITBREAKER.CALL", [][]byte{[]byte("cb1")}},
		{"CIRCUITBREAKER.CALL not found", "CIRCUITBREAKER.CALL", [][]byte{[]byte("notfound")}},
		{"CIRCUITBREAKER.CALL no args", "CIRCUITBREAKER.CALL", nil},
		{"CIRCUITBREAKER.STATUS", "CIRCUITBREAKER.STATUS", [][]byte{[]byte("cb1")}},
		{"CIRCUITBREAKER.STATUS no args", "CIRCUITBREAKER.STATUS", nil},
		{"CIRCUITBREAKER.RESET", "CIRCUITBREAKER.RESET", [][]byte{[]byte("cb1")}},
		{"CIRCUITBREAKER.RESET no args", "CIRCUITBREAKER.RESET", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestResilienceCommandsBULKHEADFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"BULKHEAD.CREATE", "BULKHEAD.CREATE", [][]byte{[]byte("bh1"), []byte("10")}},
		{"BULKHEAD.CREATE no args", "BULKHEAD.CREATE", nil},
		{"BULKHEAD.ACQUIRE", "BULKHEAD.ACQUIRE", [][]byte{[]byte("bh1")}},
		{"BULKHEAD.ACQUIRE not found", "BULKHEAD.ACQUIRE", [][]byte{[]byte("notfound")}},
		{"BULKHEAD.ACQUIRE no args", "BULKHEAD.ACQUIRE", nil},
		{"BULKHEAD.RELEASE", "BULKHEAD.RELEASE", [][]byte{[]byte("bh1")}},
		{"BULKHEAD.RELEASE no args", "BULKHEAD.RELEASE", nil},
		{"BULKHEAD.STATUS", "BULKHEAD.STATUS", [][]byte{[]byte("bh1")}},
		{"BULKHEAD.STATUS no args", "BULKHEAD.STATUS", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestResilienceCommandsRATELIMITERFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"RATELIMITER.CREATE", "RATELIMITER.CREATE", [][]byte{[]byte("rl1"), []byte("100"), []byte("60000")}},
		{"RATELIMITER.CREATE no args", "RATELIMITER.CREATE", nil},
		{"RATELIMITER.ALLOW", "RATELIMITER.ALLOW", [][]byte{[]byte("rl1")}},
		{"RATELIMITER.ALLOW not found", "RATELIMITER.ALLOW", [][]byte{[]byte("notfound")}},
		{"RATELIMITER.ALLOW no args", "RATELIMITER.ALLOW", nil},
		{"RATELIMITER.STATUS", "RATELIMITER.STATUS", [][]byte{[]byte("rl1")}},
		{"RATELIMITER.STATUS no args", "RATELIMITER.STATUS", nil},
		{"RATELIMITER.RESET", "RATELIMITER.RESET", [][]byte{[]byte("rl1")}},
		{"RATELIMITER.RESET no args", "RATELIMITER.RESET", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestResilienceCommandsRETRYFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"RETRY.CREATE", "RETRY.CREATE", [][]byte{[]byte("retry1"), []byte("3")}},
		{"RETRY.CREATE no args", "RETRY.CREATE", nil},
		{"RETRY.EXECUTE", "RETRY.EXECUTE", [][]byte{[]byte("retry1")}},
		{"RETRY.EXECUTE not found", "RETRY.EXECUTE", [][]byte{[]byte("notfound")}},
		{"RETRY.EXECUTE no args", "RETRY.EXECUTE", nil},
		{"RETRY.STATUS", "RETRY.STATUS", [][]byte{[]byte("retry1")}},
		{"RETRY.STATUS no args", "RETRY.STATUS", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestResilienceCommandsTIMEOUTFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"TIMEOUT.SET", "TIMEOUT.SET", [][]byte{[]byte("op1"), []byte("5000")}},
		{"TIMEOUT.SET no args", "TIMEOUT.SET", nil},
		{"TIMEOUT.GET", "TIMEOUT.GET", [][]byte{[]byte("op1")}},
		{"TIMEOUT.GET not found", "TIMEOUT.GET", [][]byte{[]byte("notfound")}},
		{"TIMEOUT.GET no args", "TIMEOUT.GET", nil},
		{"TIMEOUT.CLEAR", "TIMEOUT.CLEAR", [][]byte{[]byte("op1")}},
		{"TIMEOUT.CLEAR no args", "TIMEOUT.CLEAR", nil},
		{"TIMEOUT.LIST", "TIMEOUT.LIST", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestResilienceCommandsCACHEXFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"CACHEX.WARM", "CACHEX.WARM", [][]byte{[]byte("key*")}},
		{"CACHEX.WARM no args", "CACHEX.WARM", nil},
		{"CACHEX.INVALIDATE", "CACHEX.INVALIDATE", [][]byte{[]byte("key1")}},
		{"CACHEX.INVALIDATE no args", "CACHEX.INVALIDATE", nil},
		{"CACHEX.PREFETCH", "CACHEX.PREFETCH", [][]byte{[]byte("key*")}},
		{"CACHEX.PREFETCH no args", "CACHEX.PREFETCH", nil},
		{"CACHEX.STATS", "CACHEX.STATS", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}
