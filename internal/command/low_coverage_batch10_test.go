package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestLowCoverageBatch10_ShardCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtraCommands(router)

	t.Run("SHARD.MAP no shard", func(t *testing.T) {
		_ = runHandler(t, router, "SHARD.MAP", [][]byte{[]byte("shard1"), []byte("key1")}, s)
	})

	t.Run("SHARD.MOVE", func(t *testing.T) {
		_ = runHandler(t, router, "SHARD.MOVE", [][]byte{[]byte("shard1"), []byte("node1")}, s)
	})

	t.Run("SHARD.REBALANCE no shard", func(t *testing.T) {
		_ = runHandler(t, router, "SHARD.REBALANCE", [][]byte{[]byte("shard1")}, s)
	})

	t.Run("SHARD.LIST", func(t *testing.T) {
		_ = runHandler(t, router, "SHARD.LIST", [][]byte{}, s)
	})

	t.Run("SHARD.STATUS no shard", func(t *testing.T) {
		_ = runHandler(t, router, "SHARD.STATUS", [][]byte{[]byte("shard1")}, s)
	})
}

func TestLowCoverageBatch10_CompressionCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtraCommands(router)

	t.Run("COMPRESSION.COMPRESS", func(t *testing.T) {
		_ = runHandler(t, router, "COMPRESSION.COMPRESS", [][]byte{[]byte("test data")}, s)
	})

	t.Run("COMPRESSION.DEOMPRESS", func(t *testing.T) {
		_ = runHandler(t, router, "COMPRESSION.DEOMPRESS", [][]byte{[]byte("compressed:9")}, s)
	})

	t.Run("COMPRESSION.INFO", func(t *testing.T) {
		_ = runHandler(t, router, "COMPRESSION.INFO", [][]byte{}, s)
	})
}

func TestLowCoverageBatch10_DedupCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtraCommands(router)

	t.Run("DEDUP.ADD", func(t *testing.T) {
		_ = runHandler(t, router, "DEDUP.ADD", [][]byte{[]byte("dedup1"), []byte("id1")}, s)
	})

	t.Run("DEDUP.ADD duplicate", func(t *testing.T) {
		_ = runHandler(t, router, "DEDUP.ADD", [][]byte{[]byte("dedup1"), []byte("id1")}, s)
		_ = runHandler(t, router, "DEDUP.ADD", [][]byte{[]byte("dedup1"), []byte("id1")}, s)
	})

	t.Run("DEDUP.ADD with TTL", func(t *testing.T) {
		_ = runHandler(t, router, "DEDUP.ADD", [][]byte{[]byte("dedup2"), []byte("id2"), []byte("60000")}, s)
	})

	t.Run("DEDUP.CHECK exists", func(t *testing.T) {
		_ = runHandler(t, router, "DEDUP.ADD", [][]byte{[]byte("dedup3"), []byte("id3")}, s)
		_ = runHandler(t, router, "DEDUP.CHECK", [][]byte{[]byte("dedup3"), []byte("id3")}, s)
	})

	t.Run("DEDUP.CHECK not exists", func(t *testing.T) {
		_ = runHandler(t, router, "DEDUP.CHECK", [][]byte{[]byte("dedup4"), []byte("id4")}, s)
	})
}

func TestLowCoverageBatch10_EncodingCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterEncodingCommands(router)

	t.Run("TOML.ENCODE", func(t *testing.T) {
		_ = runHandler(t, router, "TOML.ENCODE", [][]byte{[]byte("key1=value1")}, s)
	})

	t.Run("TOML.DECODE", func(t *testing.T) {
		_ = runHandler(t, router, "TOML.DECODE", [][]byte{[]byte("key1 = \"value1\"")}, s)
	})

	t.Run("CBOR.ENCODE", func(t *testing.T) {
		_ = runHandler(t, router, "CBOR.ENCODE", [][]byte{[]byte("test")}, s)
	})

	t.Run("CBOR.DECODE", func(t *testing.T) {
		_ = runHandler(t, router, "CBOR.DECODE", [][]byte{[]byte("test")}, s)
	})

	t.Run("MSGPACK.ENCODE", func(t *testing.T) {
		_ = runHandler(t, router, "MSGPACK.ENCODE", [][]byte{[]byte("test")}, s)
	})

	t.Run("MSGPACK.DECODE", func(t *testing.T) {
		_ = runHandler(t, router, "MSGPACK.DECODE", [][]byte{[]byte("test")}, s)
	})

	t.Run("YAML.ENCODE", func(t *testing.T) {
		_ = runHandler(t, router, "YAML.ENCODE", [][]byte{[]byte("key1: value1")}, s)
	})

	t.Run("YAML.DECODE", func(t *testing.T) {
		_ = runHandler(t, router, "YAML.DECODE", [][]byte{[]byte("key1: value1")}, s)
	})
}

