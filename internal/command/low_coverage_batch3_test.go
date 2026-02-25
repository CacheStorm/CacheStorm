package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestLowCoverageFunctionsBatch3(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterCacheWarmingCommands(router)
	RegisterClusterCommands(router)
	RegisterDigestCommands(router)
	RegisterEncodingCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"BATCHEXEC no args", "BATCHEXEC", nil},
		{"BATCHEXEC commands", "BATCHEXEC", [][]byte{[]byte("SET key1 value1"), []byte("GET key1")}},
		{"KEYCOPY no args", "KEYCOPY", nil},
		{"KEYCOPY missing args", "KEYCOPY", [][]byte{[]byte("source")}},
		{"KEYCOPY keys", "KEYCOPY", [][]byte{[]byte("source"), []byte("dest")}},
		{"KEYOBJECT no args", "KEYOBJECT", nil},
		{"KEYOBJECT not found", "KEYOBJECT", [][]byte{[]byte("notfound")}},
		{"CLUSTER ADDSLOTS", "CLUSTER", [][]byte{[]byte("ADDSLOTS"), []byte("1")}},
		{"CLUSTER INFO", "CLUSTER", [][]byte{[]byte("INFO")}},
		{"CLUSTER NODES", "CLUSTER", [][]byte{[]byte("NODES")}},
		{"BASE64.DECODE no args", "BASE64.DECODE", nil},
		{"BASE64.DECODE invalid", "BASE64.DECODE", [][]byte{[]byte("invalid!!!")}},
		{"BASE64.DECODE valid", "BASE64.DECODE", [][]byte{[]byte("SGVsbG8=")}},
		{"TOML.ENCODE no args", "TOML.ENCODE", nil},
		{"TOML.ENCODE invalid", "TOML.ENCODE", [][]byte{[]byte("invalid")}},
		{"CBOR.DECODE no args", "CBOR.DECODE", nil},
		{"CBOR.DECODE invalid", "CBOR.DECODE", [][]byte{[]byte("invalid")}},
		{"TIMESTAMP.PARSE no args", "TIMESTAMP.PARSE", nil},
		{"TIMESTAMP.PARSE invalid", "TIMESTAMP.PARSE", [][]byte{[]byte("invalid")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestExtendedCommandsLowCoverageBatch3(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtendedCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"MSGQUEUE.NACK no args", "MSGQUEUE.NACK", nil},
		{"MSGQUEUE.NACK not found", "MSGQUEUE.NACK", [][]byte{[]byte("notfound"), []byte("msg1")}},
		{"HEALTHX.REGISTER no args", "HEALTHX.REGISTER", nil},
		{"HEALTHX.REGISTER service", "HEALTHX.REGISTER", [][]byte{[]byte("service1"), []byte("endpoint")}},
		{"HEALTHX.HISTORY no args", "HEALTHX.HISTORY", nil},
		{"HEALTHX.HISTORY not found", "HEALTHX.HISTORY", [][]byte{[]byte("notfound")}},
		{"CRON.HISTORY no args", "CRON.HISTORY", nil},
		{"CRON.HISTORY not found", "CRON.HISTORY", [][]byte{[]byte("notfound")}},
		{"WS.BROADCAST no args", "WS.BROADCAST", nil},
		{"WS.BROADCAST message", "WS.BROADCAST", [][]byte{[]byte("message")}},
		{"WS.LEAVE no args", "WS.LEAVE", nil},
		{"WS.LEAVE not found", "WS.LEAVE", [][]byte{[]byte("notfound"), []byte("room1")}},
		{"MEMO.CACHE no args", "MEMO.CACHE", nil},
		{"MEMO.CACHE missing args", "MEMO.CACHE", [][]byte{[]byte("key1")}},
		{"SENTINELX.WATCH no args", "SENTINELX.WATCH", nil},
		{"SENTINELX.WATCH master", "SENTINELX.WATCH", [][]byte{[]byte("mymaster")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestExtraCommandsLowCoverageBatch2(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtraCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"GOSSIP.BROADCAST no args", "GOSSIP.BROADCAST", nil},
		{"GOSSIP.BROADCAST message", "GOSSIP.BROADCAST", [][]byte{[]byte("test message")}},
		{"VECTOR_CLOCK.CREATE no args", "VECTOR_CLOCK.CREATE", nil},
		{"VECTOR_CLOCK.CREATE clock", "VECTOR_CLOCK.CREATE", [][]byte{[]byte("clock1")}},
		{"VECTOR_CLOCK.COMPARE no args", "VECTOR_CLOCK.COMPARE", nil},
		{"VECTOR_CLOCK.COMPARE missing args", "VECTOR_CLOCK.COMPARE", [][]byte{[]byte("clock1")}},
		{"RAFT.STATE no args", "RAFT.STATE", nil},
		{"RAFT.STATE state", "RAFT.STATE", [][]byte{[]byte("follower")}},
		{"RAFT.LEADER no args", "RAFT.LEADER", nil},
		{"RAFT.TERM no args", "RAFT.TERM", nil},
		{"SHARD.MAP no args", "SHARD.MAP", nil},
		{"SHARD.MAP key", "SHARD.MAP", [][]byte{[]byte("key1")}},
		{"SHARD.REBALANCE no args", "SHARD.REBALANCE", nil},
		{"GATEWAY.ROUTE no args", "GATEWAY.ROUTE", nil},
		{"GATEWAY.ROUTE missing args", "GATEWAY.ROUTE", [][]byte{[]byte("gw1")}},
		{"ROUTE.ADD no args", "ROUTE.ADD", nil},
		{"ROUTE.ADD missing args", "ROUTE.ADD", [][]byte{[]byte("route1")}},
		{"ROUTE.REMOVE no args", "ROUTE.REMOVE", nil},
		{"ROUTE.REMOVE not found", "ROUTE.REMOVE", [][]byte{[]byte("notfound")}},
		{"ROUTE.MATCH no args", "ROUTE.MATCH", nil},
		{"ROUTE.MATCH missing args", "ROUTE.MATCH", [][]byte{[]byte("route1")}},
		{"ROUTE.LIST no args", "ROUTE.LIST", nil},
		{"PROBE.CREATE no args", "PROBE.CREATE", nil},
		{"PROBE.CREATE probe", "PROBE.CREATE", [][]byte{[]byte("probe1"), []byte("http://localhost:8080")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestFunctionCommandsLowCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterFunctionCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"FCALL no args", "FCALL", nil},
		{"FCALL not found", "FCALL", [][]byte{[]byte("notfound")}},
		{"FUNCTION LIST", "FUNCTION", [][]byte{[]byte("LIST")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestGeoCommandsLowCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterGeoCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"GEORADIUS no args", "GEORADIUS", nil},
		{"GEORADIUS missing args", "GEORADIUS", [][]byte{[]byte("key1"), []byte("0"), []byte("0")}},
		{"GEORADIUSBYMEMBER no args", "GEORADIUSBYMEMBER", nil},
		{"GEORADIUSBYMEMBER missing args", "GEORADIUSBYMEMBER", [][]byte{[]byte("key1"), []byte("member1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestHashCommandsLowCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterHashCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"HGETDEL no args", "HGETDEL", nil},
		{"HGETDEL not found", "HGETDEL", [][]byte{[]byte("notfound"), []byte("field1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestIntegrationCommandsLowCoverageBatch2(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterIntegrationCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"CACHE.REFRESH no args", "CACHE.REFRESH", nil},
		{"CACHE.REFRESH not found", "CACHE.REFRESH", [][]byte{[]byte("notfound")}},
		{"ARRAY.MERGE no args", "ARRAY.MERGE", nil},
		{"ARRAY.MERGE missing args", "ARRAY.MERGE", [][]byte{[]byte("arr1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}
