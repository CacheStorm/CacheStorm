package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestLowCoverageBatch16_CallFunction(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterFunctionCommands(router)

	t.Run("create library with function and call", func(t *testing.T) {
		_ = runHandler(t, router, "FUNCTION", [][]byte{
			[]byte("CREATE"),
			[]byte("mathlib"),
			[]byte("redis.add = function(a, b) return tonumber(a) + tonumber(b) end"),
		}, s)

		_ = runHandler(t, router, "FCALL", [][]byte{
			[]byte("mathlib.add"),
			[]byte("0"),
			[]byte("10"),
			[]byte("20"),
		}, s)
	})

	t.Run("create library with function using keys", func(t *testing.T) {
		_ = runHandler(t, router, "FUNCTION", [][]byte{
			[]byte("CREATE"),
			[]byte("keylib"),
			[]byte("redis.getkey = function() return KEYS[1] end"),
		}, s)

		_ = runHandler(t, router, "FCALL", [][]byte{
			[]byte("keylib.getkey"),
			[]byte("1"),
			[]byte("mykey"),
		}, s)
	})

	t.Run("create library with function using argv", func(t *testing.T) {
		_ = runHandler(t, router, "FUNCTION", [][]byte{
			[]byte("CREATE"),
			[]byte("argvlib"),
			[]byte("redis.getargv = function() return ARGV[1] end"),
		}, s)

		_ = runHandler(t, router, "FCALL", [][]byte{
			[]byte("argvlib.getargv"),
			[]byte("0"),
			[]byte("myarg"),
		}, s)
	})

	t.Run("FCALL_RO same as FCALL", func(t *testing.T) {
		_ = runHandler(t, router, "FUNCTION", [][]byte{
			[]byte("CREATE"),
			[]byte("rolib"),
			[]byte("redis.rotest = function() return 'ok' end"),
		}, s)

		_ = runHandler(t, router, "FCALL_RO", [][]byte{
			[]byte("rolib.rotest"),
			[]byte("0"),
		}, s)
	})
}

func TestLowCoverageBatch16_ArrayMerge(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterIntegrationCommands(router)

	t.Run("merge into existing dest", func(t *testing.T) {
		_ = runHandler(t, router, "ARRAY.PUSH", [][]byte{[]byte("dest_arr"), []byte("a"), []byte("b")}, s)
		_ = runHandler(t, router, "ARRAY.PUSH", [][]byte{[]byte("src_arr"), []byte("c"), []byte("d")}, s)
		_ = runHandler(t, router, "ARRAY.MERGE", [][]byte{[]byte("dest_arr"), []byte("src_arr")}, s)
	})

	t.Run("merge with empty dest", func(t *testing.T) {
		_ = runHandler(t, router, "ARRAY.PUSH", [][]byte{[]byte("src_arr2"), []byte("x"), []byte("y")}, s)
		_ = runHandler(t, router, "ARRAY.MERGE", [][]byte{[]byte("empty_dest"), []byte("src_arr2")}, s)
	})
}

func TestLowCoverageBatch16_ObjectMerge(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterIntegrationCommands(router)

	t.Run("merge into existing dest", func(t *testing.T) {
		_ = runHandler(t, router, "OBJECT.SET", [][]byte{[]byte("dest_obj"), []byte("k1"), []byte("v1")}, s)
		_ = runHandler(t, router, "OBJECT.SET", [][]byte{[]byte("src_obj"), []byte("k2"), []byte("v2")}, s)
		_ = runHandler(t, router, "OBJECT.MERGE", [][]byte{[]byte("dest_obj"), []byte("src_obj")}, s)
	})

	t.Run("merge with existing key in dest", func(t *testing.T) {
		_ = runHandler(t, router, "OBJECT.SET", [][]byte{[]byte("dest_obj2"), []byte("k1"), []byte("old")}, s)
		_ = runHandler(t, router, "OBJECT.SET", [][]byte{[]byte("src_obj2"), []byte("k1"), []byte("new")}, s)
		_ = runHandler(t, router, "OBJECT.MERGE", [][]byte{[]byte("dest_obj2"), []byte("src_obj2")}, s)
	})
}