func TestLowCoverageBatch10_DigestCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterDigestCommands(router)

	t.Run("DIGEST.MD5", func(t *testing.T) {
		_ = runHandler(t, router, "DIGEST.MD5", [][]byte{[]byte("test")}, s)
	})

	t.Run("DIGEST.SHA1", func(t *testing.T) {
		_ = runHandler(t, router, "DIGEST.SHA1", [][]byte{[]byte("test")}, s)
	})

	t.Run("DIGEST.SHA256", func(t *testing.T) {
		_ = runHandler(t, router, "DIGEST.SHA256", [][]byte{[]byte("test")}, s)
	})

	t.Run("DIGEST.SHA512", func(t *testing.T) {
		_ = runHandler(t, router, "DIGEST.SHA512", [][]byte{[]byte("test")}, s)
	})

	t.Run("DIGEST.BASE64ENCODE", func(t *testing.T) {
		_ = runHandler(t, router, "DIGEST.BASE64ENCODE", [][]byte{[]byte("test")}, s)
	})

	t.Run("DIGEST.BASE64DECODE", func(t *testing.T) {
		_ = runHandler(t, router, "DIGEST.BASE64DECODE", [][]byte{[]byte("dGVzdA==")}, s)
	})

	t.Run("DIGEST.HEXENCODE", func(t *testing.T) {
		_ = runHandler(t, router, "DIGEST.HEXENCODE", [][]byte{[]byte("test")}, s)
	})

	t.Run("DIGEST.HEXDECODE", func(t *testing.T) {
		_ = runHandler(t, router, "DIGEST.HEXDECODE", [][]byte{[]byte("74657374")}, s)
	})

	t.Run("DIGEST.HMAC", func(t *testing.T) {
		_ = runHandler(t, router, "DIGEST.HMAC", [][]byte{[]byte("sha256"), []byte("secret"), []byte("message")}, s)
	})
}

func TestLowCoverageBatch10_VectorClockCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtraCommands(router)

	t.Run("VECTOR_CLOCK.CREATE", func(t *testing.T) {
		_ = runHandler(t, router, "VECTOR_CLOCK.CREATE", [][]byte{[]byte("vc1")}, s)
	})

	t.Run("VECTOR_CLOCK.INCREMENT", func(t *testing.T) {
		_ = runHandler(t, router, "VECTOR_CLOCK.CREATE", [][]byte{[]byte("vc2")}, s)
		_ = runHandler(t, router, "VECTOR_CLOCK.INCREMENT", [][]byte{[]byte("vc2"), []byte("node1")}, s)
	})

	t.Run("VECTOR_CLOCK.GET", func(t *testing.T) {
		_ = runHandler(t, router, "VECTOR_CLOCK.CREATE", [][]byte{[]byte("vc3")}, s)
		_ = runHandler(t, router, "VECTOR_CLOCK.INCREMENT", [][]byte{[]byte("vc3"), []byte("node1")}, s)
		_ = runHandler(t, router, "VECTOR_CLOCK.GET", [][]byte{[]byte("vc3")}, s)
	})

	t.Run("VECTOR_CLOCK.COMPARE", func(t *testing.T) {
		_ = runHandler(t, router, "VECTOR_CLOCK.CREATE", [][]byte{[]byte("vc4")}, s)
		_ = runHandler(t, router, "VECTOR_CLOCK.CREATE", [][]byte{[]byte("vc5")}, s)
		_ = runHandler(t, router, "VECTOR_CLOCK.COMPARE", [][]byte{[]byte("vc4"), []byte("vc5")}, s)
	})

	t.Run("VECTOR_CLOCK.MERGE", func(t *testing.T) {
		_ = runHandler(t, router, "VECTOR_CLOCK.CREATE", [][]byte{[]byte("vc6")}, s)
		_ = runHandler(t, router, "VECTOR_CLOCK.CREATE", [][]byte{[]byte("vc7")}, s)
		_ = runHandler(t, router, "VECTOR_CLOCK.MERGE", [][]byte{[]byte("vc6"), []byte("vc7")}, s)
	})
}

