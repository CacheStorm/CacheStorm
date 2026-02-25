package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestLowCoverageBatch12_ExtraCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtraCommands(router)

	t.Run("GHOST.CREATE", func(t *testing.T) {
		_ = runHandler(t, router, "GHOST.CREATE", [][]byte{[]byte("ghost1")}, s)
	})

	t.Run("GHOST.ADD", func(t *testing.T) {
		_ = runHandler(t, router, "GHOST.CREATE", [][]byte{[]byte("ghost2")}, s)
		_ = runHandler(t, router, "GHOST.ADD", [][]byte{[]byte("ghost2"), []byte("key1")}, s)
	})

	t.Run("GHOST.GET", func(t *testing.T) {
		_ = runHandler(t, router, "GHOST.CREATE", [][]byte{[]byte("ghost3")}, s)
		_ = runHandler(t, router, "GHOST.ADD", [][]byte{[]byte("ghost3"), []byte("key1")}, s)
		_ = runHandler(t, router, "GHOST.GET", [][]byte{[]byte("ghost3"), []byte("key1")}, s)
	})

	t.Run("GHOST.DELETE", func(t *testing.T) {
		_ = runHandler(t, router, "GHOST.CREATE", [][]byte{[]byte("ghost4")}, s)
		_ = runHandler(t, router, "GHOST.DELETE", [][]byte{[]byte("ghost4")}, s)
	})

	t.Run("SHARD.MAP with nodes", func(t *testing.T) {
		_ = runHandler(t, router, "SHARD.CREATE", [][]byte{[]byte("shard1"), []byte("3")}, s)
		_ = runHandler(t, router, "SHARD.MAP", [][]byte{[]byte("shard1"), []byte("key1")}, s)
	})

	t.Run("SHARD.STATUS with nodes", func(t *testing.T) {
		_ = runHandler(t, router, "SHARD.CREATE", [][]byte{[]byte("shard2"), []byte("3")}, s)
		_ = runHandler(t, router, "SHARD.STATUS", [][]byte{[]byte("shard2")}, s)
	})

	t.Run("DEDUP.REMOVE", func(t *testing.T) {
		_ = runHandler(t, router, "DEDUP.ADD", [][]byte{[]byte("dedup1"), []byte("id1")}, s)
		_ = runHandler(t, router, "DEDUP.REMOVE", [][]byte{[]byte("dedup1"), []byte("id1")}, s)
	})

	t.Run("DEDUP.CLEAR", func(t *testing.T) {
		_ = runHandler(t, router, "DEDUP.ADD", [][]byte{[]byte("dedup2"), []byte("id1")}, s)
		_ = runHandler(t, router, "DEDUP.CLEAR", [][]byte{[]byte("dedup2")}, s)
	})

	t.Run("COMPRESSION.COMPRESS", func(t *testing.T) {
		_ = runHandler(t, router, "COMPRESSION.COMPRESS", [][]byte{[]byte("test data")}, s)
	})

	t.Run("COMPRESSION.DECOMPRESS", func(t *testing.T) {
		_ = runHandler(t, router, "COMPRESSION.DECOMPRESS", [][]byte{[]byte("compressed:9")}, s)
	})

	t.Run("COMPRESSION.INFO", func(t *testing.T) {
		_ = runHandler(t, router, "COMPRESSION.INFO", [][]byte{}, s)
	})
}

func TestLowCoverageBatch12_EncodingCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterEncodingCommands(router)

	t.Run("TOML.ENCODE complex", func(t *testing.T) {
		_ = runHandler(t, router, "TOML.ENCODE", [][]byte{[]byte("key1 = \"value1\"\nkey2 = 123\n[section]\nname = \"test\"")}, s)
	})

	t.Run("TOML.DECODE complex", func(t *testing.T) {
		_ = runHandler(t, router, "TOML.DECODE", [][]byte{[]byte("key1 = \"value1\"\nkey2 = 123")}, s)
	})

	t.Run("CBOR.ENCODE complex", func(t *testing.T) {
		_ = runHandler(t, router, "CBOR.ENCODE", [][]byte{[]byte("{\"key\": \"value\"}")}, s)
	})

	t.Run("CBOR.DECODE complex", func(t *testing.T) {
		_ = runHandler(t, router, "CBOR.DECODE", [][]byte{[]byte("a1keyavalue")}, s)
	})

	t.Run("MSGPACK.ENCODE complex", func(t *testing.T) {
		_ = runHandler(t, router, "MSGPACK.ENCODE", [][]byte{[]byte("{\"key\": \"value\"}")}, s)
	})

	t.Run("MSGPACK.DECODE complex", func(t *testing.T) {
		_ = runHandler(t, router, "MSGPACK.DECODE", [][]byte{[]byte("\x81\xa3key\xa5value")}, s)
	})

	t.Run("YAML.ENCODE complex", func(t *testing.T) {
		_ = runHandler(t, router, "YAML.ENCODE", [][]byte{[]byte("key1: value1\nkey2: 123\nnested:\n  subkey: subvalue")}, s)
	})

	t.Run("YAML.DECODE complex", func(t *testing.T) {
		_ = runHandler(t, router, "YAML.DECODE", [][]byte{[]byte("key1: value1\nkey2: 123")}, s)
	})

	t.Run("XML.ENCODE", func(t *testing.T) {
		_ = runHandler(t, router, "XML.ENCODE", [][]byte{[]byte("<root><key>value</key></root>")}, s)
	})

	t.Run("XML.DECODE", func(t *testing.T) {
		_ = runHandler(t, router, "XML.DECODE", [][]byte{[]byte("<root><key>value</key></root>")}, s)
	})

	t.Run("PROTOBUF.ENCODE", func(t *testing.T) {
		_ = runHandler(t, router, "PROTOBUF.ENCODE", [][]byte{[]byte("test")}, s)
	})

	t.Run("PROTOBUF.DECODE", func(t *testing.T) {
		_ = runHandler(t, router, "PROTOBUF.DECODE", [][]byte{[]byte("test")}, s)
	})

	t.Run("AVRO.ENCODE", func(t *testing.T) {
		_ = runHandler(t, router, "AVRO.ENCODE", [][]byte{[]byte("test")}, s)
	})

	t.Run("AVRO.DECODE", func(t *testing.T) {
		_ = runHandler(t, router, "AVRO.DECODE", [][]byte{[]byte("test")}, s)
	})
}

