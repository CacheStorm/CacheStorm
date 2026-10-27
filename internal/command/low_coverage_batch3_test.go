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
		{"VECTOR_CLOCK.CREATE no args", "VECTOR_CLOCK.CREATE", nil},
		{"VECTOR_CLOCK.CREATE clock", "VECTOR_CLOCK.CREATE", [][]byte{[]byte("clock1")}},
		{"VECTOR_CLOCK.COMPARE no args", "VECTOR_CLOCK.COMPARE", nil},
		{"VECTOR_CLOCK.COMPARE missing args", "VECTOR_CLOCK.COMPARE", [][]byte{[]byte("clock1")}},
		{"GATEWAY.ROUTE no args", "GATEWAY.ROUTE", nil},
		{"GATEWAY.ROUTE missing args", "GATEWAY.ROUTE", [][]byte{[]byte("gw1")}},
		{"ROUTE.ADD no args", "ROUTE.ADD", nil},
		{"ROUTE.ADD missing args", "ROUTE.ADD", [][]byte{[]byte("route1")}},
		{"ROUTE.REMOVE no args", "ROUTE.REMOVE", nil},
		{"ROUTE.REMOVE not found", "ROUTE.REMOVE", [][]byte{[]byte("notfound")}},
		{"ROUTE.MATCH no args", "ROUTE.MATCH", nil},
		{"ROUTE.MATCH missing args", "ROUTE.MATCH", [][]byte{[]byte("route1")}},
		{"ROUTE.LIST no args", "ROUTE.LIST", nil},
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