func TestLowCoverageBatch10_NamespaceCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterNamespaceCommands(router)

	t.Run("NAMESPACE.CREATE", func(t *testing.T) {
		_ = runHandler(t, router, "NAMESPACE.CREATE", [][]byte{[]byte("ns1")}, s)
	})

	t.Run("NAMESPACE.INFO", func(t *testing.T) {
		_ = runHandler(t, router, "NAMESPACE.CREATE", [][]byte{[]byte("ns2")}, s)
		_ = runHandler(t, router, "NAMESPACE.INFO", [][]byte{[]byte("ns2")}, s)
	})

	t.Run("NAMESPACE.LIST", func(t *testing.T) {
		_ = runHandler(t, router, "NAMESPACE.LIST", [][]byte{}, s)
	})

	t.Run("NAMESPACE.DELETE", func(t *testing.T) {
		_ = runHandler(t, router, "NAMESPACE.CREATE", [][]byte{[]byte("ns3")}, s)
		_ = runHandler(t, router, "NAMESPACE.DELETE", [][]byte{[]byte("ns3")}, s)
	})
}

func TestLowCoverageBatch10_SchedulerJobCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterSchedulerCommands(router)

	t.Run("JOB.STATS", func(t *testing.T) {
		_ = runHandler(t, router, "JOB.CREATE", [][]byte{
			[]byte("job1"),
			[]byte("*/5 * * * *"),
			[]byte("echo hello"),
		}, s)
		_ = runHandler(t, router, "JOB.STATS", [][]byte{[]byte("job1")}, s)
	})

	t.Run("JOB.LIST", func(t *testing.T) {
		_ = runHandler(t, router, "JOB.LIST", [][]byte{}, s)
	})

	t.Run("JOB.DELETE", func(t *testing.T) {
		_ = runHandler(t, router, "JOB.CREATE", [][]byte{
			[]byte("job2"),
			[]byte("*/5 * * * *"),
			[]byte("echo hello"),
		}, s)
		_ = runHandler(t, router, "JOB.DELETE", [][]byte{[]byte("job2")}, s)
	})

	t.Run("JOB.PAUSE", func(t *testing.T) {
		_ = runHandler(t, router, "JOB.CREATE", [][]byte{
			[]byte("job3"),
			[]byte("*/5 * * * *"),
			[]byte("echo hello"),
		}, s)
		_ = runHandler(t, router, "JOB.PAUSE", [][]byte{[]byte("job3")}, s)
	})

	t.Run("JOB.RESUME", func(t *testing.T) {
		_ = runHandler(t, router, "JOB.CREATE", [][]byte{
			[]byte("job4"),
			[]byte("*/5 * * * *"),
			[]byte("echo hello"),
		}, s)
		_ = runHandler(t, router, "JOB.PAUSE", [][]byte{[]byte("job4")}, s)
		_ = runHandler(t, router, "JOB.RESUME", [][]byte{[]byte("job4")}, s)
	})
}

func TestLowCoverageBatch10_SchedulerCircuitCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterSchedulerCommands(router)

	t.Run("CIRCUIT.STATS", func(t *testing.T) {
		_ = runHandler(t, router, "CIRCUIT.CREATE", [][]byte{
			[]byte("circuit1"),
			[]byte("5"),
			[]byte("30"),
		}, s)
		_ = runHandler(t, router, "CIRCUIT.STATS", [][]byte{[]byte("circuit1")}, s)
	})

	t.Run("CIRCUIT.TRIP", func(t *testing.T) {
		_ = runHandler(t, router, "CIRCUIT.CREATE", [][]byte{
			[]byte("circuit2"),
			[]byte("5"),
			[]byte("30"),
		}, s)
		_ = runHandler(t, router, "CIRCUIT.TRIP", [][]byte{[]byte("circuit2")}, s)
	})

	t.Run("CIRCUIT.RESET", func(t *testing.T) {
		_ = runHandler(t, router, "CIRCUIT.CREATE", [][]byte{
			[]byte("circuit3"),
			[]byte("5"),
			[]byte("30"),
		}, s)
		_ = runHandler(t, router, "CIRCUIT.TRIP", [][]byte{[]byte("circuit3")}, s)
		_ = runHandler(t, router, "CIRCUIT.RESET", [][]byte{[]byte("circuit3")}, s)
	})

	t.Run("CIRCUIT.LIST", func(t *testing.T) {
		_ = runHandler(t, router, "CIRCUIT.LIST", [][]byte{}, s)
	})
}