func TestLowCoverageBatch12_DigestCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterDigestCommands(router)

	t.Run("DIGEST.MD5 with data", func(t *testing.T) {
		_ = runHandler(t, router, "DIGEST.MD5", [][]byte{[]byte("hello world")}, s)
	})

	t.Run("DIGEST.SHA1 with data", func(t *testing.T) {
		_ = runHandler(t, router, "DIGEST.SHA1", [][]byte{[]byte("hello world")}, s)
	})

	t.Run("DIGEST.SHA256 with data", func(t *testing.T) {
		_ = runHandler(t, router, "DIGEST.SHA256", [][]byte{[]byte("hello world")}, s)
	})

	t.Run("DIGEST.SHA512 with data", func(t *testing.T) {
		_ = runHandler(t, router, "DIGEST.SHA512", [][]byte{[]byte("hello world")}, s)
	})

	t.Run("DIGEST.BASE64ENCODE with data", func(t *testing.T) {
		_ = runHandler(t, router, "DIGEST.BASE64ENCODE", [][]byte{[]byte("hello world")}, s)
	})

	t.Run("DIGEST.BASE64DECODE with data", func(t *testing.T) {
		_ = runHandler(t, router, "DIGEST.BASE64DECODE", [][]byte{[]byte("aGVsbG8gd29ybGQ=")}, s)
	})

	t.Run("DIGEST.HEXENCODE with data", func(t *testing.T) {
		_ = runHandler(t, router, "DIGEST.HEXENCODE", [][]byte{[]byte("hello")}, s)
	})

	t.Run("DIGEST.HEXDECODE with data", func(t *testing.T) {
		_ = runHandler(t, router, "DIGEST.HEXDECODE", [][]byte{[]byte("68656c6c6f")}, s)
	})

	t.Run("DIGEST.HMAC with data", func(t *testing.T) {
		_ = runHandler(t, router, "DIGEST.HMAC", [][]byte{[]byte("sha256"), []byte("secret"), []byte("message")}, s)
	})

	t.Run("DIGEST.CRC32", func(t *testing.T) {
		_ = runHandler(t, router, "DIGEST.CRC32", [][]byte{[]byte("hello world")}, s)
	})

	t.Run("DIGEST.ADLER32", func(t *testing.T) {
		_ = runHandler(t, router, "DIGEST.ADLER32", [][]byte{[]byte("hello world")}, s)
	})
}

