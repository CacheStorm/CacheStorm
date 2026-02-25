package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestRemainingLowCoverageFunctions(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterCacheWarmingCommands(router)
	RegisterClusterCommands(router)
	RegisterDigestCommands(router)
	RegisterEncodingCommands(router)
	RegisterExtendedCommands(router)
	RegisterExtraCommands(router)
	RegisterFunctionCommands(router)
	RegisterGeoCommands(router)
	RegisterIntegrationCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"BATCHEXEC empty", "BATCHEXEC", [][]byte{[]byte("")}},
		{"KEYOBJECT exists", "KEYOBJECT", [][]byte{[]byte("key1")}},
		{"CLUSTER INFO full", "CLUSTER", [][]byte{[]byte("INFO")}},
		{"CLUSTER NODES full", "CLUSTER", [][]byte{[]byte("NODES")}},
		{"BASE64.DECODE empty", "BASE64.DECODE", [][]byte{[]byte("")}},
		{"TOML.ENCODE empty", "TOML.ENCODE", [][]byte{[]byte("")}},
		{"CBOR.DECODE empty", "CBOR.DECODE", [][]byte{[]byte("")}},
		{"HEALTHX.REGISTER full", "HEALTHX.REGISTER", [][]byte{[]byte("svc1"), []byte("http://localhost:8080"), []byte("30")}},
		{"WS.BROADCAST full", "WS.BROADCAST", [][]byte{[]byte("room1"), []byte("Hello")}},
		{"MEMO.CACHE full", "MEMO.CACHE", [][]byte{[]byte("key1"), []byte("value1"), []byte("60")}},
		{"SENTINELX.WATCH full", "SENTINELX.WATCH", [][]byte{[]byte("mymaster"), []byte("127.0.0.1"), []byte("6379")}},
		{"GOSSIP.BROADCAST full", "GOSSIP.BROADCAST", [][]byte{[]byte("channel1"), []byte("message")}},
		{"VECTOR_CLOCK.CREATE full", "VECTOR_CLOCK.CREATE", [][]byte{[]byte("clock1"), []byte("node1")}},
		{"VECTOR_CLOCK.COMPARE full", "VECTOR_CLOCK.COMPARE", [][]byte{[]byte("clock1"), []byte("clock2")}},
		{"RAFT.STATE full", "RAFT.STATE", [][]byte{[]byte("leader")}},
		{"RAFT.LEADER full", "RAFT.LEADER", [][]byte{[]byte("node1")}},
		{"RAFT.TERM full", "RAFT.TERM", [][]byte{[]byte("1")}},
		{"SHARD.MAP full", "SHARD.MAP", [][]byte{[]byte("key1"), []byte("hash")}},
		{"SHARD.REBALANCE full", "SHARD.REBALANCE", nil},
		{"GATEWAY.ROUTE full", "GATEWAY.ROUTE", [][]byte{[]byte("gw1"), []byte("/api"), []byte("svc1")}},
		{"ROUTE.ADD full", "ROUTE.ADD", [][]byte{[]byte("route1"), []byte("/path"), []byte("svc1")}},
		{"ROUTE.REMOVE full", "ROUTE.REMOVE", [][]byte{[]byte("route1")}},
		{"ROUTE.MATCH full", "ROUTE.MATCH", [][]byte{[]byte("/path")}},
		{"ROUTE.LIST full", "ROUTE.LIST", nil},
		{"PROBE.CREATE full", "PROBE.CREATE", [][]byte{[]byte("probe1"), []byte("http://localhost:8080"), []byte("30")}},
		{"FCALL full", "FCALL", [][]byte{[]byte("func1"), []byte("0")}},
		{"GEORADIUSBYMEMBER full", "GEORADIUSBYMEMBER", [][]byte{[]byte("key1"), []byte("member1"), []byte("100")}},
		{"CACHE.REFRESH full", "CACHE.REFRESH", [][]byte{[]byte("key1")}},
		{"ARRAY.MERGE full", "ARRAY.MERGE", [][]byte{[]byte("arr1"), []byte("arr2")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestAdvancedCommandsMoreCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterActorCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"ACTOR.CREATE full", "ACTOR.CREATE", [][]byte{[]byte("actor1"), []byte("type1")}},
		{"ACTOR.DELETE full", "ACTOR.DELETE", [][]byte{[]byte("actor1")}},
		{"ACTOR.SEND full", "ACTOR.SEND", [][]byte{[]byte("actor1"), []byte("message")}},
		{"ACTOR.RECV full", "ACTOR.RECV", [][]byte{[]byte("actor1")}},
		{"ACTOR.POKE full", "ACTOR.POKE", [][]byte{[]byte("actor1")}},
		{"ACTOR.LEN full", "ACTOR.LEN", [][]byte{[]byte("actor1")}},
		{"ACTOR.CLEAR full", "ACTOR.CLEAR", [][]byte{[]byte("actor1")}},
		{"DAG.CREATE full", "DAG.CREATE", [][]byte{[]byte("dag1")}},
		{"DAG.ADDNODE full", "DAG.ADDNODE", [][]byte{[]byte("dag1"), []byte("node1"), []byte("data")}},
		{"DAG.ADDEDGE full", "DAG.ADDEDGE", [][]byte{[]byte("dag1"), []byte("node1"), []byte("node2")}},
		{"DAG.TOPO full", "DAG.TOPO", [][]byte{[]byte("dag1")}},
		{"DAG.PARENTS full", "DAG.PARENTS", [][]byte{[]byte("dag1"), []byte("node1")}},
		{"DAG.CHILDREN full", "DAG.CHILDREN", [][]byte{[]byte("dag1"), []byte("node1")}},
		{"DAG.DELETE full", "DAG.DELETE", [][]byte{[]byte("dag1")}},
		{"PARALLEL.EXEC full", "PARALLEL.EXEC", [][]byte{[]byte("cmd1"), []byte("cmd2")}},
		{"PARALLEL.MAP full", "PARALLEL.MAP", [][]byte{[]byte("list1"), []byte("cmd")}},
		{"PARALLEL.REDUCE full", "PARALLEL.REDUCE", [][]byte{[]byte("list1"), []byte("cmd")}},
		{"PARALLEL.FILTER full", "PARALLEL.FILTER", [][]byte{[]byte("list1"), []byte("cmd")}},
		{"RING.ADD full", "RING.ADD", [][]byte{[]byte("ring1"), []byte("node1")}},
		{"RING.GET full", "RING.GET", [][]byte{[]byte("ring1"), []byte("key1")}},
		{"RING.NODES full", "RING.NODES", [][]byte{[]byte("ring1")}},
		{"RING.REMOVE full", "RING.REMOVE", [][]byte{[]byte("ring1"), []byte("node1")}},
		{"SEM.ACQUIRE full", "SEM.ACQUIRE", [][]byte{[]byte("sem1")}},
		{"SEM.RELEASE full", "SEM.RELEASE", [][]byte{[]byte("sem1")}},
		{"SEM.TRYACQUIRE full", "SEM.TRYACQUIRE", [][]byte{[]byte("sem1")}},
		{"SEM.VALUE full", "SEM.VALUE", [][]byte{[]byte("sem1")}},
		{"TRIE.ADD full", "TRIE.ADD", [][]byte{[]byte("trie1"), []byte("word")}},
		{"TRIE.SEARCH full", "TRIE.SEARCH", [][]byte{[]byte("trie1"), []byte("word")}},
		{"TRIE.PREFIX full", "TRIE.PREFIX", [][]byte{[]byte("trie1"), []byte("pre")}},
		{"TRIE.DELETE full", "TRIE.DELETE", [][]byte{[]byte("trie1"), []byte("word")}},
		{"TRIE.AUTOCOMPLETE full", "TRIE.AUTOCOMPLETE", [][]byte{[]byte("trie1"), []byte("pre")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestPubSubCommandsFinalCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterPubSubCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"SUBSCRIBE channel", "SUBSCRIBE", [][]byte{[]byte("channel1")}},
		{"UNSUBSCRIBE channel", "UNSUBSCRIBE", [][]byte{[]byte("channel1")}},
		{"PUBLISH message", "PUBLISH", [][]byte{[]byte("channel1"), []byte("message")}},
		{"PSUBSCRIBE pattern", "PSUBSCRIBE", [][]byte{[]byte("pattern*")}},
		{"PUNSUBSCRIBE pattern", "PUNSUBSCRIBE", [][]byte{[]byte("pattern*")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestStreamCommandsFinalCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStreamCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"XADD full", "XADD", [][]byte{[]byte("stream1"), []byte("*"), []byte("field1"), []byte("value1")}},
		{"XRANGE full", "XRANGE", [][]byte{[]byte("stream1"), []byte("-"), []byte("+")}},
		{"XREVRANGE full", "XREVRANGE", [][]byte{[]byte("stream1"), []byte("+"), []byte("-")}},
		{"XREAD full", "XREAD", [][]byte{[]byte("STREAMS"), []byte("stream1"), []byte("0")}},
		{"XGROUP.CREATE full", "XGROUP.CREATE", [][]byte{[]byte("CREATE"), []byte("stream1"), []byte("group1"), []byte("$")}},
		{"XREADGROUP full", "XREADGROUP", [][]byte{[]byte("GROUP"), []byte("group1"), []byte("consumer1"), []byte("STREAMS"), []byte("stream1"), []byte(">")}},
		{"XPENDING full", "XPENDING", [][]byte{[]byte("stream1"), []byte("group1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestSortedSetCommandsFinalCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterSortedSetCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"ZADD full", "ZADD", [][]byte{[]byte("zset1"), []byte("1"), []byte("member1")}},
		{"ZRANGE full", "ZRANGE", [][]byte{[]byte("zset1"), []byte("0"), []byte("-1")}},
		{"ZREVRANGE full", "ZREVRANGE", [][]byte{[]byte("zset1"), []byte("0"), []byte("-1")}},
		{"ZRANGEBYSCORE full", "ZRANGEBYSCORE", [][]byte{[]byte("zset1"), []byte("0"), []byte("100")}},
		{"ZREVRANGEBYSCORE full", "ZREVRANGEBYSCORE", [][]byte{[]byte("zset1"), []byte("100"), []byte("0")}},
		{"ZRANK full", "ZRANK", [][]byte{[]byte("zset1"), []byte("member1")}},
		{"ZREVRANK full", "ZREVRANK", [][]byte{[]byte("zset1"), []byte("member1")}},
		{"ZREM full", "ZREM", [][]byte{[]byte("zset1"), []byte("member1")}},
		{"ZSCORE full", "ZSCORE", [][]byte{[]byte("zset1"), []byte("member1")}},
		{"ZCARD full", "ZCARD", [][]byte{[]byte("zset1")}},
		{"ZCOUNT full", "ZCOUNT", [][]byte{[]byte("zset1"), []byte("0"), []byte("100")}},
		{"ZINCRBY full", "ZINCRBY", [][]byte{[]byte("zset1"), []byte("1"), []byte("member1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}
