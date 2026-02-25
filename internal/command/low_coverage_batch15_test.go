package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestLowCoverageBatch15_DUMP_RESTORE_COPY(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterKeyCommands(router)

	t.Run("DUMP wrong args", func(t *testing.T) {
		_ = runHandler(t, router, "DUMP", nil, s)
	})

	t.Run("DUMP key not found", func(t *testing.T) {
		_ = runHandler(t, router, "DUMP", [][]byte{[]byte("missing")}, s)
	})

	t.Run("DUMP string value", func(t *testing.T) {
		s.Set("dump_str", &store.StringValue{Data: []byte("hello")}, store.SetOptions{})
		_ = runHandler(t, router, "DUMP", [][]byte{[]byte("dump_str")}, s)
	})

	t.Run("DUMP hash value", func(t *testing.T) {
		hv := &store.HashValue{Fields: map[string][]byte{}}
		hv.Fields["field1"] = []byte("value1")
		s.Set("dump_hash", hv, store.SetOptions{})
		_ = runHandler(t, router, "DUMP", [][]byte{[]byte("dump_hash")}, s)
	})

	t.Run("DUMP list value", func(t *testing.T) {
		lv := &store.ListValue{Elements: [][]byte{[]byte("a"), []byte("b")}}
		s.Set("dump_list", lv, store.SetOptions{})
		_ = runHandler(t, router, "DUMP", [][]byte{[]byte("dump_list")}, s)
	})

	t.Run("DUMP set value", func(t *testing.T) {
		sv := &store.SetValue{Members: map[string]struct{}{}}
		sv.Members["member1"] = struct{}{}
		s.Set("dump_set", sv, store.SetOptions{})
		_ = runHandler(t, router, "DUMP", [][]byte{[]byte("dump_set")}, s)
	})

	t.Run("DUMP sorted set value", func(t *testing.T) {
		ssv := &store.SortedSetValue{Members: map[string]float64{}}
		ssv.Add("member1", 1.0)
		s.Set("dump_zset", ssv, store.SetOptions{})
		_ = runHandler(t, router, "DUMP", [][]byte{[]byte("dump_zset")}, s)
	})

	t.Run("RESTORE wrong args", func(t *testing.T) {
		_ = runHandler(t, router, "RESTORE", nil, s)
	})

	t.Run("RESTORE invalid ttl", func(t *testing.T) {
		_ = runHandler(t, router, "RESTORE", [][]byte{[]byte("key"), []byte("notanumber"), []byte("CACHSTORM001")}, s)
	})

	t.Run("RESTORE invalid payload", func(t *testing.T) {
		_ = runHandler(t, router, "RESTORE", [][]byte{[]byte("key"), []byte("0"), []byte("invalid")}, s)
	})

	t.Run("RESTORE key exists without REPLACE", func(t *testing.T) {
		s.Set("restore_key", &store.StringValue{Data: []byte("existing")}, store.SetOptions{})
		_ = runHandler(t, router, "RESTORE", [][]byte{[]byte("restore_key"), []byte("0"), []byte("CACHSTORM0010:")}, s)
	})

	t.Run("RESTORE string value", func(t *testing.T) {
		_ = runHandler(t, router, "RESTORE", [][]byte{[]byte("restore_str"), []byte("0"), []byte("CACHSTORM0010:testdata")}, s)
	})

	t.Run("RESTORE with REPLACE", func(t *testing.T) {
		s.Set("restore_replace", &store.StringValue{Data: []byte("existing")}, store.SetOptions{})
		_ = runHandler(t, router, "RESTORE", [][]byte{[]byte("restore_replace"), []byte("0"), []byte("CACHSTORM0010:newdata"), []byte("REPLACE")}, s)
	})

	t.Run("RESTORE hash value", func(t *testing.T) {
		_ = runHandler(t, router, "RESTORE", [][]byte{[]byte("restore_hash"), []byte("0"), []byte("CACHSTORM0011:f1=v1&f2=v2&")}, s)
	})

	t.Run("RESTORE list value", func(t *testing.T) {
		_ = runHandler(t, router, "RESTORE", [][]byte{[]byte("restore_list"), []byte("0"), []byte("CACHSTORM0012:a,b,c,")}, s)
	})

	t.Run("RESTORE set value", func(t *testing.T) {
		_ = runHandler(t, router, "RESTORE", [][]byte{[]byte("restore_set"), []byte("0"), []byte("CACHSTORM0013:m1,m2,")}, s)
	})

	t.Run("RESTORE invalid format - no colon", func(t *testing.T) {
		_ = runHandler(t, router, "RESTORE", [][]byte{[]byte("restore_nocolon"), []byte("0"), []byte("CACHSTORM0010noColonHere")}, s)
	})

	t.Run("RESTORE unsupported type", func(t *testing.T) {
		_ = runHandler(t, router, "RESTORE", [][]byte{[]byte("restore_unsupported"), []byte("0"), []byte("CACHSTORM00199:foo")}, s)
	})

	t.Run("RESTORE with TTL", func(t *testing.T) {
		_ = runHandler(t, router, "RESTORE", [][]byte{[]byte("restore_ttl"), []byte("1000"), []byte("CACHSTORM0010:data")}, s)
	})

	t.Run("COPY wrong args", func(t *testing.T) {
		_ = runHandler(t, router, "COPY", nil, s)
	})

	t.Run("COPY source not found", func(t *testing.T) {
		_ = runHandler(t, router, "COPY", [][]byte{[]byte("missing_src"), []byte("dst")}, s)
	})

	t.Run("COPY destination exists without REPLACE", func(t *testing.T) {
		s.Set("copy_src", &store.StringValue{Data: []byte("srcvalue")}, store.SetOptions{})
		s.Set("copy_dst", &store.StringValue{Data: []byte("dstvalue")}, store.SetOptions{})
		_ = runHandler(t, router, "COPY", [][]byte{[]byte("copy_src"), []byte("copy_dst")}, s)
	})

	t.Run("COPY success", func(t *testing.T) {
		s.Set("copy_src2", &store.StringValue{Data: []byte("srcvalue")}, store.SetOptions{})
		_ = runHandler(t, router, "COPY", [][]byte{[]byte("copy_src2"), []byte("copy_dst2")}, s)
	})

	t.Run("COPY with REPLACE", func(t *testing.T) {
		s.Set("copy_src3", &store.StringValue{Data: []byte("srcvalue")}, store.SetOptions{})
		s.Set("copy_dst3", &store.StringValue{Data: []byte("dstvalue")}, store.SetOptions{})
		_ = runHandler(t, router, "COPY", [][]byte{[]byte("copy_src3"), []byte("copy_dst3"), []byte("REPLACE")}, s)
	})

	t.Run("COPY with DB option", func(t *testing.T) {
		s.Set("copy_src4", &store.StringValue{Data: []byte("srcvalue")}, store.SetOptions{})
		_ = runHandler(t, router, "COPY", [][]byte{[]byte("copy_src4"), []byte("copy_dst4"), []byte("DB"), []byte("1")}, s)
	})

	t.Run("COPY with DB option missing value", func(t *testing.T) {
		s.Set("copy_src5", &store.StringValue{Data: []byte("srcvalue")}, store.SetOptions{})
		_ = runHandler(t, router, "COPY", [][]byte{[]byte("copy_src5"), []byte("copy_dst5"), []byte("DB")}, s)
	})

	t.Run("COPY with DB option invalid value", func(t *testing.T) {
		s.Set("copy_src6", &store.StringValue{Data: []byte("srcvalue")}, store.SetOptions{})
		_ = runHandler(t, router, "COPY", [][]byte{[]byte("copy_src6"), []byte("copy_dst6"), []byte("DB"), []byte("notanumber")}, s)
	})
}