func TestLowCoverageBatch12_IntegrationCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterIntegrationCommands(router)
	RegisterListCommands(router)
	RegisterHashCommands(router)

	t.Run("CACHE.LOCK with holder", func(t *testing.T) {
		_ = runHandler(t, router, "CACHE.LOCK", [][]byte{[]byte("lock1"), []byte("holder1"), []byte("10000")}, s)
	})

	t.Run("CACHE.UNLOCK", func(t *testing.T) {
		_ = runHandler(t, router, "CACHE.LOCK", [][]byte{[]byte("lock2"), []byte("holder2"), []byte("10000")}, s)
		_ = runHandler(t, router, "CACHE.UNLOCK", [][]byte{[]byte("lock2"), []byte("holder2")}, s)
	})

	t.Run("CACHE.REFRESH with holder", func(t *testing.T) {
		_ = runHandler(t, router, "CACHE.LOCK", [][]byte{[]byte("refresh1"), []byte("holder1"), []byte("10000")}, s)
		_ = runHandler(t, router, "CACHE.REFRESH", [][]byte{[]byte("refresh1"), []byte("holder1"), []byte("20000")}, s)
	})

	t.Run("ARRAY.CREATE", func(t *testing.T) {
		_ = runHandler(t, router, "ARRAY.CREATE", [][]byte{[]byte("arr1")}, s)
	})

	t.Run("ARRAY.PUSH", func(t *testing.T) {
		_ = runHandler(t, router, "ARRAY.CREATE", [][]byte{[]byte("arr2")}, s)
		_ = runHandler(t, router, "ARRAY.PUSH", [][]byte{[]byte("arr2"), []byte("item1")}, s)
	})

	t.Run("ARRAY.POP", func(t *testing.T) {
		_ = runHandler(t, router, "ARRAY.CREATE", [][]byte{[]byte("arr3")}, s)
		_ = runHandler(t, router, "ARRAY.PUSH", [][]byte{[]byte("arr3"), []byte("item1")}, s)
		_ = runHandler(t, router, "ARRAY.POP", [][]byte{[]byte("arr3")}, s)
	})

	t.Run("ARRAY.GET", func(t *testing.T) {
		_ = runHandler(t, router, "ARRAY.CREATE", [][]byte{[]byte("arr4")}, s)
		_ = runHandler(t, router, "ARRAY.PUSH", [][]byte{[]byte("arr4"), []byte("item1")}, s)
		_ = runHandler(t, router, "ARRAY.GET", [][]byte{[]byte("arr4")}, s)
	})

	t.Run("ARRAY.MERGE with src", func(t *testing.T) {
		_ = runHandler(t, router, "ARRAY.CREATE", [][]byte{[]byte("dest1")}, s)
		_ = runHandler(t, router, "ARRAY.CREATE", [][]byte{[]byte("src1")}, s)
		_ = runHandler(t, router, "ARRAY.PUSH", [][]byte{[]byte("src1"), []byte("item1")}, s)
		_ = runHandler(t, router, "ARRAY.MERGE", [][]byte{[]byte("dest1"), []byte("src1"), []byte("concat")}, s)
	})

	t.Run("ARRAY.INTERSECT", func(t *testing.T) {
		_ = runHandler(t, router, "ARRAY.CREATE", [][]byte{[]byte("inter1")}, s)
		_ = runHandler(t, router, "ARRAY.PUSH", [][]byte{[]byte("inter1"), []byte("a")}, s)
		_ = runHandler(t, router, "ARRAY.PUSH", [][]byte{[]byte("inter1"), []byte("b")}, s)
		_ = runHandler(t, router, "ARRAY.CREATE", [][]byte{[]byte("inter2")}, s)
		_ = runHandler(t, router, "ARRAY.PUSH", [][]byte{[]byte("inter2"), []byte("b")}, s)
		_ = runHandler(t, router, "ARRAY.PUSH", [][]byte{[]byte("inter2"), []byte("c")}, s)
		_ = runHandler(t, router, "ARRAY.INTERSECT", [][]byte{[]byte("inter1"), []byte("inter2")}, s)
	})

	t.Run("ARRAY.DIFF", func(t *testing.T) {
		_ = runHandler(t, router, "ARRAY.CREATE", [][]byte{[]byte("diff1")}, s)
		_ = runHandler(t, router, "ARRAY.PUSH", [][]byte{[]byte("diff1"), []byte("a")}, s)
		_ = runHandler(t, router, "ARRAY.PUSH", [][]byte{[]byte("diff1"), []byte("b")}, s)
		_ = runHandler(t, router, "ARRAY.CREATE", [][]byte{[]byte("diff2")}, s)
		_ = runHandler(t, router, "ARRAY.PUSH", [][]byte{[]byte("diff2"), []byte("b")}, s)
		_ = runHandler(t, router, "ARRAY.DIFF", [][]byte{[]byte("diff1"), []byte("diff2")}, s)
	})

	t.Run("ARRAY.INDEXOF", func(t *testing.T) {
		_ = runHandler(t, router, "ARRAY.CREATE", [][]byte{[]byte("idx1")}, s)
		_ = runHandler(t, router, "ARRAY.PUSH", [][]byte{[]byte("idx1"), []byte("a")}, s)
		_ = runHandler(t, router, "ARRAY.PUSH", [][]byte{[]byte("idx1"), []byte("b")}, s)
		_ = runHandler(t, router, "ARRAY.INDEXOF", [][]byte{[]byte("idx1"), []byte("b")}, s)
	})

	t.Run("OBJECT.CREATE", func(t *testing.T) {
		_ = runHandler(t, router, "OBJECT.CREATE", [][]byte{[]byte("obj1")}, s)
	})

	t.Run("OBJECT.SET", func(t *testing.T) {
		_ = runHandler(t, router, "OBJECT.CREATE", [][]byte{[]byte("obj2")}, s)
		_ = runHandler(t, router, "OBJECT.SET", [][]byte{[]byte("obj2"), []byte("key1"), []byte("value1")}, s)
	})

	t.Run("OBJECT.GET", func(t *testing.T) {
		_ = runHandler(t, router, "OBJECT.CREATE", [][]byte{[]byte("obj3")}, s)
		_ = runHandler(t, router, "OBJECT.SET", [][]byte{[]byte("obj3"), []byte("key1"), []byte("value1")}, s)
		_ = runHandler(t, router, "OBJECT.GET", [][]byte{[]byte("obj3"), []byte("key1")}, s)
	})

	t.Run("OBJECT.DELETE", func(t *testing.T) {
		_ = runHandler(t, router, "OBJECT.CREATE", [][]byte{[]byte("obj4")}, s)
		_ = runHandler(t, router, "OBJECT.SET", [][]byte{[]byte("obj4"), []byte("key1"), []byte("value1")}, s)
		_ = runHandler(t, router, "OBJECT.DELETE", [][]byte{[]byte("obj4"), []byte("key1")}, s)
	})

	t.Run("OBJECT.MERGE with objects", func(t *testing.T) {
		_ = runHandler(t, router, "OBJECT.CREATE", [][]byte{[]byte("obj5")}, s)
		_ = runHandler(t, router, "OBJECT.SET", [][]byte{[]byte("obj5"), []byte("a"), []byte("1")}, s)
		_ = runHandler(t, router, "OBJECT.CREATE", [][]byte{[]byte("obj6")}, s)
		_ = runHandler(t, router, "OBJECT.SET", [][]byte{[]byte("obj6"), []byte("b"), []byte("2")}, s)
		_ = runHandler(t, router, "OBJECT.MERGE", [][]byte{[]byte("obj5"), []byte("obj6")}, s)
	})

	t.Run("OBJECT.KEYS", func(t *testing.T) {
		_ = runHandler(t, router, "OBJECT.CREATE", [][]byte{[]byte("obj7")}, s)
		_ = runHandler(t, router, "OBJECT.SET", [][]byte{[]byte("obj7"), []byte("key1"), []byte("value1")}, s)
		_ = runHandler(t, router, "OBJECT.KEYS", [][]byte{[]byte("obj7")}, s)
	})

	t.Run("NET.WHOIS", func(t *testing.T) {
		_ = runHandler(t, router, "NET.WHOIS", [][]byte{[]byte("example.com")}, s)
	})

	t.Run("NET.DNS", func(t *testing.T) {
		_ = runHandler(t, router, "NET.DNS", [][]byte{[]byte("example.com")}, s)
	})

	t.Run("NET.PING", func(t *testing.T) {
		_ = runHandler(t, router, "NET.PING", [][]byte{[]byte("127.0.0.1")}, s)
	})

	t.Run("NET.PORT", func(t *testing.T) {
		_ = runHandler(t, router, "NET.PORT", [][]byte{[]byte("127.0.0.1"), []byte("80")}, s)
	})
}