func TestLowCoverageBatch10_ResilienceCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	t.Run("OBSERVABILITY.METRIC", func(t *testing.T) {
		_ = runHandler(t, router, "OBSERVABILITY.METRIC", [][]byte{
			[]byte("metric1"),
			[]byte("counter"),
			[]byte("1"),
		}, s)
	})

	t.Run("OBSERVABILITY.TRACE", func(t *testing.T) {
		_ = runHandler(t, router, "OBSERVABILITY.TRACE", [][]byte{
			[]byte("trace1"),
			[]byte("start"),
		}, s)
	})

	t.Run("OBSERVABILITY.SPAN", func(t *testing.T) {
		_ = runHandler(t, router, "OBSERVABILITY.SPAN", [][]byte{
			[]byte("span1"),
			[]byte("trace1"),
			[]byte("operation"),
		}, s)
	})
}

func TestLowCoverageBatch10_SketchCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	t.Run("SKETCH.MERGE", func(t *testing.T) {
		_ = runHandler(t, router, "SKETCH.CREATE", [][]byte{[]byte("sketch1")}, s)
		_ = runHandler(t, router, "SKETCH.CREATE", [][]byte{[]byte("sketch2")}, s)
		_ = runHandler(t, router, "SKETCH.MERGE", [][]byte{[]byte("sketch1"), []byte("sketch2")}, s)
	})

	t.Run("SKETCH.QUERY", func(t *testing.T) {
		_ = runHandler(t, router, "SKETCH.CREATE", [][]byte{[]byte("sketch3")}, s)
		_ = runHandler(t, router, "SKETCH.UPDATE", [][]byte{[]byte("sketch3"), []byte("item1")}, s)
		_ = runHandler(t, router, "SKETCH.QUERY", [][]byte{[]byte("sketch3"), []byte("item1")}, s)
	})
}

func TestLowCoverageBatch10_ReplicationCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterReplicationCommands(router)

	t.Run("REPLICAOF", func(t *testing.T) {
		_ = runHandler(t, router, "REPLICAOF", [][]byte{[]byte("127.0.0.1"), []byte("6379")}, s)
	})

	t.Run("REPLICAOF NO ONE", func(t *testing.T) {
		_ = runHandler(t, router, "REPLICAOF", [][]byte{[]byte("NO"), []byte("ONE")}, s)
	})

	t.Run("SYNC", func(t *testing.T) {
		_ = runHandler(t, router, "SYNC", [][]byte{}, s)
	})

	t.Run("PSYNC", func(t *testing.T) {
		_ = runHandler(t, router, "PSYNC", [][]byte{[]byte("0"), []byte("0")}, s)
	})

	t.Run("WAIT", func(t *testing.T) {
		_ = runHandler(t, router, "WAIT", [][]byte{[]byte("1"), []byte("1000")}, s)
	})
}

func TestLowCoverageBatch10_MonitoringCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMonitoringCommands(router)

	t.Run("MONITOR.START", func(t *testing.T) {
		_ = runHandler(t, router, "MONITOR.START", [][]byte{}, s)
	})

	t.Run("MONITOR.STOP", func(t *testing.T) {
		_ = runHandler(t, router, "MONITOR.STOP", [][]byte{}, s)
	})

	t.Run("SLOWLOG.GET", func(t *testing.T) {
		_ = runHandler(t, router, "SLOWLOG.GET", [][]byte{[]byte("10")}, s)
	})

	t.Run("SLOWLOG.LEN", func(t *testing.T) {
		_ = runHandler(t, router, "SLOWLOG.LEN", [][]byte{}, s)
	})

	t.Run("SLOWLOG.RESET", func(t *testing.T) {
		_ = runHandler(t, router, "SLOWLOG.RESET", [][]byte{}, s)
	})
}