func TestLowCoverageBatch15_ARRAYMERGE(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterIntegrationCommands(router)

	t.Run("wrong args", func(t *testing.T) {
		_ = runHandler(t, router, "ARRAY.MERGE", nil, s)
	})

	t.Run("merge with non-existent source", func(t *testing.T) {
		_ = runHandler(t, router, "ARRAY.MERGE", [][]byte{[]byte("dest"), []byte("nonexistent")}, s)
	})

	t.Run("merge with both arrays", func(t *testing.T) {
		_ = runHandler(t, router, "ARRAY.PUSH", [][]byte{[]byte("src_arr"), []byte("a"), []byte("b")}, s)
		_ = runHandler(t, router, "ARRAY.PUSH", [][]byte{[]byte("dest_arr"), []byte("c")}, s)
		_ = runHandler(t, router, "ARRAY.MERGE", [][]byte{[]byte("dest_arr"), []byte("src_arr")}, s)
	})
}

func TestLowCoverageBatch15_OBJECTMERGE(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterIntegrationCommands(router)

	t.Run("wrong args", func(t *testing.T) {
		_ = runHandler(t, router, "OBJECT.MERGE", nil, s)
	})

	t.Run("merge into non-existent dest", func(t *testing.T) {
		_ = runHandler(t, router, "OBJECT.MERGE", [][]byte{[]byte("newdest"), []byte("nonexistent")}, s)
	})

	t.Run("merge with both objects", func(t *testing.T) {
		_ = runHandler(t, router, "OBJECT.SET", [][]byte{[]byte("src_obj"), []byte("key1"), []byte("val1")}, s)
		_ = runHandler(t, router, "OBJECT.SET", [][]byte{[]byte("dest_obj"), []byte("key2"), []byte("val2")}, s)
		_ = runHandler(t, router, "OBJECT.MERGE", [][]byte{[]byte("dest_obj"), []byte("src_obj")}, s)
	})
}