func TestLowCoverageBatch16_QuotaXCreate(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	t.Run("create and use quota", func(t *testing.T) {
		_ = runHandler(t, router, "QUOTAX.CREATE", [][]byte{[]byte("quota1"), []byte("100"), []byte("60000")}, s)
		_ = runHandler(t, router, "QUOTAX.CHECK", [][]byte{[]byte("quota1"), []byte("30")}, s)
		_ = runHandler(t, router, "QUOTAX.CHECK", [][]byte{[]byte("quota1"), []byte("50")}, s)
		_ = runHandler(t, router, "QUOTAX.USAGE", [][]byte{[]byte("quota1")}, s)
	})

	t.Run("quota exceeded", func(t *testing.T) {
		_ = runHandler(t, router, "QUOTAX.CREATE", [][]byte{[]byte("quota2"), []byte("50"), []byte("60000")}, s)
		_ = runHandler(t, router, "QUOTAX.CHECK", [][]byte{[]byte("quota2"), []byte("30")}, s)
		_ = runHandler(t, router, "QUOTAX.CHECK", [][]byte{[]byte("quota2"), []byte("100")}, s)
	})

	t.Run("check non-existent quota", func(t *testing.T) {
		_ = runHandler(t, router, "QUOTAX.CHECK", [][]byte{[]byte("nonexistent"), []byte("10")}, s)
	})

	t.Run("usage non-existent quota", func(t *testing.T) {
		_ = runHandler(t, router, "QUOTAX.USAGE", [][]byte{[]byte("nonexistent")}, s)
	})

	t.Run("reset quota", func(t *testing.T) {
		_ = runHandler(t, router, "QUOTAX.CREATE", [][]byte{[]byte("quota3"), []byte("100"), []byte("60000")}, s)
		_ = runHandler(t, router, "QUOTAX.CHECK", [][]byte{[]byte("quota3"), []byte("30")}, s)
		_ = runHandler(t, router, "QUOTAX.RESET", [][]byte{[]byte("quota3")}, s)
		_ = runHandler(t, router, "QUOTAX.USAGE", [][]byte{[]byte("quota3")}, s)
	})
}

func TestLowCoverageBatch16_SpatialWithin(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMVCCCommands(router)

	t.Run("SPATIAL.WITHIN wrong args", func(t *testing.T) {
		_ = runHandler(t, router, "SPATIAL.WITHIN", nil, s)
	})

	t.Run("SPATIAL.WITHIN non-existent index", func(t *testing.T) {
		_ = runHandler(t, router, "SPATIAL.WITHIN", [][]byte{
			[]byte("nonexistent"),
			[]byte("0.0"),
			[]byte("0.0"),
			[]byte("1000"),
		}, s)
	})
}

func TestLowCoverageBatch16_MetricsCmd(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMonitoringCommands(router)

	t.Run("METRICS.CMD with valid command", func(t *testing.T) {
		_ = runHandler(t, router, "METRICS.CMD", [][]byte{[]byte("GET")}, s)
	})

	t.Run("METRICS.CMD with invalid command", func(t *testing.T) {
		_ = runHandler(t, router, "METRICS.CMD", [][]byte{[]byte("INVALID")}, s)
	})
}

func TestLowCoverageBatch16_MemoryXFree(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	t.Run("free after alloc", func(t *testing.T) {
		_ = runHandler(t, router, "MEMORYX.ALLOC", [][]byte{[]byte("mem1"), []byte("100")}, s)
		_ = runHandler(t, router, "MEMORYX.FREE", [][]byte{[]byte("mem1"), []byte("50")}, s)
		_ = runHandler(t, router, "MEMORYX.STATS", [][]byte{[]byte("mem1")}, s)
	})

	t.Run("free all", func(t *testing.T) {
		_ = runHandler(t, router, "MEMORYX.ALLOC", [][]byte{[]byte("mem2"), []byte("100")}, s)
		_ = runHandler(t, router, "MEMORYX.FREE", [][]byte{[]byte("mem2"), []byte("100")}, s)
		_ = runHandler(t, router, "MEMORYX.STATS", [][]byte{[]byte("mem2")}, s)
	})

	t.Run("free from non-existent", func(t *testing.T) {
		_ = runHandler(t, router, "MEMORYX.FREE", [][]byte{[]byte("nonexistent"), []byte("50")}, s)
	})
}

func TestLowCoverageBatch16_EncodingCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterEncodingCommands(router)

	t.Run("TOML.ENCODE basic", func(t *testing.T) {
		_ = runHandler(t, router, "TOML.ENCODE", [][]byte{[]byte("key"), []byte("value")}, s)
	})

	t.Run("TOML.DECODE basic", func(t *testing.T) {
		_ = runHandler(t, router, "TOML.DECODE", [][]byte{[]byte("key = \"value\"")}, s)
	})

	t.Run("CBOR.ENCODE basic", func(t *testing.T) {
		_ = runHandler(t, router, "CBOR.ENCODE", [][]byte{[]byte("data")}, s)
	})

	t.Run("CBOR.DECODE basic", func(t *testing.T) {
		_ = runHandler(t, router, "CBOR.DECODE", [][]byte{[]byte("data")}, s)
	})
}