func TestLowCoverageBatch10_DataStructuresCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterDataStructuresCommands(router)

	t.Run("QUEUE.PUSH", func(t *testing.T) {
		_ = runHandler(t, router, "QUEUE.PUSH", [][]byte{[]byte("queue1"), []byte("item1")}, s)
	})

	t.Run("QUEUE.POP", func(t *testing.T) {
		_ = runHandler(t, router, "QUEUE.PUSH", [][]byte{[]byte("queue2"), []byte("item1")}, s)
		_ = runHandler(t, router, "QUEUE.POP", [][]byte{[]byte("queue2")}, s)
	})

	t.Run("QUEUE.PEEK", func(t *testing.T) {
		_ = runHandler(t, router, "QUEUE.PUSH", [][]byte{[]byte("queue3"), []byte("item1")}, s)
		_ = runHandler(t, router, "QUEUE.PEEK", [][]byte{[]byte("queue3")}, s)
	})

	t.Run("STACK.PUSH", func(t *testing.T) {
		_ = runHandler(t, router, "STACK.PUSH", [][]byte{[]byte("stack1"), []byte("item1")}, s)
	})

	t.Run("STACK.POP", func(t *testing.T) {
		_ = runHandler(t, router, "STACK.PUSH", [][]byte{[]byte("stack2"), []byte("item1")}, s)
		_ = runHandler(t, router, "STACK.POP", [][]byte{[]byte("stack2")}, s)
	})

	t.Run("PRIORITYQUEUE.PUSH", func(t *testing.T) {
		_ = runHandler(t, router, "PRIORITYQUEUE.PUSH", [][]byte{[]byte("pq1"), []byte("item1"), []byte("5")}, s)
	})

	t.Run("PRIORITYQUEUE.POP", func(t *testing.T) {
		_ = runHandler(t, router, "PRIORITYQUEUE.PUSH", [][]byte{[]byte("pq2"), []byte("item1"), []byte("5")}, s)
		_ = runHandler(t, router, "PRIORITYQUEUE.POP", [][]byte{[]byte("pq2")}, s)
	})
}

func TestLowCoverageBatch10_SearchCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterSearchCommands(router)

	t.Run("FT.CREATE", func(t *testing.T) {
		_ = runHandler(t, router, "FT.CREATE", [][]byte{[]byte("idx1")}, s)
	})

	t.Run("FT.SEARCH", func(t *testing.T) {
		_ = runHandler(t, router, "FT.CREATE", [][]byte{[]byte("idx2")}, s)
		_ = runHandler(t, router, "FT.SEARCH", [][]byte{[]byte("idx2"), []byte("query")}, s)
	})

	t.Run("FT.DROPINDEX", func(t *testing.T) {
		_ = runHandler(t, router, "FT.CREATE", [][]byte{[]byte("idx3")}, s)
		_ = runHandler(t, router, "FT.DROPINDEX", [][]byte{[]byte("idx3")}, s)
	})
}

func TestLowCoverageBatch10_ProbabilisticCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterProbabilisticCommands(router)

	t.Run("BF.RESERVE", func(t *testing.T) {
		_ = runHandler(t, router, "BF.RESERVE", [][]byte{[]byte("bf0"), []byte("1000000")}, s)
	})

	t.Run("BF.ADD", func(t *testing.T) {
		_ = runHandler(t, router, "BF.RESERVE", [][]byte{[]byte("bf1"), []byte("1000000")}, s)
		_ = runHandler(t, router, "BF.ADD", [][]byte{[]byte("bf1"), []byte("item1")}, s)
	})

	t.Run("BF.EXISTS", func(t *testing.T) {
		_ = runHandler(t, router, "BF.RESERVE", [][]byte{[]byte("bf2"), []byte("1000000")}, s)
		_ = runHandler(t, router, "BF.ADD", [][]byte{[]byte("bf2"), []byte("item1")}, s)
		_ = runHandler(t, router, "BF.EXISTS", [][]byte{[]byte("bf2"), []byte("item1")}, s)
	})

	t.Run("BF.MADD", func(t *testing.T) {
		_ = runHandler(t, router, "BF.RESERVE", [][]byte{[]byte("bf3"), []byte("1000000")}, s)
		_ = runHandler(t, router, "BF.MADD", [][]byte{[]byte("bf3"), []byte("item1"), []byte("item2")}, s)
	})

	t.Run("BF.INFO", func(t *testing.T) {
		_ = runHandler(t, router, "BF.RESERVE", [][]byte{[]byte("bf4"), []byte("1000000")}, s)
		_ = runHandler(t, router, "BF.ADD", [][]byte{[]byte("bf4"), []byte("item1")}, s)
		_ = runHandler(t, router, "BF.INFO", [][]byte{[]byte("bf4")}, s)
	})
}