func TestLowCoverageBatch12_LuaCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterScriptCommands(router)

	t.Run("EVAL with complex script", func(t *testing.T) {
		script := `
			local result = {}
			for i = 1, 10 do
				result[i] = i * 2
			end
			return result
		`
		_ = runHandler(t, router, "EVAL", [][]byte{[]byte(script), []byte("0")}, s)
	})

	t.Run("EVAL with redis calls", func(t *testing.T) {
		s.Set("lua_key1", &store.StringValue{Data: []byte("value1")}, store.SetOptions{})
		script := `return redis.call('GET', KEYS[1])`
		_ = runHandler(t, router, "EVAL", [][]byte{[]byte(script), []byte("1"), []byte("lua_key1")}, s)
	})

	t.Run("EVALSHA", func(t *testing.T) {
		_ = runHandler(t, router, "SCRIPT", [][]byte{[]byte("LOAD"), []byte("return 42")}, s)
		_ = runHandler(t, router, "EVALSHA", [][]byte{[]byte("abc123"), []byte("0")}, s)
	})

	t.Run("SCRIPT EXISTS", func(t *testing.T) {
		_ = runHandler(t, router, "SCRIPT", [][]byte{[]byte("EXISTS"), []byte("abc123")}, s)
	})

	t.Run("SCRIPT FLUSH", func(t *testing.T) {
		_ = runHandler(t, router, "SCRIPT", [][]byte{[]byte("FLUSH")}, s)
	})
}

func TestLowCoverageBatch12_ReplicationCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterReplicationCommands(router)

	t.Run("REPLICAOF with host", func(t *testing.T) {
		_ = runHandler(t, router, "REPLICAOF", [][]byte{[]byte("192.168.1.1"), []byte("6379")}, s)
	})

	t.Run("REPLICAOF NO ONE", func(t *testing.T) {
		_ = runHandler(t, router, "REPLICAOF", [][]byte{[]byte("NO"), []byte("ONE")}, s)
	})

	t.Run("SYNC", func(t *testing.T) {
		_ = runHandler(t, router, "SYNC", [][]byte{}, s)
	})

	t.Run("PSYNC", func(t *testing.T) {
		_ = runHandler(t, router, "PSYNC", [][]byte{[]byte("?"), []byte("-1")}, s)
	})

	t.Run("WAIT", func(t *testing.T) {
		_ = runHandler(t, router, "WAIT", [][]byte{[]byte("1"), []byte("1000")}, s)
	})

	t.Run("INFO replication", func(t *testing.T) {
		_ = runHandler(t, router, "INFO", [][]byte{[]byte("replication")}, s)
	})
}

func TestLowCoverageBatch12_DataStructuresCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterDataStructuresCommands(router)

	t.Run("QUEUE.PUSH multiple", func(t *testing.T) {
		_ = runHandler(t, router, "QUEUE.PUSH", [][]byte{[]byte("q1"), []byte("a")}, s)
		_ = runHandler(t, router, "QUEUE.PUSH", [][]byte{[]byte("q1"), []byte("b")}, s)
		_ = runHandler(t, router, "QUEUE.PUSH", [][]byte{[]byte("q1"), []byte("c")}, s)
	})

	t.Run("QUEUE.POP all", func(t *testing.T) {
		_ = runHandler(t, router, "QUEUE.PUSH", [][]byte{[]byte("q2"), []byte("a")}, s)
		_ = runHandler(t, router, "QUEUE.POP", [][]byte{[]byte("q2")}, s)
		_ = runHandler(t, router, "QUEUE.POP", [][]byte{[]byte("q2")}, s)
	})

	t.Run("QUEUE.PEEK", func(t *testing.T) {
		_ = runHandler(t, router, "QUEUE.PUSH", [][]byte{[]byte("q3"), []byte("a")}, s)
		_ = runHandler(t, router, "QUEUE.PEEK", [][]byte{[]byte("q3")}, s)
	})

	t.Run("QUEUE.LEN", func(t *testing.T) {
		_ = runHandler(t, router, "QUEUE.PUSH", [][]byte{[]byte("q4"), []byte("a")}, s)
		_ = runHandler(t, router, "QUEUE.LEN", [][]byte{[]byte("q4")}, s)
	})

	t.Run("STACK.PUSH multiple", func(t *testing.T) {
		_ = runHandler(t, router, "STACK.PUSH", [][]byte{[]byte("s1"), []byte("a")}, s)
		_ = runHandler(t, router, "STACK.PUSH", [][]byte{[]byte("s1"), []byte("b")}, s)
	})

	t.Run("STACK.POP", func(t *testing.T) {
		_ = runHandler(t, router, "STACK.PUSH", [][]byte{[]byte("s2"), []byte("a")}, s)
		_ = runHandler(t, router, "STACK.POP", [][]byte{[]byte("s2")}, s)
	})

	t.Run("STACK.PEEK", func(t *testing.T) {
		_ = runHandler(t, router, "STACK.PUSH", [][]byte{[]byte("s3"), []byte("a")}, s)
		_ = runHandler(t, router, "STACK.PEEK", [][]byte{[]byte("s3")}, s)
	})

	t.Run("PRIORITYQUEUE.PUSH multiple", func(t *testing.T) {
		_ = runHandler(t, router, "PRIORITYQUEUE.PUSH", [][]byte{[]byte("pq1"), []byte("a"), []byte("5")}, s)
		_ = runHandler(t, router, "PRIORITYQUEUE.PUSH", [][]byte{[]byte("pq1"), []byte("b"), []byte("3")}, s)
		_ = runHandler(t, router, "PRIORITYQUEUE.PUSH", [][]byte{[]byte("pq1"), []byte("c"), []byte("7")}, s)
	})

	t.Run("PRIORITYQUEUE.POP", func(t *testing.T) {
		_ = runHandler(t, router, "PRIORITYQUEUE.PUSH", [][]byte{[]byte("pq2"), []byte("a"), []byte("5")}, s)
		_ = runHandler(t, router, "PRIORITYQUEUE.POP", [][]byte{[]byte("pq2")}, s)
	})

	t.Run("PRIORITYQUEUE.PEEK", func(t *testing.T) {
		_ = runHandler(t, router, "PRIORITYQUEUE.PUSH", [][]byte{[]byte("pq3"), []byte("a"), []byte("5")}, s)
		_ = runHandler(t, router, "PRIORITYQUEUE.PEEK", [][]byte{[]byte("pq3")}, s)
	})
}