func TestLowCoverageBatch16_DigestCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterDigestCommands(router)

	t.Run("BASE64ENCODE basic", func(t *testing.T) {
		_ = runHandler(t, router, "BASE64ENCODE", [][]byte{[]byte("Hello World")}, s)
	})

	t.Run("BASE64DECODE basic", func(t *testing.T) {
		_ = runHandler(t, router, "BASE64DECODE", [][]byte{[]byte("SGVsbG8gV29ybGQ=")}, s)
	})

	t.Run("BASE64DECODE invalid", func(t *testing.T) {
		_ = runHandler(t, router, "BASE64DECODE", [][]byte{[]byte("invalid!!")}, s)
	})
}

func TestLowCoverageBatch16_SchedulerCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterSchedulerCommands(router)

	t.Run("CIRCUIT.STATS non-existent", func(t *testing.T) {
		_ = runHandler(t, router, "CIRCUIT.STATS", [][]byte{[]byte("nonexistent")}, s)
	})
}

func TestLowCoverageBatch16_StreamCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStreamCommands(router)

	t.Run("XAUTOCLAIM wrong args", func(t *testing.T) {
		_ = runHandler(t, router, "XAUTOCLAIM", nil, s)
	})

	t.Run("XAUTOCLAIM non-existent stream", func(t *testing.T) {
		_ = runHandler(t, router, "XAUTOCLAIM", [][]byte{
			[]byte("mystream"),
			[]byte("mygroup"),
			[]byte("consumer1"),
			[]byte("0"),
			[]byte("10"),
		}, s)
	})
}

func TestLowCoverageBatch16_NamespaceCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterNamespaceCommands(router)

	t.Run("NSINFO non-existent", func(t *testing.T) {
		_ = runHandler(t, router, "NSINFO", [][]byte{[]byte("nonexistent")}, s)
	})
}

func TestLowCoverageBatch16_ClientTracking(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterClientCommands(router)

	t.Run("CLIENT TRACKING on", func(t *testing.T) {
		_ = runHandler(t, router, "CLIENT", [][]byte{[]byte("TRACKING"), []byte("ON")}, s)
	})

	t.Run("CLIENT TRACKING off", func(t *testing.T) {
		_ = runHandler(t, router, "CLIENT", [][]byte{[]byte("TRACKING"), []byte("OFF")}, s)
	})

	t.Run("CLIENT TRACKING with noloop", func(t *testing.T) {
		_ = runHandler(t, router, "CLIENT", [][]byte{[]byte("TRACKING"), []byte("ON"), []byte("NOLOOP")}, s)
	})

	t.Run("CLIENT TRACKING with bcast", func(t *testing.T) {
		_ = runHandler(t, router, "CLIENT", [][]byte{[]byte("TRACKING"), []byte("ON"), []byte("BCAST")}, s)
	})

	t.Run("CLIENT TRACKING with prefixes", func(t *testing.T) {
		_ = runHandler(t, router, "CLIENT", [][]byte{[]byte("TRACKING"), []byte("ON"), []byte("BCAST"), []byte("PREFIX"), []byte("user:")}, s)
	})

	t.Run("CLIENT TRACKING with redirect", func(t *testing.T) {
		_ = runHandler(t, router, "CLIENT", [][]byte{[]byte("TRACKING"), []byte("ON"), []byte("REDIRECT"), []byte("123")}, s)
	})

	t.Run("CLIENT TRACKING with redirect invalid", func(t *testing.T) {
		_ = runHandler(t, router, "CLIENT", [][]byte{[]byte("TRACKING"), []byte("ON"), []byte("REDIRECT"), []byte("notanumber")}, s)
	})

	t.Run("CLIENT TRACKING invalid option", func(t *testing.T) {
		_ = runHandler(t, router, "CLIENT", [][]byte{[]byte("TRACKING"), []byte("INVALID")}, s)
	})

	t.Run("CLIENT TRACKING wrong args", func(t *testing.T) {
		_ = runHandler(t, router, "CLIENT", [][]byte{[]byte("TRACKING")}, s)
	})
}

func TestLowCoverageBatch16_BackupRestore(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterUtilityExtCommands(router)

	t.Run("BACKUP.RESTORE wrong args", func(t *testing.T) {
		_ = runHandler(t, router, "BACKUP.RESTORE", nil, s)
	})

	t.Run("BACKUP.RESTORE non-existent file", func(t *testing.T) {
		_ = runHandler(t, router, "BACKUP.RESTORE", [][]byte{[]byte("/nonexistent/backup.rdb")}, s)
	})
}

func TestLowCoverageBatch16_TemplateCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterTemplateCommands(router)

	t.Run("matchRegex basic", func(t *testing.T) {
		_ = runHandler(t, router, "TEMPLATE.MATCH", [][]byte{[]byte("hello.*"), []byte("hello world")}, s)
	})

	t.Run("TEMPLATE.RENDER basic", func(t *testing.T) {
		_ = runHandler(t, router, "TEMPLATE.RENDER", [][]byte{[]byte("Hello {{.Name}}"), []byte("{\"Name\":\"World\"}")}, s)
	})
}
