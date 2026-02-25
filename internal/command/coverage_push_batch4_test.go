package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestCachewarmCommandsMoreCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterCacheWarmingCommands(router)

	s.Set("key1", &store.StringValue{Data: []byte("value1")}, store.SetOptions{})
	s.Set("key2", &store.StringValue{Data: []byte("value2")}, store.SetOptions{})

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"BATCHEXEC multi commands", "BATCHEXEC", [][]byte{[]byte("SET key1 val1"), []byte("GET key1"), []byte("DEL key1")}},
		{"BATCHEXEC empty string", "BATCHEXEC", [][]byte{[]byte("")}},
		{"KEYCOPY both keys exist", "KEYCOPY", [][]byte{[]byte("key1"), []byte("key2")}},
		{"KEYCOPY source not found", "KEYCOPY", [][]byte{[]byte("notfound"), []byte("dest")}},
		{"KEYCOPY dest exists", "KEYCOPY", [][]byte{[]byte("key2"), []byte("key1")}},
		{"KEYOBJECT string key", "KEYOBJECT", [][]byte{[]byte("key1")}},
		{"KEYOBJECT hash key", "KEYOBJECT", [][]byte{[]byte("hash1")}},
		{"KEYOBJECT not found", "KEYOBJECT", [][]byte{[]byte("notfound")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestClusterCommandsBatch4(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterClusterCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"CLUSTER INFO with stats", "CLUSTER", [][]byte{[]byte("INFO")}},
		{"CLUSTER NODES with nodes", "CLUSTER", [][]byte{[]byte("NODES")}},
		{"CLUSTER MEET node", "CLUSTER", [][]byte{[]byte("MEET"), []byte("127.0.0.1"), []byte("6379")}},
		{"CLUSTER FORGET node", "CLUSTER", [][]byte{[]byte("FORGET"), []byte("node1")}},
		{"CLUSTER REPLICATE node", "CLUSTER", [][]byte{[]byte("REPLICATE"), []byte("node1")}},
		{"CLUSTER ADDSLOTS slots", "CLUSTER", [][]byte{[]byte("ADDSLOTS"), []byte("1"), []byte("2"), []byte("3")}},
		{"CLUSTER DELSLOTS slots", "CLUSTER", [][]byte{[]byte("DELSLOTS"), []byte("1")}},
		{"CLUSTER SETSLOT slot", "CLUSTER", [][]byte{[]byte("SETSLOT"), []byte("1"), []byte("IMPORTING"), []byte("node1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestDigestCommandsMoreCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterDigestCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"BASE64.DECODE valid data", "BASE64.DECODE", [][]byte{[]byte("SGVsbG8gV29ybGQ=")}},
		{"BASE64.DECODE empty", "BASE64.DECODE", [][]byte{[]byte("")}},
		{"BASE64.DECODE invalid", "BASE64.DECODE", [][]byte{[]byte("!!!invalid")}},
		{"BASE64.ENCODE data", "BASE64.ENCODE", [][]byte{[]byte("Hello World")}},
		{"CRYPTO.HASH data", "CRYPTO.HASH", [][]byte{[]byte("sha256"), []byte("data")}},
		{"CRYPTO.VERIFY data", "CRYPTO.VERIFY", [][]byte{[]byte("sha256"), []byte("data"), []byte("hash")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestEncodingCommandsBatch4(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterEncodingCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"TOML.ENCODE valid", "TOML.ENCODE", [][]byte{[]byte(`{"key":"value"}`)}},
		{"TOML.ENCODE empty", "TOML.ENCODE", [][]byte{[]byte("")}},
		{"TOML.DECODE valid", "TOML.DECODE", [][]byte{[]byte("key = \"value\"")}},
		{"TOML.DECODE empty", "TOML.DECODE", [][]byte{[]byte("")}},
		{"CBOR.ENCODE valid", "CBOR.ENCODE", [][]byte{[]byte(`{"key":"value"}`)}},
		{"CBOR.DECODE valid", "CBOR.DECODE", [][]byte{[]byte("data")}},
		{"CBOR.DECODE empty", "CBOR.DECODE", [][]byte{[]byte("")}},
		{"MSGPACK.ENCODE valid", "MSGPACK.ENCODE", [][]byte{[]byte(`{"key":"value"}`)}},
		{"MSGPACK.DECODE valid", "MSGPACK.DECODE", [][]byte{[]byte("data")}},
		{"BSON.ENCODE valid", "BSON.ENCODE", [][]byte{[]byte(`{"key":"value"}`)}},
		{"BSON.DECODE valid", "BSON.DECODE", [][]byte{[]byte("data")}},
		{"TIMESTAMP.PARSE valid", "TIMESTAMP.PARSE", [][]byte{[]byte("2024-01-15T10:30:00Z")}},
		{"TIMESTAMP.PARSE invalid", "TIMESTAMP.PARSE", [][]byte{[]byte("invalid")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestExtraCommandsBatch4(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtraCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"VECTOR_CLOCK.COMPARE both exist", "VECTOR_CLOCK.COMPARE", [][]byte{[]byte("clock1"), []byte("clock2")}},
		{"RAFT.STATE set", "RAFT.STATE", [][]byte{[]byte("follower")}},
		{"RAFT.LEADER set", "RAFT.LEADER", [][]byte{[]byte("node1")}},
		{"RAFT.TERM set", "RAFT.TERM", [][]byte{[]byte("1")}},
		{"RAFT.VOTE cast", "RAFT.VOTE", [][]byte{[]byte("node1"), []byte("1")}},
		{"RAFT.APPEND entry", "RAFT.APPEND", [][]byte{[]byte("1"), []byte("command")}},
		{"RAFT.COMMIT index", "RAFT.COMMIT", [][]byte{[]byte("1")}},
		{"SHARD.MAP key hash", "SHARD.MAP", [][]byte{[]byte("key1"), []byte("hash")}},
		{"SHARD.REBALANCE trigger", "SHARD.REBALANCE", nil},
		{"GATEWAY.ROUTE create", "GATEWAY.ROUTE", [][]byte{[]byte("gw1"), []byte("/api"), []byte("svc1")}},
		{"ROUTE.ADD new route", "ROUTE.ADD", [][]byte{[]byte("route1"), []byte("/path"), []byte("svc1")}},
		{"ROUTE.REMOVE existing", "ROUTE.REMOVE", [][]byte{[]byte("route1")}},
		{"ROUTE.MATCH path", "ROUTE.MATCH", [][]byte{[]byte("/api/test")}},
		{"ROUTE.LIST all", "ROUTE.LIST", nil},
		{"PROBE.CREATE health", "PROBE.CREATE", [][]byte{[]byte("probe1"), []byte("http://localhost:8080/health"), []byte("30")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestFunctionCommandsBatch4(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterFunctionCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"FCALL with args", "FCALL", [][]byte{[]byte("func1"), []byte("1"), []byte("arg1")}},
		{"FUNCTION LIST all", "FUNCTION", [][]byte{[]byte("LIST")}},
		{"FUNCTION LIST with pattern", "FUNCTION", [][]byte{[]byte("LIST"), []byte("func*")}},
		{"FUNCTION LOAD lua", "FUNCTION", [][]byte{[]byte("LOAD"), []byte("return redis.call('GET', KEYS[1])")}},
		{"FUNCTION DELETE name", "FUNCTION", [][]byte{[]byte("DELETE"), []byte("func1")}},
		{"FUNCTION FLUSH all", "FUNCTION", [][]byte{[]byte("FLUSH")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestGeoCommandsMoreCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterGeoCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"GEOADD coordinates", "GEOADD", [][]byte{[]byte("geo1"), []byte("-74.0060"), []byte("40.7128"), []byte("NYC")}},
		{"GEODIST members", "GEODIST", [][]byte{[]byte("geo1"), []byte("NYC"), []byte("LA")}},
		{"GEOHASH member", "GEOHASH", [][]byte{[]byte("geo1"), []byte("NYC")}},
		{"GEOPOS members", "GEOPOS", [][]byte{[]byte("geo1"), []byte("NYC")}},
		{"GEORADIUS with options", "GEORADIUS", [][]byte{[]byte("geo1"), []byte("-74.0060"), []byte("40.7128"), []byte("100"), []byte("km"), []byte("WITHDIST")}},
		{"GEORADIUSBYMEMBER with options", "GEORADIUSBYMEMBER", [][]byte{[]byte("geo1"), []byte("NYC"), []byte("100"), []byte("km"), []byte("WITHCOORD")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestMVCCCommandsMoreCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMVCCCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"SPATIAL.CREATE index", "SPATIAL.CREATE", [][]byte{[]byte("spatial1")}},
		{"SPATIAL.ADD point", "SPATIAL.ADD", [][]byte{[]byte("spatial1"), []byte("p1"), []byte("40.7128"), []byte("-74.0060")}},
		{"SPATIAL.WITHIN search", "SPATIAL.WITHIN", [][]byte{[]byte("spatial1"), []byte("40.7128"), []byte("-74.0060"), []byte("1000")}},
		{"SPATIAL.NEARBY search", "SPATIAL.NEARBY", [][]byte{[]byte("spatial1"), []byte("40.7128"), []byte("-74.0060"), []byte("1000")}},
		{"ROLLUP.CREATE index", "ROLLUP.CREATE", [][]byte{[]byte("rollup1")}},
		{"ROLLUP.ADD data", "ROLLUP.ADD", [][]byte{[]byte("rollup1"), []byte("100")}},
		{"ROLLUP.GET data", "ROLLUP.GET", [][]byte{[]byte("rollup1")}},
		{"ANALYTICS.INCR metric", "ANALYTICS.INCR", [][]byte{[]byte("metric1"), []byte("100"), []byte("value1")}},
		{"ANALYTICS.DECR metric", "ANALYTICS.DECR", [][]byte{[]byte("metric1"), []byte("100"), []byte("value1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}