func TestLowCoverageBatch10_StreamCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStreamCommands(router)

	t.Run("XADD with fields", func(t *testing.T) {
		_ = runHandler(t, router, "XADD", [][]byte{
			[]byte("stream1"),
			[]byte("*"),
			[]byte("field1"),
			[]byte("value1"),
			[]byte("field2"),
			[]byte("value2"),
		}, s)
	})

	t.Run("XLEN", func(t *testing.T) {
		_ = runHandler(t, router, "XADD", [][]byte{[]byte("stream2"), []byte("*"), []byte("f"), []byte("v")}, s)
		_ = runHandler(t, router, "XLEN", [][]byte{[]byte("stream2")}, s)
	})

	t.Run("XRANGE", func(t *testing.T) {
		_ = runHandler(t, router, "XADD", [][]byte{[]byte("stream3"), []byte("*"), []byte("f"), []byte("v")}, s)
		_ = runHandler(t, router, "XRANGE", [][]byte{[]byte("stream3"), []byte("-"), []byte("+")}, s)
	})

	t.Run("XREAD", func(t *testing.T) {
		_ = runHandler(t, router, "XADD", [][]byte{[]byte("stream4"), []byte("*"), []byte("f"), []byte("v")}, s)
		_ = runHandler(t, router, "XREAD", [][]byte{[]byte("STREAMS"), []byte("stream4"), []byte("0")}, s)
	})

	t.Run("XDEL", func(t *testing.T) {
		_ = runHandler(t, router, "XADD", [][]byte{[]byte("stream5"), []byte("1234567890123-0"), []byte("f"), []byte("v")}, s)
		_ = runHandler(t, router, "XDEL", [][]byte{[]byte("stream5"), []byte("1234567890123-0")}, s)
	})

	t.Run("XTRIM", func(t *testing.T) {
		_ = runHandler(t, router, "XADD", [][]byte{[]byte("stream6"), []byte("*"), []byte("f"), []byte("v")}, s)
		_ = runHandler(t, router, "XTRIM", [][]byte{[]byte("stream6"), []byte("MAXLEN"), []byte("100")}, s)
	})
}

func TestLowCoverageBatch10_GeoCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterGeoCommands(router)

	t.Run("GEOADD", func(t *testing.T) {
		_ = runHandler(t, router, "GEOADD", [][]byte{
			[]byte("geo1"),
			[]byte("13.361389"),
			[]byte("38.115556"),
			[]byte("Palermo"),
		}, s)
	})

	t.Run("GEOHASH", func(t *testing.T) {
		_ = runHandler(t, router, "GEOADD", [][]byte{
			[]byte("geo2"),
			[]byte("13.361389"),
			[]byte("38.115556"),
			[]byte("Palermo"),
		}, s)
		_ = runHandler(t, router, "GEOHASH", [][]byte{[]byte("geo2"), []byte("Palermo")}, s)
	})

	t.Run("GEOPOS", func(t *testing.T) {
		_ = runHandler(t, router, "GEOADD", [][]byte{
			[]byte("geo3"),
			[]byte("13.361389"),
			[]byte("38.115556"),
			[]byte("Palermo"),
		}, s)
		_ = runHandler(t, router, "GEOPOS", [][]byte{[]byte("geo3"), []byte("Palermo")}, s)
	})

	t.Run("GEODIST", func(t *testing.T) {
		_ = runHandler(t, router, "GEOADD", [][]byte{
			[]byte("geo4"),
			[]byte("13.361389"),
			[]byte("38.115556"),
			[]byte("Palermo"),
			[]byte("15.087269"),
			[]byte("37.502669"),
			[]byte("Catania"),
		}, s)
		_ = runHandler(t, router, "GEODIST", [][]byte{[]byte("geo4"), []byte("Palermo"), []byte("Catania")}, s)
	})

	t.Run("GEORADIUS", func(t *testing.T) {
		_ = runHandler(t, router, "GEOADD", [][]byte{
			[]byte("geo5"),
			[]byte("13.361389"),
			[]byte("38.115556"),
			[]byte("Palermo"),
		}, s)
		_ = runHandler(t, router, "GEORADIUS", [][]byte{
			[]byte("geo5"),
			[]byte("15"),
			[]byte("37"),
			[]byte("200"),
			[]byte("km"),
		}, s)
	})
}

