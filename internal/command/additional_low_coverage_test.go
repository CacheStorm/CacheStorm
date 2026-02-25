package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestConfigCommandsLowCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterConfigCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"CONFIG.SET no args", "CONFIG.SET", nil},
		{"CONFIG.SET missing args", "CONFIG.SET", [][]byte{[]byte("parameter")}},
		{"CONFIG.SET valid", "CONFIG.SET", [][]byte{[]byte("maxclients"), []byte("1000")}},
		{"CONFIG.SET appendonly", "CONFIG.SET", [][]byte{[]byte("appendonly"), []byte("yes")}},
		{"CONFIG.SET invalid", "CONFIG.SET", [][]byte{[]byte("invalid"), []byte("value")}},
		{"CONFIG.GET no args", "CONFIG.GET", nil},
		{"CONFIG.GET pattern", "CONFIG.GET", [][]byte{[]byte("*")}},
		{"CONFIG.REWRITE", "CONFIG.REWRITE", nil},
		{"CONFIG.RESETSTAT", "CONFIG.RESETSTAT", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestReplicationCommandsLowCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterReplicationCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"REPLCONF no args", "REPLCONF", nil},
		{"REPLCONF listening-port", "REPLCONF", [][]byte{[]byte("listening-port"), []byte("6379")}},
		{"REPLCONF ip-address", "REPLCONF", [][]byte{[]byte("ip-address"), []byte("127.0.0.1")}},
		{"REPLCONF capa", "REPLCONF", [][]byte{[]byte("capa"), []byte("psync2")}},
		{"REPLCONF ack", "REPLCONF", [][]byte{[]byte("ack"), []byte("100")}},
		{"REPLCONF getack", "REPLCONF", [][]byte{[]byte("getack"), []byte("*")}},
		{"PSYNC no args", "PSYNC", nil},
		{"PSYNC replid", "PSYNC", [][]byte{[]byte("?"), []byte("-1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestUtilityCommandsLowCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterUtilityCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"ID.CREATE no args", "ID.CREATE", nil},
		{"ID.CREATE count", "ID.CREATE", [][]byte{[]byte("5")}},
		{"ID.VERIFY no args", "ID.VERIFY", nil},
		{"ID.VERIFY id", "ID.VERIFY", [][]byte{[]byte("abc123")}},
		{"ID.PARSE no args", "ID.PARSE", nil},
		{"ID.PARSE id", "ID.PARSE", [][]byte{[]byte("abc123")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestClusterCommandsLowCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterClusterCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"CLUSTER.INFO no args", "CLUSTER.INFO", nil},
		{"CLUSTER.NODES", "CLUSTER.NODES", nil},
		{"CLUSTER.SLOTS", "CLUSTER.SLOTS", nil},
		{"CLUSTER.MEET no args", "CLUSTER.MEET", nil},
		{"CLUSTER.MEET node", "CLUSTER.MEET", [][]byte{[]byte("127.0.0.1"), []byte("6379")}},
		{"CLUSTER.FORGET no args", "CLUSTER.FORGET", nil},
		{"CLUSTER.FORGET node", "CLUSTER.FORGET", [][]byte{[]byte("node123")}},
		{"CLUSTER.REPLICATE no args", "CLUSTER.REPLICATE", nil},
		{"CLUSTER.REPLICATE node", "CLUSTER.REPLICATE", [][]byte{[]byte("node123")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestCacheWarmCommandsLowCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterCacheWarmingCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"KEYOBJECT no args", "KEYOBJECT", nil},
		{"KEYOBJECT not found", "KEYOBJECT", [][]byte{[]byte("notfound")}},
		{"KEYOBJECT string", "KEYOBJECT", [][]byte{[]byte("key1")}},
		{"WARMUP no args", "WARMUP", nil},
		{"WARMUP pattern", "WARMUP", [][]byte{[]byte("*")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestStatsCommandsLowCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStatsCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"HISTOGRAM.CREATE no args", "HISTOGRAM.CREATE", nil},
		{"HISTOGRAM.CREATE histogram", "HISTOGRAM.CREATE", [][]byte{[]byte("hist1")}},
		{"HISTOGRAM.ADD no args", "HISTOGRAM.ADD", nil},
		{"HISTOGRAM.ADD not found", "HISTOGRAM.ADD", [][]byte{[]byte("notfound"), []byte("100")}},
		{"HISTOGRAM.GET no args", "HISTOGRAM.GET", nil},
		{"HISTOGRAM.GET not found", "HISTOGRAM.GET", [][]byte{[]byte("notfound")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestDSCommandsLowCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterDataStructuresCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"SLIDINGWINDOW.INCR no args", "SLIDINGWINDOW.INCR", nil},
		{"SLIDINGWINDOW.INCR not found", "SLIDINGWINDOW.INCR", [][]byte{[]byte("notfound"), []byte("1")}},
		{"SLIDINGWINDOW.DECR no args", "SLIDINGWINDOW.DECR", nil},
		{"SLIDINGWINDOW.DECR not found", "SLIDINGWINDOW.DECR", [][]byte{[]byte("notfound"), []byte("1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestResilienceCommandsLowCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"BACKPRESSURE.CHECK no args", "BACKPRESSURE.CHECK", nil},
		{"BACKPRESSURE.CHECK create", "BACKPRESSURE.CHECK", [][]byte{[]byte("bp1"), []byte("100")}},
		{"CIRCUITBREAKER.CREATE no args", "CIRCUITBREAKER.CREATE", nil},
		{"CIRCUITBREAKER.CREATE cb", "CIRCUITBREAKER.CREATE", [][]byte{[]byte("cb1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestGraphCommandsLowCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterGraphCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"GRAPH.ADDEDGE no args", "GRAPH.ADDEDGE", nil},
		{"GRAPH.ADDEDGE missing args", "GRAPH.ADDEDGE", [][]byte{[]byte("graph1"), []byte("node1")}},
		{"GRAPH.ADDEDGE edge", "GRAPH.ADDEDGE", [][]byte{[]byte("graph1"), []byte("node1"), []byte("node2"), []byte("edge1")}},
		{"GRAPH.CREATE no args", "GRAPH.CREATE", nil},
		{"GRAPH.CREATE graph", "GRAPH.CREATE", [][]byte{[]byte("graph1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestMLCommandsLowCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMLCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"TENSOR.CREATE no args", "TENSOR.CREATE", nil},
		{"TENSOR.CREATE tensor", "TENSOR.CREATE", [][]byte{[]byte("tensor1"), []byte("[1,2,3]")}},
		{"TENSOR.GET no args", "TENSOR.GET", nil},
		{"TENSOR.GET not found", "TENSOR.GET", [][]byte{[]byte("notfound")}},
		{"TENSOR.DELETE no args", "TENSOR.DELETE", nil},
		{"TENSOR.DELETE not found", "TENSOR.DELETE", [][]byte{[]byte("notfound")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}
