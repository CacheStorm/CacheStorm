package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestCachewarmCommandsBatch7(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterCacheWarmingCommands(router)

	s.Set("key1", &store.StringValue{Data: []byte("value1")}, store.SetOptions{})
	s.Set("hash1", &store.HashValue{Fields: map[string][]byte{"field1": []byte("value1")}}, store.SetOptions{})
	s.Set("list1", &store.ListValue{Elements: [][]byte{[]byte("item1")}}, store.SetOptions{})

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"BATCHEXEC multiple", "BATCHEXEC", [][]byte{[]byte("SET key1 val1"), []byte("GET key1"), []byte("SET key2 val2")}},
		{"BATCHEXEC empty", "BATCHEXEC", [][]byte{[]byte("")}},
		{"KEYOBJECT string", "KEYOBJECT", [][]byte{[]byte("key1")}},
		{"KEYOBJECT hash", "KEYOBJECT", [][]byte{[]byte("hash1")}},
		{"KEYOBJECT list", "KEYOBJECT", [][]byte{[]byte("list1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestClusterCommandsBatch7(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterClusterCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"CLUSTER INFO detailed", "CLUSTER", [][]byte{[]byte("INFO")}},
		{"CLUSTER NODES detailed", "CLUSTER", [][]byte{[]byte("NODES")}},
		{"CLUSTER SLOTS", "CLUSTER", [][]byte{[]byte("SLOTS")}},
		{"CLUSTER KEYSLOT key", "CLUSTER", [][]byte{[]byte("KEYSLOT"), []byte("mykey")}},
		{"CLUSTER COUNTKEYSINSLOT slot", "CLUSTER", [][]byte{[]byte("COUNTKEYSINSLOT"), []byte("0")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestDigestCommandsBatch7(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterDigestCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"BASE64.DECODE empty", "BASE64.DECODE", [][]byte{[]byte("")}},
		{"BASE64.DECODE valid", "BASE64.DECODE", [][]byte{[]byte("SGVsbG8=")}},
		{"BASE64.DECODE invalid", "BASE64.DECODE", [][]byte{[]byte("!!!")}},
		{"CRYPTO.HASH sha256", "CRYPTO.HASH", [][]byte{[]byte("sha256"), []byte("test")}},
		{"CRYPTO.HASH md5", "CRYPTO.HASH", [][]byte{[]byte("md5"), []byte("test")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestEncodingCommandsBatch7(t *testing.T) {
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
		{"CBOR.ENCODE valid", "CBOR.ENCODE", [][]byte{[]byte(`{"key":"value"}`)}},
		{"CBOR.DECODE empty", "CBOR.DECODE", [][]byte{[]byte("")}},
		{"MSGPACK.ENCODE valid", "MSGPACK.ENCODE", [][]byte{[]byte(`{"key":"value"}`)}},
		{"BSON.ENCODE valid", "BSON.ENCODE", [][]byte{[]byte(`{"key":"value"}`)}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestExtraCommandsBatch7(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtraCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"VECTOR_CLOCK.COMPARE two", "VECTOR_CLOCK.COMPARE", [][]byte{[]byte("c1"), []byte("c2")}},
		{"RAFT.STATE set", "RAFT.STATE", [][]byte{[]byte("follower")}},
		{"RAFT.LEADER set", "RAFT.LEADER", [][]byte{[]byte("node1")}},
		{"SHARD.REBALANCE trigger", "SHARD.REBALANCE", nil},
		{"ROUTE.REMOVE existing", "ROUTE.REMOVE", [][]byte{[]byte("route1")}},
		{"ROUTE.MATCH path", "ROUTE.MATCH", [][]byte{[]byte("/api/test")}},
		{"ROUTE.LIST all", "ROUTE.LIST", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestFunctionCommandsBatch7(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterFunctionCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"FCALL function", "FCALL", [][]byte{[]byte("myfunc"), []byte("0")}},
		{"FCALL with keys", "FCALL", [][]byte{[]byte("myfunc"), []byte("1"), []byte("key1")}},
		{"FUNCTION LIST all", "FUNCTION", [][]byte{[]byte("LIST")}},
		{"FUNCTION LOAD code", "FUNCTION", [][]byte{[]byte("LOAD"), []byte("return 1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestIntegrationCommandsBatch7(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterIntegrationCommands(router)

	s.Set("arr1", &store.ListValue{Elements: [][]byte{[]byte("1"), []byte("2")}}, store.SetOptions{})
	s.Set("arr2", &store.ListValue{Elements: [][]byte{[]byte("3"), []byte("4")}}, store.SetOptions{})
	s.Set("obj1", &store.HashValue{Fields: map[string][]byte{"a": []byte("1"), "b": []byte("2")}}, store.SetOptions{})
	s.Set("obj2", &store.HashValue{Fields: map[string][]byte{"c": []byte("3")}}, store.SetOptions{})

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"CACHE.REFRESH key", "CACHE.REFRESH", [][]byte{[]byte("key1")}},
		{"ARRAY.MERGE two", "ARRAY.MERGE", [][]byte{[]byte("arr1"), []byte("arr2")}},
		{"OBJECT.MERGE two", "OBJECT.MERGE", [][]byte{[]byte("obj1"), []byte("obj2")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestMLCommandsBatch7(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMLCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"TENSOR.CREATE array", "TENSOR.CREATE", [][]byte{[]byte("t1"), []byte("[1,2,3]")}},
		{"TENSOR.CREATE matrix", "TENSOR.CREATE", [][]byte{[]byte("t2"), []byte("[[1,2],[3,4]]")}},
		{"TENSOR.CREATE no args", "TENSOR.CREATE", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestMoreCommandsBatch7(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"TRACESPAN start", "TRACESPAN", [][]byte{[]byte("start"), []byte("t1")}},
		{"TRACESPAN end", "TRACESPAN", [][]byte{[]byte("end"), []byte("t1")}},
		{"QUOTAX.CREATE quota", "QUOTAX.CREATE", [][]byte{[]byte("q1"), []byte("1000")}},
		{"METER.CREATE meter", "METER.CREATE", [][]byte{[]byte("m1")}},
		{"TENANT.CREATE tenant", "TENANT.CREATE", [][]byte{[]byte("t1")}},
		{"LEASE.CREATE lease", "LEASE.CREATE", [][]byte{[]byte("l1"), []byte("60")}},
		{"LEASE.RENEW lease", "LEASE.RENEW", [][]byte{[]byte("l1")}},
		{"SKETCH.CREATE sketch", "SKETCH.CREATE", [][]byte{[]byte("s1")}},
		{"SKETCH.UPDATE item", "SKETCH.UPDATE", [][]byte{[]byte("s1"), []byte("item1")}},
		{"SKETCH.MERGE two", "SKETCH.MERGE", [][]byte{[]byte("s1"), []byte("s2")}},
		{"PARTITION.ADD item", "PARTITION.ADD", [][]byte{[]byte("p1"), []byte("item1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestMVCCCommandsBatch7(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMVCCCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"SPATIAL.WITHIN search", "SPATIAL.WITHIN", [][]byte{[]byte("s1"), []byte("40.7"), []byte("-74.0"), []byte("1000")}},
		{"ROLLUP.ADD data", "ROLLUP.ADD", [][]byte{[]byte("r1"), []byte("100")}},
		{"ROLLUP.GET data", "ROLLUP.GET", [][]byte{[]byte("r1")}},
		{"QUOTA.SET quota", "QUOTA.SET", [][]byte{[]byte("q1"), []byte("1000")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestNamespaceCommandsBatch7(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterNamespaceCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"NAMESPACE.CREATE ns", "NAMESPACE.CREATE", [][]byte{[]byte("ns1")}},
		{"NAMESPACE.INFO ns", "NAMESPACE.INFO", [][]byte{[]byte("ns1")}},
		{"NAMESPACE.DELETE ns", "NAMESPACE.DELETE", [][]byte{[]byte("ns1")}},
		{"NAMESPACE.LIST all", "NAMESPACE.LIST", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestResilienceCommandsBatch7(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"OBSERVABILITY.METRIC name", "OBSERVABILITY.METRIC", [][]byte{[]byte("metric1")}},
		{"DIAGNOSTIC.RUN check", "DIAGNOSTIC.RUN", nil},
		{"MEMORYX.FREE memory", "MEMORYX.FREE", nil},
		{"MEMORYX.STATS stats", "MEMORYX.STATS", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestSchedulerCommandsBatch7(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterSchedulerCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"JOB.CREATE job", "JOB.CREATE", [][]byte{[]byte("j1"), []byte("* * * * *"), []byte("echo test")}},
		{"JOB.STATS job", "JOB.STATS", [][]byte{[]byte("j1")}},
		{"CIRCUIT.CREATE circuit", "CIRCUIT.CREATE", [][]byte{[]byte("c1")}},
		{"CIRCUIT.STATS circuit", "CIRCUIT.STATS", [][]byte{[]byte("c1")}},
		{"SESSION.REFRESH session", "SESSION.REFRESH", [][]byte{[]byte("s1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestMonitoringCommandsBatch7(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMonitoringCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"METRICS all", "METRICS", nil},
		{"SLOWLOG.CONFIG set", "SLOWLOG.CONFIG", [][]byte{[]byte("100")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}