func TestLowCoverageBatch12_SearchCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterSearchCommands(router)

	t.Run("FT.CREATE with schema", func(t *testing.T) {
		_ = runHandler(t, router, "FT.CREATE", [][]byte{
			[]byte("idx1"),
			[]byte("SCHEMA"),
			[]byte("title"),
			[]byte("TEXT"),
			[]byte("body"),
			[]byte("TEXT"),
		}, s)
	})

	t.Run("FT.ADD", func(t *testing.T) {
		_ = runHandler(t, router, "FT.CREATE", [][]byte{[]byte("idx2")}, s)
		_ = runHandler(t, router, "FT.ADD", [][]byte{
			[]byte("idx2"),
			[]byte("doc1"),
			[]byte("1.0"),
			[]byte("FIELDS"),
			[]byte("title"),
			[]byte("Hello World"),
			[]byte("body"),
			[]byte("This is a test"),
		}, s)
	})

	t.Run("FT.SEARCH with query", func(t *testing.T) {
		_ = runHandler(t, router, "FT.CREATE", [][]byte{[]byte("idx3")}, s)
		_ = runHandler(t, router, "FT.ADD", [][]byte{
			[]byte("idx3"),
			[]byte("doc1"),
			[]byte("1.0"),
			[]byte("FIELDS"),
			[]byte("title"),
			[]byte("Hello"),
		}, s)
		_ = runHandler(t, router, "FT.SEARCH", [][]byte{[]byte("idx3"), []byte("Hello")}, s)
	})

	t.Run("FT.INFO", func(t *testing.T) {
		_ = runHandler(t, router, "FT.CREATE", [][]byte{[]byte("idx4")}, s)
		_ = runHandler(t, router, "FT.INFO", [][]byte{[]byte("idx4")}, s)
	})

	t.Run("FT.DROPINDEX", func(t *testing.T) {
		_ = runHandler(t, router, "FT.CREATE", [][]byte{[]byte("idx5")}, s)
		_ = runHandler(t, router, "FT.DROPINDEX", [][]byte{[]byte("idx5")}, s)
	})
}

func TestLowCoverageBatch12_ProbabilisticCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterProbabilisticCommands(router)

	t.Run("BF.RESERVE with error rate", func(t *testing.T) {
		_ = runHandler(t, router, "BF.RESERVE", [][]byte{[]byte("bf1"), []byte("100000"), []byte("0.001")}, s)
	})

	t.Run("BF.ADD multiple items", func(t *testing.T) {
		_ = runHandler(t, router, "BF.RESERVE", [][]byte{[]byte("bf2"), []byte("10000")}, s)
		_ = runHandler(t, router, "BF.ADD", [][]byte{[]byte("bf2"), []byte("item1")}, s)
		_ = runHandler(t, router, "BF.ADD", [][]byte{[]byte("bf2"), []byte("item2")}, s)
		_ = runHandler(t, router, "BF.ADD", [][]byte{[]byte("bf2"), []byte("item3")}, s)
	})

	t.Run("BF.EXISTS multiple", func(t *testing.T) {
		_ = runHandler(t, router, "BF.RESERVE", [][]byte{[]byte("bf3"), []byte("10000")}, s)
		_ = runHandler(t, router, "BF.ADD", [][]byte{[]byte("bf3"), []byte("exists1")}, s)
		_ = runHandler(t, router, "BF.EXISTS", [][]byte{[]byte("bf3"), []byte("exists1")}, s)
		_ = runHandler(t, router, "BF.EXISTS", [][]byte{[]byte("bf3"), []byte("notexists")}, s)
	})

	t.Run("CF.ADD", func(t *testing.T) {
		_ = runHandler(t, router, "CF.ADD", [][]byte{[]byte("cf1"), []byte("item1")}, s)
	})

	t.Run("CF.EXISTS", func(t *testing.T) {
		_ = runHandler(t, router, "CF.ADD", [][]byte{[]byte("cf2"), []byte("item1")}, s)
		_ = runHandler(t, router, "CF.EXISTS", [][]byte{[]byte("cf2"), []byte("item1")}, s)
	})

	t.Run("TOPK.ADD", func(t *testing.T) {
		_ = runHandler(t, router, "TOPK.ADD", [][]byte{[]byte("topk1"), []byte("item1")}, s)
	})

	t.Run("TOPK.LIST", func(t *testing.T) {
		_ = runHandler(t, router, "TOPK.ADD", [][]byte{[]byte("topk2"), []byte("item1")}, s)
		_ = runHandler(t, router, "TOPK.LIST", [][]byte{[]byte("topk2")}, s)
	})
}

