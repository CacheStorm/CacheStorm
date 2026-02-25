package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestMoreCommandsLowCoverageBatch5(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"TRACESPAN start", "TRACESPAN", [][]byte{[]byte("start"), []byte("trace1")}},
		{"TRACESPAN end", "TRACESPAN", [][]byte{[]byte("end"), []byte("trace1")}},
		{"TRACESPAN no args", "TRACESPAN", nil},
		{"LOGX.WRITE message", "LOGX.WRITE", [][]byte{[]byte("log1"), []byte("level"), []byte("message")}},
		{"LOGX.WRITE no args", "LOGX.WRITE", nil},
		{"LOGX.READ log", "LOGX.READ", [][]byte{[]byte("log1")}},
		{"LOGX.READ no args", "LOGX.READ", nil},
		{"QUOTAX.CREATE quota", "QUOTAX.CREATE", [][]byte{[]byte("quota1"), []byte("1000")}},
		{"QUOTAX.CREATE no args", "QUOTAX.CREATE", nil},
		{"METER.CREATE meter", "METER.CREATE", [][]byte{[]byte("meter1")}},
		{"METER.CREATE no args", "METER.CREATE", nil},
		{"TENANT.CREATE tenant", "TENANT.CREATE", [][]byte{[]byte("tenant1")}},
		{"TENANT.CREATE no args", "TENANT.CREATE", nil},
		{"LEASE.CREATE lease", "LEASE.CREATE", [][]byte{[]byte("lease1"), []byte("60")}},
		{"LEASE.CREATE no args", "LEASE.CREATE", nil},
		{"LEASE.RENEW lease", "LEASE.RENEW", [][]byte{[]byte("lease1")}},
		{"LEASE.RENEW no args", "LEASE.RENEW", nil},
		{"BLOOMX.CREATE bloom", "BLOOMX.CREATE", [][]byte{[]byte("bloom1"), []byte("1000"), []byte("0.01")}},
		{"BLOOMX.CREATE no args", "BLOOMX.CREATE", nil},
		{"SKETCH.CREATE sketch", "SKETCH.CREATE", [][]byte{[]byte("sketch1")}},
		{"SKETCH.CREATE no args", "SKETCH.CREATE", nil},
		{"SKETCH.UPDATE item", "SKETCH.UPDATE", [][]byte{[]byte("sketch1"), []byte("item1")}},
		{"SKETCH.UPDATE no args", "SKETCH.UPDATE", nil},
		{"PARTITION.ADD item", "PARTITION.ADD", [][]byte{[]byte("part1"), []byte("item1")}},
		{"PARTITION.ADD no args", "PARTITION.ADD", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestMVCCCommandsLowCoverageBatch5(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMVCCCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"SPATIAL.WITHIN search", "SPATIAL.WITHIN", [][]byte{[]byte("spatial1"), []byte("40.7128"), []byte("-74.0060"), []byte("1000")}},
		{"SPATIAL.WITHIN no args", "SPATIAL.WITHIN", nil},
		{"ROLLUP.ADD data", "ROLLUP.ADD", [][]byte{[]byte("rollup1"), []byte("100")}},
		{"ROLLUP.ADD no args", "ROLLUP.ADD", nil},
		{"ROLLUP.GET data", "ROLLUP.GET", [][]byte{[]byte("rollup1")}},
		{"ROLLUP.GET no args", "ROLLUP.GET", nil},
		{"QUOTA.SET quota", "QUOTA.SET", [][]byte{[]byte("quota1"), []byte("1000")}},
		{"QUOTA.SET no args", "QUOTA.SET", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestReplicationCommandsBatch5(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterReplicationCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"ROLE", "ROLE", nil},
		{"REPLICAOF host port", "REPLICAOF", [][]byte{[]byte("127.0.0.1"), []byte("6379")}},
		{"REPLICAOF no one", "REPLICAOF", [][]byte{[]byte("NO"), []byte("ONE")}},
		{"REPLICAOF no args", "REPLICAOF", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestResilienceCommandsLowCoverageBatch5(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"DIAGNOSTIC.RUN check", "DIAGNOSTIC.RUN", nil},
		{"MEMORYX.FREE memory", "MEMORYX.FREE", nil},
		{"MEMORYX.STATS memory", "MEMORYX.STATS", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestSchedulerCommandsLowCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterSchedulerCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"JOB.CREATE job", "JOB.CREATE", [][]byte{[]byte("job1"), []byte("* * * * *"), []byte("cmd")}},
		{"JOB.CREATE no args", "JOB.CREATE", nil},
		{"JOB.STATS job", "JOB.STATS", [][]byte{[]byte("job1")}},
		{"JOB.STATS no args", "JOB.STATS", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestMLCommandsLowCoverageBatch5(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMLCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"TENSOR.CREATE tensor", "TENSOR.CREATE", [][]byte{[]byte("tensor1"), []byte("[1,2,3]")}},
		{"TENSOR.CREATE no args", "TENSOR.CREATE", nil},
		{"TENSOR.GET tensor", "TENSOR.GET", [][]byte{[]byte("tensor1")}},
		{"TENSOR.DELETE tensor", "TENSOR.DELETE", [][]byte{[]byte("tensor1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestMonitoringCommandsLowCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMonitoringCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"METRICS", "METRICS", nil},
		{"SLOWLOG.CONFIG set", "SLOWLOG.CONFIG", [][]byte{[]byte("100")}},
		{"SLOWLOG.CONFIG no args", "SLOWLOG.CONFIG", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestIntegrationCommandsLowCoverageBatch5(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterIntegrationCommands(router)

	s.Set("arr1", &store.ListValue{Elements: [][]byte{[]byte("1"), []byte("2")}}, store.SetOptions{})
	s.Set("arr2", &store.ListValue{Elements: [][]byte{[]byte("3"), []byte("4")}}, store.SetOptions{})
	s.Set("obj1", &store.HashValue{Fields: map[string][]byte{"a": []byte("1"), "b": []byte("2")}}, store.SetOptions{})

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"CACHE.REFRESH key", "CACHE.REFRESH", [][]byte{[]byte("key1")}},
		{"CACHE.REFRESH no args", "CACHE.REFRESH", nil},
		{"ARRAY.MERGE two arrays", "ARRAY.MERGE", [][]byte{[]byte("arr1"), []byte("arr2")}},
		{"ARRAY.MERGE no args", "ARRAY.MERGE", nil},
		{"OBJECT.MERGE objects", "OBJECT.MERGE", [][]byte{[]byte("obj1"), []byte("obj2")}},
		{"OBJECT.MERGE no args", "OBJECT.MERGE", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}