func TestLowCoverageBatch10_JSONCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterJSONCommands(router)

	t.Run("JSON.SET", func(t *testing.T) {
		_ = runHandler(t, router, "JSON.SET", [][]byte{
			[]byte("json1"),
			[]byte("$"),
			[]byte(`{"key": "value"}`),
		}, s)
	})

	t.Run("JSON.GET", func(t *testing.T) {
		_ = runHandler(t, router, "JSON.SET", [][]byte{
			[]byte("json2"),
			[]byte("$"),
			[]byte(`{"key": "value"}`),
		}, s)
		_ = runHandler(t, router, "JSON.GET", [][]byte{[]byte("json2")}, s)
	})

	t.Run("JSON.DEL", func(t *testing.T) {
		_ = runHandler(t, router, "JSON.SET", [][]byte{
			[]byte("json3"),
			[]byte("$"),
			[]byte(`{"key": "value"}`),
		}, s)
		_ = runHandler(t, router, "JSON.DEL", [][]byte{[]byte("json3"), []byte("$.key")}, s)
	})

	t.Run("JSON.TYPE", func(t *testing.T) {
		_ = runHandler(t, router, "JSON.SET", [][]byte{
			[]byte("json4"),
			[]byte("$"),
			[]byte(`{"key": "value"}`),
		}, s)
		_ = runHandler(t, router, "JSON.TYPE", [][]byte{[]byte("json4")}, s)
	})

	t.Run("JSON.ARRAPPEND", func(t *testing.T) {
		_ = runHandler(t, router, "JSON.SET", [][]byte{
			[]byte("json5"),
			[]byte("$"),
			[]byte(`{"arr": [1, 2, 3]}`),
		}, s)
		_ = runHandler(t, router, "JSON.ARRAPPEND", [][]byte{
			[]byte("json5"),
			[]byte("$.arr"),
			[]byte("4"),
		}, s)
	})
}

func TestLowCoverageBatch10_HyperLogLogCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterHyperLogLogCommands(router)

	t.Run("PFADD", func(t *testing.T) {
		_ = runHandler(t, router, "PFADD", [][]byte{[]byte("hll1"), []byte("a"), []byte("b"), []byte("c")}, s)
	})

	t.Run("PFCOUNT", func(t *testing.T) {
		_ = runHandler(t, router, "PFADD", [][]byte{[]byte("hll2"), []byte("a"), []byte("b"), []byte("c")}, s)
		_ = runHandler(t, router, "PFCOUNT", [][]byte{[]byte("hll2")}, s)
	})

	t.Run("PFMERGE", func(t *testing.T) {
		_ = runHandler(t, router, "PFADD", [][]byte{[]byte("hll3"), []byte("a"), []byte("b")}, s)
		_ = runHandler(t, router, "PFADD", [][]byte{[]byte("hll4"), []byte("c"), []byte("d")}, s)
		_ = runHandler(t, router, "PFMERGE", [][]byte{[]byte("hll5"), []byte("hll3"), []byte("hll4")}, s)
	})
}