func TestLowCoverageBatch12_StreamCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStreamCommands(router)

	t.Run("XADD with ID", func(t *testing.T) {
		_ = runHandler(t, router, "XADD", [][]byte{
			[]byte("stream1"),
			[]byte("1234567890123-0"),
			[]byte("field1"),
			[]byte("value1"),
		}, s)
	})

	t.Run("XADD multiple entries", func(t *testing.T) {
		_ = runHandler(t, router, "XADD", [][]byte{[]byte("stream2"), []byte("*"), []byte("f"), []byte("v1")}, s)
		_ = runHandler(t, router, "XADD", [][]byte{[]byte("stream2"), []byte("*"), []byte("f"), []byte("v2")}, s)
		_ = runHandler(t, router, "XADD", [][]byte{[]byte("stream2"), []byte("*"), []byte("f"), []byte("v3")}, s)
	})

	t.Run("XRANGE with range", func(t *testing.T) {
		_ = runHandler(t, router, "XADD", [][]byte{[]byte("stream3"), []byte("*"), []byte("f"), []byte("v")}, s)
		_ = runHandler(t, router, "XRANGE", [][]byte{[]byte("stream3"), []byte("-"), []byte("+"), []byte("COUNT"), []byte("10")}, s)
	})

	t.Run("XREVRANGE", func(t *testing.T) {
		_ = runHandler(t, router, "XADD", [][]byte{[]byte("stream4"), []byte("*"), []byte("f"), []byte("v")}, s)
		_ = runHandler(t, router, "XREVRANGE", [][]byte{[]byte("stream4"), []byte("+"), []byte("-")}, s)
	})

	t.Run("XGROUP CREATE", func(t *testing.T) {
		_ = runHandler(t, router, "XADD", [][]byte{[]byte("stream5"), []byte("*"), []byte("f"), []byte("v")}, s)
		_ = runHandler(t, router, "XGROUP", [][]byte{[]byte("CREATE"), []byte("stream5"), []byte("group1"), []byte("$")}, s)
	})

	t.Run("XREADGROUP", func(t *testing.T) {
		_ = runHandler(t, router, "XADD", [][]byte{[]byte("stream6"), []byte("*"), []byte("f"), []byte("v")}, s)
		_ = runHandler(t, router, "XGROUP", [][]byte{[]byte("CREATE"), []byte("stream6"), []byte("group1"), []byte("$")}, s)
		_ = runHandler(t, router, "XREADGROUP", [][]byte{
			[]byte("GROUP"),
			[]byte("group1"),
			[]byte("consumer1"),
			[]byte("STREAMS"),
			[]byte("stream6"),
			[]byte(">"),
		}, s)
	})

	t.Run("XACK", func(t *testing.T) {
		_ = runHandler(t, router, "XADD", [][]byte{[]byte("stream7"), []byte("1234567890123-0"), []byte("f"), []byte("v")}, s)
		_ = runHandler(t, router, "XGROUP", [][]byte{[]byte("CREATE"), []byte("stream7"), []byte("group1"), []byte("$")}, s)
		_ = runHandler(t, router, "XACK", [][]byte{[]byte("stream7"), []byte("group1"), []byte("1234567890123-0")}, s)
	})

	t.Run("XCLAIM", func(t *testing.T) {
		_ = runHandler(t, router, "XADD", [][]byte{[]byte("stream8"), []byte("1234567890123-0"), []byte("f"), []byte("v")}, s)
		_ = runHandler(t, router, "XGROUP", [][]byte{[]byte("CREATE"), []byte("stream8"), []byte("group1"), []byte("$")}, s)
		_ = runHandler(t, router, "XCLAIM", [][]byte{
			[]byte("stream8"),
			[]byte("group1"),
			[]byte("consumer1"),
			[]byte("0"),
			[]byte("1234567890123-0"),
		}, s)
	})
}

func TestLowCoverageBatch12_GeoCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterGeoCommands(router)

	t.Run("GEOADD multiple", func(t *testing.T) {
		_ = runHandler(t, router, "GEOADD", [][]byte{
			[]byte("geo1"),
			[]byte("13.361389"),
			[]byte("38.115556"),
			[]byte("Palermo"),
			[]byte("15.087269"),
			[]byte("37.502669"),
			[]byte("Catania"),
		}, s)
	})

	t.Run("GEOHASH multiple", func(t *testing.T) {
		_ = runHandler(t, router, "GEOADD", [][]byte{
			[]byte("geo2"),
			[]byte("13.361389"),
			[]byte("38.115556"),
			[]byte("Palermo"),
		}, s)
		_ = runHandler(t, router, "GEOHASH", [][]byte{[]byte("geo2"), []byte("Palermo"), []byte("NonExistent")}, s)
	})

	t.Run("GEOPOS multiple", func(t *testing.T) {
		_ = runHandler(t, router, "GEOADD", [][]byte{
			[]byte("geo3"),
			[]byte("13.361389"),
			[]byte("38.115556"),
			[]byte("Palermo"),
		}, s)
		_ = runHandler(t, router, "GEOPOS", [][]byte{[]byte("geo3"), []byte("Palermo")}, s)
	})

	t.Run("GEODIST with unit", func(t *testing.T) {
		_ = runHandler(t, router, "GEOADD", [][]byte{
			[]byte("geo4"),
			[]byte("13.361389"),
			[]byte("38.115556"),
			[]byte("Palermo"),
			[]byte("15.087269"),
			[]byte("37.502669"),
			[]byte("Catania"),
		}, s)
		_ = runHandler(t, router, "GEODIST", [][]byte{[]byte("geo4"), []byte("Palermo"), []byte("Catania"), []byte("km")}, s)
	})

	t.Run("GEORADIUS with options", func(t *testing.T) {
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
			[]byte("WITHDIST"),
		}, s)
	})

	t.Run("GEORADIUSBYMEMBER", func(t *testing.T) {
		_ = runHandler(t, router, "GEOADD", [][]byte{
			[]byte("geo6"),
			[]byte("13.361389"),
			[]byte("38.115556"),
			[]byte("Palermo"),
			[]byte("15.087269"),
			[]byte("37.502669"),
			[]byte("Catania"),
		}, s)
		_ = runHandler(t, router, "GEORADIUSBYMEMBER", [][]byte{
			[]byte("geo6"),
			[]byte("Palermo"),
			[]byte("200"),
			[]byte("km"),
		}, s)
	})
}

