package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestSchedulerCommandsFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterSchedulerCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"SCHEDULER.CREATE", "SCHEDULER.CREATE", [][]byte{[]byte("job1"), []byte("* * * * *")}},
		{"SCHEDULER.CREATE no args", "SCHEDULER.CREATE", nil},
		{"SCHEDULER.DELETE", "SCHEDULER.DELETE", [][]byte{[]byte("job1")}},
		{"SCHEDULER.DELETE no args", "SCHEDULER.DELETE", nil},
		{"SCHEDULER.LIST", "SCHEDULER.LIST", nil},
		{"SCHEDULER.RUN", "SCHEDULER.RUN", [][]byte{[]byte("job1")}},
		{"SCHEDULER.RUN no args", "SCHEDULER.RUN", nil},
		{"SCHEDULER.ENABLE", "SCHEDULER.ENABLE", [][]byte{[]byte("job1")}},
		{"SCHEDULER.ENABLE no args", "SCHEDULER.ENABLE", nil},
		{"SCHEDULER.DISABLE", "SCHEDULER.DISABLE", [][]byte{[]byte("job1")}},
		{"SCHEDULER.DISABLE no args", "SCHEDULER.DISABLE", nil},
		{"SCHEDULER.STATUS", "SCHEDULER.STATUS", [][]byte{[]byte("job1")}},
		{"SCHEDULER.STATUS no args", "SCHEDULER.STATUS", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestSchedulerCommandsPauseResumeFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterSchedulerCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"SCHEDULER.PAUSE", "SCHEDULER.PAUSE", nil},
		{"SCHEDULER.RESUME", "SCHEDULER.RESUME", nil},
		{"SCHEDULER.CLEAR", "SCHEDULER.CLEAR", nil},
		{"SCHEDULER.INFO", "SCHEDULER.INFO", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestWorkflowCommandsExtendedFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterWorkflowCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"WORKFLOW.CREATE", "WORKFLOW.CREATE", [][]byte{[]byte("wf1")}},
		{"WORKFLOW.CREATE no args", "WORKFLOW.CREATE", nil},
		{"WORKFLOW.DELETE", "WORKFLOW.DELETE", [][]byte{[]byte("wf1")}},
		{"WORKFLOW.DELETE no args", "WORKFLOW.DELETE", nil},
		{"WORKFLOW.START", "WORKFLOW.START", [][]byte{[]byte("wf1")}},
		{"WORKFLOW.START no args", "WORKFLOW.START", nil},
		{"WORKFLOW.STOP", "WORKFLOW.STOP", [][]byte{[]byte("wf1")}},
		{"WORKFLOW.STOP no args", "WORKFLOW.STOP", nil},
		{"WORKFLOW.STATUS", "WORKFLOW.STATUS", [][]byte{[]byte("wf1")}},
		{"WORKFLOW.STATUS no args", "WORKFLOW.STATUS", nil},
		{"WORKFLOW.LIST", "WORKFLOW.LIST", nil},
		{"WORKFLOW.ADDSTEP", "WORKFLOW.ADDSTEP", [][]byte{[]byte("wf1"), []byte("step1")}},
		{"WORKFLOW.ADDSTEP no args", "WORKFLOW.ADDSTEP", nil},
		{"WORKFLOW.REMOVESTEP", "WORKFLOW.REMOVESTEP", [][]byte{[]byte("wf1"), []byte("step1")}},
		{"WORKFLOW.REMOVESTEP no args", "WORKFLOW.REMOVESTEP", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestWorkflowCommandsExecutionFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterWorkflowCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"WORKFLOW.EXECUTE", "WORKFLOW.EXECUTE", [][]byte{[]byte("wf1")}},
		{"WORKFLOW.EXECUTE no args", "WORKFLOW.EXECUTE", nil},
		{"WORKFLOW.PAUSE", "WORKFLOW.PAUSE", [][]byte{[]byte("wf1")}},
		{"WORKFLOW.PAUSE no args", "WORKFLOW.PAUSE", nil},
		{"WORKFLOW.RESUME", "WORKFLOW.RESUME", [][]byte{[]byte("wf1")}},
		{"WORKFLOW.RESUME no args", "WORKFLOW.RESUME", nil},
		{"WORKFLOW.RESET", "WORKFLOW.RESET", [][]byte{[]byte("wf1")}},
		{"WORKFLOW.RESET no args", "WORKFLOW.RESET", nil},
		{"WORKFLOW.LOG", "WORKFLOW.LOG", [][]byte{[]byte("wf1")}},
		{"WORKFLOW.LOG no args", "WORKFLOW.LOG", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestReplicationCommandsFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterReplicationCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"REPLICAOF", "REPLICAOF", [][]byte{[]byte("127.0.0.1"), []byte("6379")}},
		{"REPLICAOF no one", "REPLICAOF", [][]byte{[]byte("NO"), []byte("ONE")}},
		{"REPLICAOF no args", "REPLICAOF", nil},
		{"ROLE", "ROLE", nil},
		{"SLAVEOF", "SLAVEOF", [][]byte{[]byte("127.0.0.1"), []byte("6379")}},
		{"SLAVEOF no one", "SLAVEOF", [][]byte{[]byte("NO"), []byte("ONE")}},
		{"SLAVEOF no args", "SLAVEOF", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestReplicationCommandsSyncFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterReplicationCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"SYNC", "SYNC", nil},
		{"PSYNC", "PSYNC", [][]byte{[]byte("?"), []byte("-1")}},
		{"PSYNC no args", "PSYNC", nil},
		{"WAIT", "WAIT", [][]byte{[]byte("1"), []byte("1000")}},
		{"WAIT no args", "WAIT", nil},
		{"WAITAOF", "WAITAOF", [][]byte{[]byte("1"), []byte("1000")}},
		{"WAITAOF no args", "WAITAOF", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestReplicationCommandsInfoFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterReplicationCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"INFO REPLICATION", "INFO", [][]byte{[]byte("REPLICATION")}},
		{"INFO no args", "INFO", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestFunctionCommandsFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterFunctionCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"FUNCTION CREATE", "FUNCTION", [][]byte{[]byte("CREATE"), []byte("func1"), []byte("return 1")}},
		{"FUNCTION CREATE no args", "FUNCTION", [][]byte{[]byte("CREATE")}},
		{"FUNCTION DELETE", "FUNCTION", [][]byte{[]byte("DELETE"), []byte("func1")}},
		{"FUNCTION DELETE no args", "FUNCTION", [][]byte{[]byte("DELETE")}},
		{"FUNCTION LIST", "FUNCTION", [][]byte{[]byte("LIST")}},
		{"FUNCTION KILL", "FUNCTION", [][]byte{[]byte("KILL")}},
		{"FUNCTION FLUSH", "FUNCTION", [][]byte{[]byte("FLUSH")}},
		{"FUNCTION no args", "FUNCTION", nil},
		{"FUNCTION unknown", "FUNCTION", [][]byte{[]byte("UNKNOWN")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestFunctionCommandsCallFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterFunctionCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"FCALL", "FCALL", [][]byte{[]byte("func1")}},
		{"FCALL with args", "FCALL", [][]byte{[]byte("func1"), []byte("arg1"), []byte("arg2")}},
		{"FCALL no args", "FCALL", nil},
		{"FCALL_RO", "FCALL_RO", [][]byte{[]byte("func1")}},
		{"FCALL_RO no args", "FCALL_RO", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestStatsCommandsFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStatsCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"TDIGEST.CREATE", "TDIGEST.CREATE", [][]byte{[]byte("td1"), []byte("100")}},
		{"TDIGEST.CREATE no args", "TDIGEST.CREATE", nil},
		{"TDIGEST.ADD", "TDIGEST.ADD", [][]byte{[]byte("td1"), []byte("1.0"), []byte("2.0"), []byte("3.0")}},
		{"TDIGEST.ADD no args", "TDIGEST.ADD", nil},
		{"TDIGEST.QUANTILE", "TDIGEST.QUANTILE", [][]byte{[]byte("td1"), []byte("0.5")}},
		{"TDIGEST.QUANTILE no args", "TDIGEST.QUANTILE", nil},
		{"TDIGEST.CDF", "TDIGEST.CDF", [][]byte{[]byte("td1"), []byte("2.0")}},
		{"TDIGEST.CDF no args", "TDIGEST.CDF", nil},
		{"TDIGEST.MEAN", "TDIGEST.MEAN", [][]byte{[]byte("td1")}},
		{"TDIGEST.MEAN no args", "TDIGEST.MEAN", nil},
		{"TDIGEST.MIN", "TDIGEST.MIN", [][]byte{[]byte("td1")}},
		{"TDIGEST.MIN no args", "TDIGEST.MIN", nil},
		{"TDIGEST.MAX", "TDIGEST.MAX", [][]byte{[]byte("td1")}},
		{"TDIGEST.MAX no args", "TDIGEST.MAX", nil},
		{"TDIGEST.INFO", "TDIGEST.INFO", [][]byte{[]byte("td1")}},
		{"TDIGEST.INFO no args", "TDIGEST.INFO", nil},
		{"TDIGEST.RESET", "TDIGEST.RESET", [][]byte{[]byte("td1")}},
		{"TDIGEST.RESET no args", "TDIGEST.RESET", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestUtilityCommandsFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterUtilityCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"ID.CREATE", "ID.CREATE", [][]byte{[]byte("gen1")}},
		{"ID.CREATE no args", "ID.CREATE", nil},
		{"ID.NEXT", "ID.NEXT", [][]byte{[]byte("gen1")}},
		{"ID.NEXT no args", "ID.NEXT", nil},
		{"ID.NEXTN", "ID.NEXTN", [][]byte{[]byte("gen1"), []byte("5")}},
		{"ID.NEXTN no args", "ID.NEXTN", nil},
		{"ID.CURRENT", "ID.CURRENT", [][]byte{[]byte("gen1")}},
		{"ID.CURRENT no args", "ID.CURRENT", nil},
		{"ID.SET", "ID.SET", [][]byte{[]byte("gen1"), []byte("100")}},
		{"ID.SET no args", "ID.SET", nil},
		{"ID.DELETE", "ID.DELETE", [][]byte{[]byte("gen1")}},
		{"ID.DELETE no args", "ID.DELETE", nil},
		{"SNOWFLAKE.NEXT", "SNOWFLAKE.NEXT", nil},
		{"SNOWFLAKE.PARSE", "SNOWFLAKE.PARSE", [][]byte{[]byte("1234567890123456789")}},
		{"SNOWFLAKE.PARSE no args", "SNOWFLAKE.PARSE", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestUtilityExtCommandsFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterUtilityExtCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"UUID.NEXT", "UUID.NEXT", nil},
		{"UUID.NEXT count", "UUID.NEXT", [][]byte{[]byte("5")}},
		{"UUID.PARSE", "UUID.PARSE", [][]byte{[]byte("550e8400-e29b-41d4-a716-446655440000")}},
		{"UUID.PARSE no args", "UUID.PARSE", nil},
		{"UUID.VALIDATE", "UUID.VALIDATE", [][]byte{[]byte("550e8400-e29b-41d4-a716-446655440000")}},
		{"UUID.VALIDATE no args", "UUID.VALIDATE", nil},
		{"ULID.NEXT", "ULID.NEXT", nil},
		{"ULID.NEXT count", "ULID.NEXT", [][]byte{[]byte("5")}},
		{"ULID.EXTRACT", "ULID.EXTRACT", [][]byte{[]byte("01H1Q1Q1Q1Q1Q1Q1Q1Q1Q1Q1")}},
		{"ULID.EXTRACT no args", "ULID.EXTRACT", nil},
		{"ULID.VALIDATE", "ULID.VALIDATE", [][]byte{[]byte("01H1Q1Q1Q1Q1Q1Q1Q1Q1Q1Q1")}},
		{"ULID.VALIDATE no args", "ULID.VALIDATE", nil},
		{"TIMESTAMP.NOW", "TIMESTAMP.NOW", nil},
		{"TIMESTAMP.PARSE", "TIMESTAMP.PARSE", [][]byte{[]byte("2024-01-01T00:00:00Z"), []byte("2006-01-02T15:04:05Z")}},
		{"TIMESTAMP.PARSE no args", "TIMESTAMP.PARSE", nil},
		{"TIMESTAMP.FORMAT", "TIMESTAMP.FORMAT", [][]byte{[]byte("1704067200"), []byte("2006-01-02")}},
		{"TIMESTAMP.FORMAT no args", "TIMESTAMP.FORMAT", nil},
		{"TIMESTAMP.ADD", "TIMESTAMP.ADD", [][]byte{[]byte("1704067200"), []byte("1h")}},
		{"TIMESTAMP.ADD no args", "TIMESTAMP.ADD", nil},
		{"TIMESTAMP.DIFF", "TIMESTAMP.DIFF", [][]byte{[]byte("1704067200"), []byte("1704153600")}},
		{"TIMESTAMP.DIFF no args", "TIMESTAMP.DIFF", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestMonitoringCommandsFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMonitoringCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"MONITOR.START", "MONITOR.START", nil},
		{"MONITOR.STOP", "MONITOR.STOP", nil},
		{"MONITOR.STATUS", "MONITOR.STATUS", nil},
		{"METRIC.GET", "METRIC.GET", [][]byte{[]byte("metric1")}},
		{"METRIC.GET no args", "METRIC.GET", nil},
		{"METRIC.RESET", "METRIC.RESET", [][]byte{[]byte("metric1")}},
		{"METRIC.RESET no args", "METRIC.RESET", nil},
		{"METRIC.LIST", "METRIC.LIST", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestStreamCommandsFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStreamCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"XADD", "XADD", [][]byte{[]byte("stream1"), []byte("*"), []byte("field1"), []byte("value1")}},
		{"XADD no args", "XADD", nil},
		{"XRANGE", "XRANGE", [][]byte{[]byte("stream1"), []byte("-"), []byte("+")}},
		{"XRANGE no args", "XRANGE", nil},
		{"XREVRANGE", "XREVRANGE", [][]byte{[]byte("stream1"), []byte("+"), []byte("-")}},
		{"XREVRANGE no args", "XREVRANGE", nil},
		{"XLEN", "XLEN", [][]byte{[]byte("stream1")}},
		{"XLEN no args", "XLEN", nil},
		{"XDEL", "XDEL", [][]byte{[]byte("stream1"), []byte("1-0")}},
		{"XDEL no args", "XDEL", nil},
		{"XTRIM", "XTRIM", [][]byte{[]byte("stream1"), []byte("MAXLEN"), []byte("100")}},
		{"XTRIM no args", "XTRIM", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}