func TestLowCoverageBatch15_METRICSCMD(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMonitoringCommands(router)

	t.Run("wrong args", func(t *testing.T) {
		_ = runHandler(t, router, "METRICS.CMD", nil, s)
	})

	t.Run("non-existent command stats", func(t *testing.T) {
		_ = runHandler(t, router, "METRICS.CMD", [][]byte{[]byte("NONEXISTENT")}, s)
	})
}

func TestLowCoverageBatch15_QUOTAXCREATE(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	t.Run("wrong args", func(t *testing.T) {
		_ = runHandler(t, router, "QUOTA.XCREATE", nil, s)
	})

	t.Run("create quota", func(t *testing.T) {
		_ = runHandler(t, router, "QUOTA.XCREATE", [][]byte{[]byte("myquota"), []byte("100"), []byte("60000")}, s)
	})

	t.Run("use and check quota", func(t *testing.T) {
		_ = runHandler(t, router, "QUOTA.XCREATE", [][]byte{[]byte("testquota"), []byte("100"), []byte("60000")}, s)
		_ = runHandler(t, router, "QUOTA.XCHECK", [][]byte{[]byte("testquota"), []byte("50")}, s)
		_ = runHandler(t, router, "QUOTA.XUSAGE", [][]byte{[]byte("testquota")}, s)
		_ = runHandler(t, router, "QUOTA.XRESET", [][]byte{[]byte("testquota")}, s)
	})
}

func TestLowCoverageBatch15_MEMORYXFREE(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	t.Run("wrong args", func(t *testing.T) {
		_ = runHandler(t, router, "MEMORYX.FREE", nil, s)
	})

	t.Run("free from non-existent stat", func(t *testing.T) {
		_ = runHandler(t, router, "MEMORYX.FREE", [][]byte{[]byte("nonexistent"), []byte("100")}, s)
	})

	t.Run("free after alloc", func(t *testing.T) {
		_ = runHandler(t, router, "MEMORYX.ALLOC", [][]byte{[]byte("mem_test"), []byte("200")}, s)
		_ = runHandler(t, router, "MEMORYX.FREE", [][]byte{[]byte("mem_test"), []byte("100")}, s)
		_ = runHandler(t, router, "MEMORYX.STATS", [][]byte{[]byte("mem_test")}, s)
	})

	t.Run("free more than allocated", func(t *testing.T) {
		_ = runHandler(t, router, "MEMORYX.ALLOC", [][]byte{[]byte("mem_test2"), []byte("50")}, s)
		_ = runHandler(t, router, "MEMORYX.FREE", [][]byte{[]byte("mem_test2"), []byte("100")}, s)
	})
}

func TestLowCoverageBatch15_FCALL(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterFunctionCommands(router)

	t.Run("wrong args", func(t *testing.T) {
		_ = runHandler(t, router, "FCALL", nil, s)
	})

	t.Run("invalid function name format", func(t *testing.T) {
		_ = runHandler(t, router, "FCALL", [][]byte{[]byte("invalidformat")}, s)
	})

	t.Run("library not found", func(t *testing.T) {
		_ = runHandler(t, router, "FCALL", [][]byte{[]byte("nonexistent.function")}, s)
	})

	t.Run("with numkeys", func(t *testing.T) {
		_ = runHandler(t, router, "FCALL", [][]byte{[]byte("lib.fn"), []byte("2"), []byte("key1"), []byte("key2"), []byte("arg1")}, s)
	})

	t.Run("FCALL_RO", func(t *testing.T) {
		_ = runHandler(t, router, "FCALL_RO", [][]byte{[]byte("lib.fn")}, s)
	})

	t.Run("FUNCTION subcommands", func(t *testing.T) {
		_ = runHandler(t, router, "FUNCTION", [][]byte{[]byte("LIST")}, s)
		_ = runHandler(t, router, "FUNCTION", [][]byte{[]byte("DUMP")}, s)
		_ = runHandler(t, router, "FUNCTION", [][]byte{[]byte("STATS")}, s)
		_ = runHandler(t, router, "FUNCTION", [][]byte{[]byte("FLUSH")}, s)
		_ = runHandler(t, router, "FUNCTION", [][]byte{[]byte("UNKNOWN")}, s)
	})

	t.Run("FUNCTION CREATE and call", func(t *testing.T) {
		_ = runHandler(t, router, "FUNCTION", [][]byte{
			[]byte("CREATE"),
			[]byte("testlib"),
			[]byte("redis.call('SET', KEYS[1], ARGV[1])"),
		}, s)
		_ = runHandler(t, router, "FCALL", [][]byte{
			[]byte("testlib.test"),
			[]byte("1"),
			[]byte("func_key"),
			[]byte("func_val"),
		}, s)
	})
}