func TestLowCoverageBatch12_JSONCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterJSONCommands(router)

	t.Run("JSON.SET nested", func(t *testing.T) {
		_ = runHandler(t, router, "JSON.SET", [][]byte{
			[]byte("json1"),
			[]byte("$"),
			[]byte(`{"user": {"name": "John", "age": 30}}`),
		}, s)
	})

	t.Run("JSON.GET path", func(t *testing.T) {
		_ = runHandler(t, router, "JSON.SET", [][]byte{
			[]byte("json2"),
			[]byte("$"),
			[]byte(`{"user": {"name": "John"}}`),
		}, s)
		_ = runHandler(t, router, "JSON.GET", [][]byte{[]byte("json2"), []byte("$.user.name")}, s)
	})

	t.Run("JSON.TYPE", func(t *testing.T) {
		_ = runHandler(t, router, "JSON.SET", [][]byte{
			[]byte("json3"),
			[]byte("$"),
			[]byte(`{"arr": [1, 2, 3], "obj": {}, "str": "hello", "num": 42, "bool": true}`),
		}, s)
		_ = runHandler(t, router, "JSON.TYPE", [][]byte{[]byte("json3"), []byte("$.arr")}, s)
		_ = runHandler(t, router, "JSON.TYPE", [][]byte{[]byte("json3"), []byte("$.obj")}, s)
		_ = runHandler(t, router, "JSON.TYPE", [][]byte{[]byte("json3"), []byte("$.str")}, s)
		_ = runHandler(t, router, "JSON.TYPE", [][]byte{[]byte("json3"), []byte("$.num")}, s)
	})

	t.Run("JSON.NUMINCRBY", func(t *testing.T) {
		_ = runHandler(t, router, "JSON.SET", [][]byte{
			[]byte("json4"),
			[]byte("$"),
			[]byte(`{"count": 10}`),
		}, s)
		_ = runHandler(t, router, "JSON.NUMINCRBY", [][]byte{[]byte("json4"), []byte("$.count"), []byte("5")}, s)
	})

	t.Run("JSON.ARRAPPEND multiple", func(t *testing.T) {
		_ = runHandler(t, router, "JSON.SET", [][]byte{
			[]byte("json5"),
			[]byte("$"),
			[]byte(`{"arr": [1, 2]}`),
		}, s)
		_ = runHandler(t, router, "JSON.ARRAPPEND", [][]byte{
			[]byte("json5"),
			[]byte("$.arr"),
			[]byte("3"),
			[]byte("4"),
			[]byte("5"),
		}, s)
	})

	t.Run("JSON.ARRPOP", func(t *testing.T) {
		_ = runHandler(t, router, "JSON.SET", [][]byte{
			[]byte("json6"),
			[]byte("$"),
			[]byte(`{"arr": [1, 2, 3]}`),
		}, s)
		_ = runHandler(t, router, "JSON.ARRPOP", [][]byte{[]byte("json6"), []byte("$.arr")}, s)
	})

	t.Run("JSON.OBJKEYS", func(t *testing.T) {
		_ = runHandler(t, router, "JSON.SET", [][]byte{
			[]byte("json7"),
			[]byte("$"),
			[]byte(`{"a": 1, "b": 2, "c": 3}`),
		}, s)
		_ = runHandler(t, router, "JSON.OBJKEYS", [][]byte{[]byte("json7")}, s)
	})

	t.Run("JSON.OBJLEN", func(t *testing.T) {
		_ = runHandler(t, router, "JSON.SET", [][]byte{
			[]byte("json8"),
			[]byte("$"),
			[]byte(`{"a": 1, "b": 2}`),
		}, s)
		_ = runHandler(t, router, "JSON.OBJLEN", [][]byte{[]byte("json8")}, s)
	})
}

func TestLowCoverageBatch12_HyperLogLogCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterHyperLogLogCommands(router)

	t.Run("PFADD many items", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			_ = runHandler(t, router, "PFADD", [][]byte{[]byte("hll1"), []byte(string(rune('a' + i)))}, s)
		}
	})

	t.Run("PFCOUNT multiple keys", func(t *testing.T) {
		_ = runHandler(t, router, "PFADD", [][]byte{[]byte("hll2"), []byte("a"), []byte("b")}, s)
		_ = runHandler(t, router, "PFADD", [][]byte{[]byte("hll3"), []byte("c"), []byte("d")}, s)
		_ = runHandler(t, router, "PFCOUNT", [][]byte{[]byte("hll2"), []byte("hll3")}, s)
	})

	t.Run("PFMERGE multiple", func(t *testing.T) {
		_ = runHandler(t, router, "PFADD", [][]byte{[]byte("hll4"), []byte("a"), []byte("b")}, s)
		_ = runHandler(t, router, "PFADD", [][]byte{[]byte("hll5"), []byte("c"), []byte("d")}, s)
		_ = runHandler(t, router, "PFMERGE", [][]byte{[]byte("hll6"), []byte("hll4"), []byte("hll5")}, s)
		_ = runHandler(t, router, "PFCOUNT", [][]byte{[]byte("hll6")}, s)
	})
}

func TestLowCoverageBatch12_BitmapCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterBitmapCommands(router)

	t.Run("SETBIT multiple positions", func(t *testing.T) {
		_ = runHandler(t, router, "SETBIT", [][]byte{[]byte("bm1"), []byte("0"), []byte("1")}, s)
		_ = runHandler(t, router, "SETBIT", [][]byte{[]byte("bm1"), []byte("1"), []byte("1")}, s)
		_ = runHandler(t, router, "SETBIT", [][]byte{[]byte("bm1"), []byte("2"), []byte("0")}, s)
		_ = runHandler(t, router, "SETBIT", [][]byte{[]byte("bm1"), []byte("3"), []byte("1")}, s)
	})

	t.Run("GETBIT multiple positions", func(t *testing.T) {
		_ = runHandler(t, router, "SETBIT", [][]byte{[]byte("bm2"), []byte("0"), []byte("1")}, s)
		_ = runHandler(t, router, "GETBIT", [][]byte{[]byte("bm2"), []byte("0")}, s)
		_ = runHandler(t, router, "GETBIT", [][]byte{[]byte("bm2"), []byte("1")}, s)
	})

	t.Run("BITCOUNT with range", func(t *testing.T) {
		_ = runHandler(t, router, "SETBIT", [][]byte{[]byte("bm3"), []byte("0"), []byte("1")}, s)
		_ = runHandler(t, router, "SETBIT", [][]byte{[]byte("bm3"), []byte("7"), []byte("1")}, s)
		_ = runHandler(t, router, "BITCOUNT", [][]byte{[]byte("bm3"), []byte("0"), []byte("1")}, s)
	})

	t.Run("BITPOS with options", func(t *testing.T) {
		_ = runHandler(t, router, "SETBIT", [][]byte{[]byte("bm4"), []byte("0"), []byte("1")}, s)
		_ = runHandler(t, router, "BITPOS", [][]byte{[]byte("bm4"), []byte("1")}, s)
		_ = runHandler(t, router, "BITPOS", [][]byte{[]byte("bm4"), []byte("0")}, s)
	})

	t.Run("BITOP multiple", func(t *testing.T) {
		_ = runHandler(t, router, "SETBIT", [][]byte{[]byte("bm5"), []byte("0"), []byte("1")}, s)
		_ = runHandler(t, router, "SETBIT", [][]byte{[]byte("bm6"), []byte("0"), []byte("1")}, s)
		_ = runHandler(t, router, "BITOP", [][]byte{[]byte("AND"), []byte("bm7"), []byte("bm5"), []byte("bm6")}, s)
		_ = runHandler(t, router, "BITOP", [][]byte{[]byte("OR"), []byte("bm8"), []byte("bm5"), []byte("bm6")}, s)
		_ = runHandler(t, router, "BITOP", [][]byte{[]byte("XOR"), []byte("bm9"), []byte("bm5"), []byte("bm6")}, s)
	})
}