func TestLowCoverageBatch10_BitmapCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterBitmapCommands(router)

	t.Run("SETBIT", func(t *testing.T) {
		_ = runHandler(t, router, "SETBIT", [][]byte{[]byte("bm1"), []byte("0"), []byte("1")}, s)
	})

	t.Run("GETBIT", func(t *testing.T) {
		_ = runHandler(t, router, "SETBIT", [][]byte{[]byte("bm2"), []byte("0"), []byte("1")}, s)
		_ = runHandler(t, router, "GETBIT", [][]byte{[]byte("bm2"), []byte("0")}, s)
	})

	t.Run("BITCOUNT", func(t *testing.T) {
		_ = runHandler(t, router, "SETBIT", [][]byte{[]byte("bm3"), []byte("0"), []byte("1")}, s)
		_ = runHandler(t, router, "BITCOUNT", [][]byte{[]byte("bm3")}, s)
	})

	t.Run("BITPOS", func(t *testing.T) {
		_ = runHandler(t, router, "SETBIT", [][]byte{[]byte("bm4"), []byte("0"), []byte("1")}, s)
		_ = runHandler(t, router, "BITPOS", [][]byte{[]byte("bm4"), []byte("1")}, s)
	})

	t.Run("BITOP", func(t *testing.T) {
		_ = runHandler(t, router, "SETBIT", [][]byte{[]byte("bm5"), []byte("0"), []byte("1")}, s)
		_ = runHandler(t, router, "SETBIT", [][]byte{[]byte("bm6"), []byte("0"), []byte("1")}, s)
		_ = runHandler(t, router, "BITOP", [][]byte{[]byte("AND"), []byte("bm7"), []byte("bm5"), []byte("bm6")}, s)
	})
}

func TestLowCoverageBatch10_TimeSeriesCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterTSCommands(router)

	t.Run("TS.CREATE", func(t *testing.T) {
		_ = runHandler(t, router, "TS.CREATE", [][]byte{[]byte("ts1")}, s)
	})

	t.Run("TS.ADD", func(t *testing.T) {
		_ = runHandler(t, router, "TS.CREATE", [][]byte{[]byte("ts2")}, s)
		_ = runHandler(t, router, "TS.ADD", [][]byte{[]byte("ts2"), []byte("*"), []byte("100")}, s)
	})

	t.Run("TS.GET", func(t *testing.T) {
		_ = runHandler(t, router, "TS.CREATE", [][]byte{[]byte("ts3")}, s)
		_ = runHandler(t, router, "TS.ADD", [][]byte{[]byte("ts3"), []byte("*"), []byte("100")}, s)
		_ = runHandler(t, router, "TS.GET", [][]byte{[]byte("ts3")}, s)
	})

	t.Run("TS.RANGE", func(t *testing.T) {
		_ = runHandler(t, router, "TS.CREATE", [][]byte{[]byte("ts4")}, s)
		_ = runHandler(t, router, "TS.ADD", [][]byte{[]byte("ts4"), []byte("*"), []byte("100")}, s)
		_ = runHandler(t, router, "TS.RANGE", [][]byte{[]byte("ts4"), []byte("-"), []byte("+")}, s)
	})
}

func TestLowCoverageBatch10_TagCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterTagCommands(router)

	t.Run("TAG.ADD", func(t *testing.T) {
		_ = runHandler(t, router, "TAG.ADD", [][]byte{[]byte("key1"), []byte("tag1")}, s)
	})

	t.Run("TAG.REMOVE", func(t *testing.T) {
		_ = runHandler(t, router, "TAG.ADD", [][]byte{[]byte("key2"), []byte("tag1")}, s)
		_ = runHandler(t, router, "TAG.REMOVE", [][]byte{[]byte("key2"), []byte("tag1")}, s)
	})

	t.Run("TAG.GET", func(t *testing.T) {
		_ = runHandler(t, router, "TAG.ADD", [][]byte{[]byte("key3"), []byte("tag1")}, s)
		_ = runHandler(t, router, "TAG.GET", [][]byte{[]byte("key3")}, s)
	})

	t.Run("TAG.FIND", func(t *testing.T) {
		_ = runHandler(t, router, "TAG.ADD", [][]byte{[]byte("key4"), []byte("tag1")}, s)
		_ = runHandler(t, router, "TAG.FIND", [][]byte{[]byte("tag1")}, s)
	})
}
