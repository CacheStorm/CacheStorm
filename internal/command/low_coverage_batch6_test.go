package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestServerCommandsLowCoverageBatch6(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterServerCommands(router)

	s.Set("key1", &store.StringValue{Data: []byte("value1")}, store.SetOptions{})

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"COMMAND DOCS no args", "COMMAND", [][]byte{[]byte("DOCS")}},
		{"COMMAND DOCS with command", "COMMAND", [][]byte{[]byte("DOCS"), []byte("GET")}},
		{"DUMP key exists", "DUMP", [][]byte{[]byte("key1")}},
		{"DUMP key not found", "DUMP", [][]byte{[]byte("notfound")}},
		{"DUMP no args", "DUMP", nil},
		{"RESTORE key", "RESTORE", [][]byte{[]byte("newkey"), []byte("0"), []byte("data")}},
		{"RESTORE with REPLACE", "RESTORE", [][]byte{[]byte("newkey"), []byte("0"), []byte("data"), []byte("REPLACE")}},
		{"RESTORE no args", "RESTORE", nil},
		{"COPY key exists", "COPY", [][]byte{[]byte("key1"), []byte("key2")}},
		{"COPY with REPLACE", "COPY", [][]byte{[]byte("key1"), []byte("key2"), []byte("REPLACE")}},
		{"COPY no args", "COPY", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestSortedSetCommandsLowCoverageBatch6(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterSortedSetCommands(router)

	s.Set("zset1", &store.SortedSetValue{Members: map[string]float64{"member1": 1.0, "member2": 2.0}}, store.SetOptions{})

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"ZSCAN cursor", "ZSCAN", [][]byte{[]byte("zset1"), []byte("0")}},
		{"ZSCAN with MATCH", "ZSCAN", [][]byte{[]byte("zset1"), []byte("0"), []byte("MATCH"), []byte("member*")}},
		{"ZSCAN with COUNT", "ZSCAN", [][]byte{[]byte("zset1"), []byte("0"), []byte("COUNT"), []byte("10")}},
		{"ZSCAN no args", "ZSCAN", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestStatsCommandsLowCoverageBatch6(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStatsCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"SAMPLE.CREATE sample", "SAMPLE.CREATE", [][]byte{[]byte("sample1")}},
		{"SAMPLE.CREATE no args", "SAMPLE.CREATE", nil},
		{"HISTOGRAM.CREATE hist", "HISTOGRAM.CREATE", [][]byte{[]byte("hist1")}},
		{"HISTOGRAM.CREATE no args", "HISTOGRAM.CREATE", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestStreamCommandsLowCoverageBatch6(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStreamCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"XAUTOCLAIM stream", "XAUTOCLAIM", [][]byte{[]byte("stream1"), []byte("group1"), []byte("consumer1"), []byte("0"), []byte("10")}},
		{"XAUTOCLAIM no args", "XAUTOCLAIM", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestUtilityCommandsLowCoverageBatch6(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterUtilityCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"LOCK.TRY lock", "LOCK.TRY", [][]byte{[]byte("lock1"), []byte("token1"), []byte("30")}},
		{"LOCK.TRY no args", "LOCK.TRY", nil},
		{"LOCK.ACQUIRE lock", "LOCK.ACQUIRE", [][]byte{[]byte("lock1"), []byte("token1"), []byte("30")}},
		{"LOCK.ACQUIRE no args", "LOCK.ACQUIRE", nil},
		{"LOCK.RELEASE lock", "LOCK.RELEASE", [][]byte{[]byte("lock1"), []byte("token1")}},
		{"LOCK.RELEASE no args", "LOCK.RELEASE", nil},
		{"LOCK.RENEW lock", "LOCK.RENEW", [][]byte{[]byte("lock1"), []byte("token1"), []byte("30")}},
		{"LOCK.RENEW no args", "LOCK.RENEW", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestUtilityExtCommandsLowCoverageBatch6(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterUtilityExtCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"FLAG.ADDRULE rule", "FLAG.ADDRULE", [][]byte{[]byte("flag1"), []byte("condition")}},
		{"FLAG.ADDRULE no args", "FLAG.ADDRULE", nil},
		{"BACKUP.CREATE backup", "BACKUP.CREATE", [][]byte{[]byte("backup1")}},
		{"BACKUP.CREATE no args", "BACKUP.CREATE", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestWorkflowCommandsLowCoverageBatch6(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterWorkflowCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"WORKFLOW.FAIL workflow", "WORKFLOW.FAIL", [][]byte{[]byte("wf1"), []byte("error message")}},
		{"WORKFLOW.FAIL no args", "WORKFLOW.FAIL", nil},
		{"CHAINED.SET key", "CHAINED.SET", [][]byte{[]byte("key1"), []byte("value1")}},
		{"CHAINED.SET no args", "CHAINED.SET", nil},
		{"CHAINED.GET key", "CHAINED.GET", [][]byte{[]byte("key1")}},
		{"CHAINED.GET no args", "CHAINED.GET", nil},
		{"CHAINED.DEL key", "CHAINED.DEL", [][]byte{[]byte("key1")}},
		{"CHAINED.DEL no args", "CHAINED.DEL", nil},
		{"REACTIVE.UNWATCH key", "REACTIVE.UNWATCH", [][]byte{[]byte("key1")}},
		{"REACTIVE.UNWATCH no args", "REACTIVE.UNWATCH", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestSchedulerCommandsLowCoverageBatch6(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterSchedulerCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"CIRCUIT.CREATE circuit", "CIRCUIT.CREATE", [][]byte{[]byte("circuit1")}},
		{"CIRCUIT.CREATE no args", "CIRCUIT.CREATE", nil},
		{"SESSION.REFRESH session", "SESSION.REFRESH", [][]byte{[]byte("session1")}},
		{"SESSION.REFRESH no args", "SESSION.REFRESH", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestScriptCommandsLowCoverageBatch6(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterScriptCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"EVAL script", "EVAL", [][]byte{[]byte("return redis.call('GET', KEYS[1])"), []byte("1"), []byte("key1")}},
		{"EVAL no args", "EVAL", nil},
		{"EVALSHA sha", "EVALSHA", [][]byte{[]byte("abc123"), []byte("1"), []byte("key1")}},
		{"EVALSHA no args", "EVALSHA", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}