func TestLowCoverageBatch12_TimeSeriesCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterTSCommands(router)

	t.Run("TS.CREATE with options", func(t *testing.T) {
		_ = runHandler(t, router, "TS.CREATE", [][]byte{[]byte("ts1"), []byte("RETENTION"), []byte("0")}, s)
	})

	t.Run("TS.ADD with timestamp", func(t *testing.T) {
		_ = runHandler(t, router, "TS.CREATE", [][]byte{[]byte("ts2")}, s)
		_ = runHandler(t, router, "TS.ADD", [][]byte{[]byte("ts2"), []byte("1000"), []byte("10.5")}, s)
	})

	t.Run("TS.GET", func(t *testing.T) {
		_ = runHandler(t, router, "TS.CREATE", [][]byte{[]byte("ts3")}, s)
		_ = runHandler(t, router, "TS.ADD", [][]byte{[]byte("ts3"), []byte("*"), []byte("100")}, s)
		_ = runHandler(t, router, "TS.GET", [][]byte{[]byte("ts3")}, s)
	})

	t.Run("TS.RANGE with options", func(t *testing.T) {
		_ = runHandler(t, router, "TS.CREATE", [][]byte{[]byte("ts4")}, s)
		_ = runHandler(t, router, "TS.ADD", [][]byte{[]byte("ts4"), []byte("1000"), []byte("10")}, s)
		_ = runHandler(t, router, "TS.ADD", [][]byte{[]byte("ts4"), []byte("2000"), []byte("20")}, s)
		_ = runHandler(t, router, "TS.RANGE", [][]byte{[]byte("ts4"), []byte("0"), []byte("+"), []byte("AGGREGATION"), []byte("AVG"), []byte("1000")}, s)
	})

	t.Run("TS.MADD", func(t *testing.T) {
		_ = runHandler(t, router, "TS.CREATE", [][]byte{[]byte("ts5")}, s)
		_ = runHandler(t, router, "TS.MADD", [][]byte{
			[]byte("ts5"), []byte("1000"), []byte("10"),
			[]byte("ts5"), []byte("2000"), []byte("20"),
		}, s)
	})

	t.Run("TS.INCRBY", func(t *testing.T) {
		_ = runHandler(t, router, "TS.CREATE", [][]byte{[]byte("ts6")}, s)
		_ = runHandler(t, router, "TS.INCRBY", [][]byte{[]byte("ts6"), []byte("5")}, s)
	})

	t.Run("TS.DECRBY", func(t *testing.T) {
		_ = runHandler(t, router, "TS.CREATE", [][]byte{[]byte("ts7")}, s)
		_ = runHandler(t, router, "TS.DECRBY", [][]byte{[]byte("ts7"), []byte("3")}, s)
	})
}

func TestLowCoverageBatch12_TagCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterTagCommands(router)

	t.Run("TAG.ADD multiple", func(t *testing.T) {
		_ = runHandler(t, router, "TAG.ADD", [][]byte{[]byte("key1"), []byte("tag1")}, s)
		_ = runHandler(t, router, "TAG.ADD", [][]byte{[]byte("key1"), []byte("tag2")}, s)
		_ = runHandler(t, router, "TAG.ADD", [][]byte{[]byte("key1"), []byte("tag3")}, s)
	})

	t.Run("TAG.GET", func(t *testing.T) {
		_ = runHandler(t, router, "TAG.ADD", [][]byte{[]byte("key2"), []byte("tag1")}, s)
		_ = runHandler(t, router, "TAG.GET", [][]byte{[]byte("key2")}, s)
	})

	t.Run("TAG.REMOVE", func(t *testing.T) {
		_ = runHandler(t, router, "TAG.ADD", [][]byte{[]byte("key3"), []byte("tag1")}, s)
		_ = runHandler(t, router, "TAG.REMOVE", [][]byte{[]byte("key3"), []byte("tag1")}, s)
	})

	t.Run("TAG.FIND", func(t *testing.T) {
		_ = runHandler(t, router, "TAG.ADD", [][]byte{[]byte("key4"), []byte("tag1")}, s)
		_ = runHandler(t, router, "TAG.FIND", [][]byte{[]byte("tag1")}, s)
	})

	t.Run("TAG.LIST", func(t *testing.T) {
		_ = runHandler(t, router, "TAG.LIST", [][]byte{}, s)
	})

	t.Run("TAG.CLEAR", func(t *testing.T) {
		_ = runHandler(t, router, "TAG.ADD", [][]byte{[]byte("key5"), []byte("tag1")}, s)
		_ = runHandler(t, router, "TAG.CLEAR", [][]byte{[]byte("key5")}, s)
	})
}
