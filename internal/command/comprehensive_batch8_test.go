package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestComprehensiveLowCoverageBatch8(t *testing.T) {
	s := store.NewStore()

	s.Set("str1", &store.StringValue{Data: []byte("value1")}, store.SetOptions{})
	s.Set("hash1", &store.HashValue{Fields: map[string][]byte{"f1": []byte("v1")}}, store.SetOptions{})
	s.Set("list1", &store.ListValue{Elements: [][]byte{[]byte("a"), []byte("b")}}, store.SetOptions{})
	s.Set("set1", &store.SetValue{Members: map[string]struct{}{"m1": {}}}, store.SetOptions{})
	s.Set("zset1", &store.SortedSetValue{Members: map[string]float64{"m1": 1.0}}, store.SetOptions{})

	router := NewRouter()
	RegisterStringCommands(router)
	RegisterHashCommands(router)
	RegisterListCommands(router)
	RegisterSetCommands(router)
	RegisterSortedSetCommands(router)
	RegisterCacheWarmingCommands(router)
	RegisterClusterCommands(router)
	RegisterDigestCommands(router)
	RegisterEncodingCommands(router)
	RegisterExtraCommands(router)
	RegisterFunctionCommands(router)
	RegisterIntegrationCommands(router)
	RegisterMLCommands(router)
	RegisterMoreCommands(router)
	RegisterMVCCCommands(router)
	RegisterResilienceCommands(router)
	RegisterSchedulerCommands(router)
	RegisterMonitoringCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"BATCHEXEC commands", "BATCHEXEC", [][]byte{[]byte("SET k1 v1"), []byte("GET k1"), []byte("DEL k1")}},
		{"KEYOBJECT string", "KEYOBJECT", [][]byte{[]byte("str1")}},
		{"KEYOBJECT hash", "KEYOBJECT", [][]byte{[]byte("hash1")}},
		{"KEYOBJECT list", "KEYOBJECT", [][]byte{[]byte("list1")}},
		{"KEYOBJECT set", "KEYOBJECT", [][]byte{[]byte("set1")}},
		{"KEYOBJECT zset", "KEYOBJECT", [][]byte{[]byte("zset1")}},
		{"CLUSTER INFO", "CLUSTER", [][]byte{[]byte("INFO")}},
		{"CLUSTER NODES", "CLUSTER", [][]byte{[]byte("NODES")}},
		{"BASE64.DECODE valid", "BASE64.DECODE", [][]byte{[]byte("SGVsbG8=")}},
		{"TOML.ENCODE json", "TOML.ENCODE", [][]byte{[]byte(`{"key":"value"}`)}},
		{"CBOR.DECODE empty", "CBOR.DECODE", [][]byte{[]byte("")}},
		{"VECTOR_CLOCK.COMPARE two", "VECTOR_CLOCK.COMPARE", [][]byte{[]byte("c1"), []byte("c2")}},
		{"RAFT.STATE follower", "RAFT.STATE", [][]byte{[]byte("follower")}},
		{"RAFT.LEADER node", "RAFT.LEADER", [][]byte{[]byte("node1")}},
		{"SHARD.REBALANCE", "SHARD.REBALANCE", nil},
		{"ROUTE.REMOVE r1", "ROUTE.REMOVE", [][]byte{[]byte("r1")}},
		{"ROUTE.MATCH path", "ROUTE.MATCH", [][]byte{[]byte("/api")}},
		{"ROUTE.LIST", "ROUTE.LIST", nil},
		{"FCALL func", "FCALL", [][]byte{[]byte("func1"), []byte("0")}},
		{"CACHE.REFRESH key", "CACHE.REFRESH", [][]byte{[]byte("k1")}},
		{"ARRAY.MERGE two", "ARRAY.MERGE", [][]byte{[]byte("list1"), []byte("list1")}},
		{"OBJECT.MERGE two", "OBJECT.MERGE", [][]byte{[]byte("hash1"), []byte("hash1")}},
		{"TENSOR.CREATE arr", "TENSOR.CREATE", [][]byte{[]byte("t1"), []byte("[1,2,3]")}},
		{"QUOTAX.CREATE q", "QUOTAX.CREATE", [][]byte{[]byte("q1"), []byte("100")}},
		{"METER.CREATE m", "METER.CREATE", [][]byte{[]byte("m1")}},
		{"TENANT.CREATE t", "TENANT.CREATE", [][]byte{[]byte("t1")}},
		{"LEASE.CREATE l", "LEASE.CREATE", [][]byte{[]byte("l1"), []byte("60")}},
		{"LEASE.RENEW l", "LEASE.RENEW", [][]byte{[]byte("l1")}},
		{"SKETCH.CREATE s", "SKETCH.CREATE", [][]byte{[]byte("s1")}},
		{"SKETCH.UPDATE i", "SKETCH.UPDATE", [][]byte{[]byte("s1"), []byte("i1")}},
		{"SKETCH.MERGE two", "SKETCH.MERGE", [][]byte{[]byte("s1"), []byte("s2")}},
		{"PARTITION.ADD i", "PARTITION.ADD", [][]byte{[]byte("p1"), []byte("i1")}},
		{"SPATIAL.WITHIN search", "SPATIAL.WITHIN", [][]byte{[]byte("s1"), []byte("40.7"), []byte("-74.0"), []byte("100")}},
		{"ROLLUP.ADD data", "ROLLUP.ADD", [][]byte{[]byte("r1"), []byte("100")}},
		{"ROLLUP.GET data", "ROLLUP.GET", [][]byte{[]byte("r1")}},
		{"QUOTA.SET q", "QUOTA.SET", [][]byte{[]byte("q1"), []byte("100")}},
		{"OBSERVABILITY.METRIC m", "OBSERVABILITY.METRIC", [][]byte{[]byte("m1")}},
		{"DIAGNOSTIC.RUN", "DIAGNOSTIC.RUN", nil},
		{"MEMORYX.FREE", "MEMORYX.FREE", nil},
		{"MEMORYX.STATS", "MEMORYX.STATS", nil},
		{"JOB.CREATE j", "JOB.CREATE", [][]byte{[]byte("j1"), []byte("* * * * *"), []byte("cmd")}},
		{"JOB.STATS j", "JOB.STATS", [][]byte{[]byte("j1")}},
		{"CIRCUIT.CREATE c", "CIRCUIT.CREATE", [][]byte{[]byte("c1")}},
		{"CIRCUIT.STATS c", "CIRCUIT.STATS", [][]byte{[]byte("c1")}},
		{"SESSION.REFRESH s", "SESSION.REFRESH", [][]byte{[]byte("s1")}},
		{"SLOWLOG.CONFIG n", "SLOWLOG.CONFIG", [][]byte{[]byte("100")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestAllCommandsBatch8(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()

	RegisterStringCommands(router)
	RegisterHashCommands(router)
	RegisterListCommands(router)
	RegisterSetCommands(router)
	RegisterSortedSetCommands(router)
	RegisterBitmapCommands(router)
	RegisterHyperLogLogCommands(router)
	RegisterGeoCommands(router)
	RegisterStreamCommands(router)
	RegisterJSONCommands(router)
	RegisterPubSubCommands(router)
	RegisterTransactionCommands(router)

	s.Set("key1", &store.StringValue{Data: []byte("value1")}, store.SetOptions{})
	s.Set("hash1", &store.HashValue{Fields: map[string][]byte{"field1": []byte("value1")}}, store.SetOptions{})
	s.Set("list1", &store.ListValue{Elements: [][]byte{[]byte("a"), []byte("b"), []byte("c")}}, store.SetOptions{})
	s.Set("set1", &store.SetValue{Members: map[string]struct{}{"member1": {}, "member2": {}}}, store.SetOptions{})
	s.Set("zset1", &store.SortedSetValue{Members: map[string]float64{"m1": 1.0, "m2": 2.0}}, store.SetOptions{})

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"GET key", "GET", [][]byte{[]byte("key1")}},
		{"SET key value", "SET", [][]byte{[]byte("key2"), []byte("value2")}},
		{"DEL key", "DEL", [][]byte{[]byte("key1")}},
		{"EXISTS key", "EXISTS", [][]byte{[]byte("key1")}},
		{"INCR key", "INCR", [][]byte{[]byte("counter")}},
		{"DECR key", "DECR", [][]byte{[]byte("counter")}},
		{"APPEND key value", "APPEND", [][]byte{[]byte("key1"), []byte("suffix")}},
		{"STRLEN key", "STRLEN", [][]byte{[]byte("key1")}},
		{"HGET key field", "HGET", [][]byte{[]byte("hash1"), []byte("field1")}},
		{"HSET key field value", "HSET", [][]byte{[]byte("hash1"), []byte("field2"), []byte("value2")}},
		{"HDEL key field", "HDEL", [][]byte{[]byte("hash1"), []byte("field1")}},
		{"HKEYS key", "HKEYS", [][]byte{[]byte("hash1")}},
		{"HVALS key", "HVALS", [][]byte{[]byte("hash1")}},
		{"HLEN key", "HLEN", [][]byte{[]byte("hash1")}},
		{"LPUSH key value", "LPUSH", [][]byte{[]byte("list1"), []byte("new")}},
		{"RPUSH key value", "RPUSH", [][]byte{[]byte("list1"), []byte("new")}},
		{"LPOP key", "LPOP", [][]byte{[]byte("list1")}},
		{"RPOP key", "RPOP", [][]byte{[]byte("list1")}},
		{"LLEN key", "LLEN", [][]byte{[]byte("list1")}},
		{"LRANGE key 0 -1", "LRANGE", [][]byte{[]byte("list1"), []byte("0"), []byte("-1")}},
		{"SADD key member", "SADD", [][]byte{[]byte("set1"), []byte("member3")}},
		{"SREM key member", "SREM", [][]byte{[]byte("set1"), []byte("member1")}},
		{"SMEMBERS key", "SMEMBERS", [][]byte{[]byte("set1")}},
		{"SCARD key", "SCARD", [][]byte{[]byte("set1")}},
		{"ZADD key score member", "ZADD", [][]byte{[]byte("zset1"), []byte("3.0"), []byte("m3")}},
		{"ZREM key member", "ZREM", [][]byte{[]byte("zset1"), []byte("m1")}},
		{"ZRANGE key 0 -1", "ZRANGE", [][]byte{[]byte("zset1"), []byte("0"), []byte("-1")}},
		{"ZCARD key", "ZCARD", [][]byte{[]byte("zset1")}},
		{"ZSCORE key member", "ZSCORE", [][]byte{[]byte("zset1"), []byte("m1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}
