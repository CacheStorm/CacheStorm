package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestLowCoverageBatch14_KeyObjectSubcommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterCacheWarmingCommands(router)

	t.Run("KEY.OBJECT ENCODING string", func(t *testing.T) {
		s.Set("ko_enc_str", &store.StringValue{Data: []byte("value")}, store.SetOptions{})
		_ = runHandler(t, router, "KEY.OBJECT", [][]byte{[]byte("ENCODING"), []byte("ko_enc_str")}, s)
	})

	t.Run("KEY.OBJECT ENCODING hash", func(t *testing.T) {
		hv := &store.HashValue{Fields: map[string][]byte{}}
		hv.Fields["f1"] = []byte("v1")
		s.Set("ko_enc_hash", hv, store.SetOptions{})
		_ = runHandler(t, router, "KEY.OBJECT", [][]byte{[]byte("ENCODING"), []byte("ko_enc_hash")}, s)
	})

	t.Run("KEY.OBJECT ENCODING list", func(t *testing.T) {
		lv := &store.ListValue{Elements: [][]byte{[]byte("a")}}
		s.Set("ko_enc_list", lv, store.SetOptions{})
		_ = runHandler(t, router, "KEY.OBJECT", [][]byte{[]byte("ENCODING"), []byte("ko_enc_list")}, s)
	})

	t.Run("KEY.OBJECT ENCODING set", func(t *testing.T) {
		sv := &store.SetValue{Members: map[string]struct{}{}}
		sv.Members["m1"] = struct{}{}
		s.Set("ko_enc_set", sv, store.SetOptions{})
		_ = runHandler(t, router, "KEY.OBJECT", [][]byte{[]byte("ENCODING"), []byte("ko_enc_set")}, s)
	})

	t.Run("KEY.OBJECT ENCODING zset", func(t *testing.T) {
		ssv := &store.SortedSetValue{Members: map[string]float64{}}
		ssv.Add("m1", 1.0)
		s.Set("ko_enc_zset", ssv, store.SetOptions{})
		_ = runHandler(t, router, "KEY.OBJECT", [][]byte{[]byte("ENCODING"), []byte("ko_enc_zset")}, s)
	})

	t.Run("KEY.OBJECT IDLETIME", func(t *testing.T) {
		s.Set("ko_idle", &store.StringValue{Data: []byte("value")}, store.SetOptions{})
		_ = runHandler(t, router, "KEY.OBJECT", [][]byte{[]byte("IDLETIME"), []byte("ko_idle")}, s)
	})

	t.Run("KEY.OBJECT REFCOUNT", func(t *testing.T) {
		s.Set("ko_ref", &store.StringValue{Data: []byte("value")}, store.SetOptions{})
		_ = runHandler(t, router, "KEY.OBJECT", [][]byte{[]byte("REFCOUNT"), []byte("ko_ref")}, s)
	})

	t.Run("KEY.OBJECT FREQ", func(t *testing.T) {
		s.Set("ko_freq", &store.StringValue{Data: []byte("value")}, store.SetOptions{})
		_ = runHandler(t, router, "KEY.OBJECT", [][]byte{[]byte("FREQ"), []byte("ko_freq")}, s)
	})

	t.Run("KEY.OBJECT TYPE", func(t *testing.T) {
		s.Set("ko_type", &store.StringValue{Data: []byte("value")}, store.SetOptions{})
		_ = runHandler(t, router, "KEY.OBJECT", [][]byte{[]byte("TYPE"), []byte("ko_type")}, s)
	})

	t.Run("KEY.OBJECT unknown subcommand", func(t *testing.T) {
		s.Set("ko_unknown", &store.StringValue{Data: []byte("value")}, store.SetOptions{})
		_ = runHandler(t, router, "KEY.OBJECT", [][]byte{[]byte("UNKNOWN"), []byte("ko_unknown")}, s)
	})

	t.Run("KEY.OBJECT non-existent key", func(t *testing.T) {
		_ = runHandler(t, router, "KEY.OBJECT", [][]byte{[]byte("TYPE"), []byte("nonexistent")}, s)
	})

	t.Run("KEY.OBJECT wrong args", func(t *testing.T) {
		_ = runHandler(t, router, "KEY.OBJECT", nil, s)
	})
}

func TestLowCoverageBatch14_FunctionCallFunction(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterFunctionCommands(router)

	t.Run("FUNCTION.LOAD and FCALL various", func(t *testing.T) {
		_ = runHandler(t, router, "FUNCTION.LOAD", [][]byte{
			[]byte("math_func"),
			[]byte("Lua"),
			[]byte("return tonumber(ARGV[1]) + tonumber(ARGV[2])"),
		}, s)
		_ = runHandler(t, router, "FCALL", [][]byte{
			[]byte("math_func"),
			[]byte("0"),
			[]byte("10"),
			[]byte("20"),
		}, s)
	})

	t.Run("FUNCTION.LOAD with keys", func(t *testing.T) {
		s.Set("func_key1", &store.StringValue{Data: []byte("hello")}, store.SetOptions{})
		_ = runHandler(t, router, "FUNCTION.LOAD", [][]byte{
			[]byte("get_func"),
			[]byte("Lua"),
			[]byte("return redis.call('GET', KEYS[1])"),
		}, s)
		_ = runHandler(t, router, "FCALL", [][]byte{
			[]byte("get_func"),
			[]byte("1"),
			[]byte("func_key1"),
		}, s)
	})

	t.Run("FUNCTION.LOAD with hash operations", func(t *testing.T) {
		hv := &store.HashValue{Fields: map[string][]byte{}}
		hv.Fields["field1"] = []byte("value1")
		s.Set("func_hash", hv, store.SetOptions{})
		_ = runHandler(t, router, "FUNCTION.LOAD", [][]byte{
			[]byte("hget_func"),
			[]byte("Lua"),
			[]byte("return redis.call('HGET', KEYS[1], ARGV[1])"),
		}, s)
		_ = runHandler(t, router, "FCALL", [][]byte{
			[]byte("hget_func"),
			[]byte("1"),
			[]byte("func_hash"),
			[]byte("field1"),
		}, s)
	})

	t.Run("FUNCTION.LOAD with list operations", func(t *testing.T) {
		lv := &store.ListValue{Elements: [][]byte{[]byte("a"), []byte("b")}}
		s.Set("func_list", lv, store.SetOptions{})
		_ = runHandler(t, router, "FUNCTION.LOAD", [][]byte{
			[]byte("lrange_func"),
			[]byte("Lua"),
			[]byte("return redis.call('LRANGE', KEYS[1], 0, -1)"),
		}, s)
		_ = runHandler(t, router, "FCALL", [][]byte{
			[]byte("lrange_func"),
			[]byte("1"),
			[]byte("func_list"),
		}, s)
	})
}

func TestLowCoverageBatch14_ArrayMerge(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterIntegrationCommands(router)

	t.Run("ARRAY.MERGE with concat mode", func(t *testing.T) {
		_ = runHandler(t, router, "ARRAY.CREATE", [][]byte{[]byte("merge_dst")}, s)
		_ = runHandler(t, router, "ARRAY.CREATE", [][]byte{[]byte("merge_src")}, s)
		_ = runHandler(t, router, "ARRAY.PUSH", [][]byte{[]byte("merge_dst"), []byte("a")}, s)
		_ = runHandler(t, router, "ARRAY.PUSH", [][]byte{[]byte("merge_src"), []byte("b")}, s)
		_ = runHandler(t, router, "ARRAY.MERGE", [][]byte{[]byte("merge_dst"), []byte("merge_src"), []byte("concat")}, s)
	})

	t.Run("ARRAY.MERGE with union mode", func(t *testing.T) {
		_ = runHandler(t, router, "ARRAY.CREATE", [][]byte{[]byte("union_dst")}, s)
		_ = runHandler(t, router, "ARRAY.CREATE", [][]byte{[]byte("union_src")}, s)
		_ = runHandler(t, router, "ARRAY.PUSH", [][]byte{[]byte("union_dst"), []byte("a")}, s)
		_ = runHandler(t, router, "ARRAY.PUSH", [][]byte{[]byte("union_src"), []byte("b")}, s)
		_ = runHandler(t, router, "ARRAY.MERGE", [][]byte{[]byte("union_dst"), []byte("union_src"), []byte("union")}, s)
	})

	t.Run("ARRAY.MERGE with replace mode", func(t *testing.T) {
		_ = runHandler(t, router, "ARRAY.CREATE", [][]byte{[]byte("replace_dst")}, s)
		_ = runHandler(t, router, "ARRAY.CREATE", [][]byte{[]byte("replace_src")}, s)
		_ = runHandler(t, router, "ARRAY.PUSH", [][]byte{[]byte("replace_dst"), []byte("a")}, s)
		_ = runHandler(t, router, "ARRAY.PUSH", [][]byte{[]byte("replace_src"), []byte("b")}, s)
		_ = runHandler(t, router, "ARRAY.MERGE", [][]byte{[]byte("replace_dst"), []byte("replace_src"), []byte("replace")}, s)
	})
}

func TestLowCoverageBatch14_ObjectMerge(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterIntegrationCommands(router)

	t.Run("OBJECT.MERGE with shallow mode", func(t *testing.T) {
		_ = runHandler(t, router, "OBJECT.CREATE", [][]byte{[]byte("obj_merge_dst")}, s)
		_ = runHandler(t, router, "OBJECT.CREATE", [][]byte{[]byte("obj_merge_src")}, s)
		_ = runHandler(t, router, "OBJECT.SET", [][]byte{[]byte("obj_merge_dst"), []byte("a"), []byte("1")}, s)
		_ = runHandler(t, router, "OBJECT.SET", [][]byte{[]byte("obj_merge_src"), []byte("b"), []byte("2")}, s)
		_ = runHandler(t, router, "OBJECT.MERGE", [][]byte{[]byte("obj_merge_dst"), []byte("obj_merge_src"), []byte("shallow")}, s)
	})

	t.Run("OBJECT.MERGE with deep mode", func(t *testing.T) {
		_ = runHandler(t, router, "OBJECT.CREATE", [][]byte{[]byte("obj_deep_dst")}, s)
		_ = runHandler(t, router, "OBJECT.CREATE", [][]byte{[]byte("obj_deep_src")}, s)
		_ = runHandler(t, router, "OBJECT.SET", [][]byte{[]byte("obj_deep_dst"), []byte("a"), []byte("1")}, s)
		_ = runHandler(t, router, "OBJECT.SET", [][]byte{[]byte("obj_deep_src"), []byte("b"), []byte("2")}, s)
		_ = runHandler(t, router, "OBJECT.MERGE", [][]byte{[]byte("obj_deep_dst"), []byte("obj_deep_src"), []byte("deep")}, s)
	})

	t.Run("OBJECT.MERGE with replace mode", func(t *testing.T) {
		_ = runHandler(t, router, "OBJECT.CREATE", [][]byte{[]byte("obj_replace_dst")}, s)
		_ = runHandler(t, router, "OBJECT.CREATE", [][]byte{[]byte("obj_replace_src")}, s)
		_ = runHandler(t, router, "OBJECT.SET", [][]byte{[]byte("obj_replace_dst"), []byte("a"), []byte("1")}, s)
		_ = runHandler(t, router, "OBJECT.SET", [][]byte{[]byte("obj_replace_src"), []byte("b"), []byte("2")}, s)
		_ = runHandler(t, router, "OBJECT.MERGE", [][]byte{[]byte("obj_replace_dst"), []byte("obj_replace_src"), []byte("replace")}, s)
	})
}

func TestLowCoverageBatch14_MetricsCommand(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMonitoringCommands(router)

	t.Run("METRICS with json format", func(t *testing.T) {
		_ = runHandler(t, router, "METRICS", [][]byte{[]byte("json")}, s)
	})

	t.Run("METRICS with text format", func(t *testing.T) {
		_ = runHandler(t, router, "METRICS", [][]byte{[]byte("text")}, s)
	})

	t.Run("METRICS with all section", func(t *testing.T) {
		_ = runHandler(t, router, "METRICS", [][]byte{[]byte("all")}, s)
	})

	t.Run("METRICS with memory section", func(t *testing.T) {
		_ = runHandler(t, router, "METRICS", [][]byte{[]byte("memory")}, s)
	})

	t.Run("METRICS with cpu section", func(t *testing.T) {
		_ = runHandler(t, router, "METRICS", [][]byte{[]byte("cpu")}, s)
	})
}

func TestLowCoverageBatch14_QuotaCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	t.Run("QUOTA.XCREATE with strict mode", func(t *testing.T) {
		_ = runHandler(t, router, "QUOTA.XCREATE", [][]byte{
			[]byte("quota_strict"),
			[]byte("1000"),
			[]byte("3600"),
			[]byte("strict"),
		}, s)
	})

	t.Run("QUOTA.XCREATE with soft mode", func(t *testing.T) {
		_ = runHandler(t, router, "QUOTA.XCREATE", [][]byte{
			[]byte("quota_soft"),
			[]byte("1000"),
			[]byte("3600"),
			[]byte("soft"),
		}, s)
	})

	t.Run("QUOTA.XCREATE with sliding window", func(t *testing.T) {
		_ = runHandler(t, router, "QUOTA.XCREATE", [][]byte{
			[]byte("quota_sliding"),
			[]byte("100"),
			[]byte("60"),
			[]byte("sliding"),
		}, s)
	})
}

func TestLowCoverageBatch14_SketchCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	t.Run("SKETCH.CREATE with error rate", func(t *testing.T) {
		_ = runHandler(t, router, "SKETCH.CREATE", [][]byte{
			[]byte("sketch_err"),
			[]byte("0.01"),
		}, s)
	})

	t.Run("SKETCH.CREATE with buckets", func(t *testing.T) {
		_ = runHandler(t, router, "SKETCH.CREATE", [][]byte{
			[]byte("sketch_buckets"),
			[]byte("0.01"),
			[]byte("buckets"),
			[]byte("1000"),
		}, s)
	})

	t.Run("SKETCH.UPDATE with count", func(t *testing.T) {
		_ = runHandler(t, router, "SKETCH.CREATE", [][]byte{[]byte("sketch_update")}, s)
		_ = runHandler(t, router, "SKETCH.UPDATE", [][]byte{
			[]byte("sketch_update"),
			[]byte("item1"),
			[]byte("5"),
		}, s)
	})

	t.Run("SKETCH.UPDATE with increment", func(t *testing.T) {
		_ = runHandler(t, router, "SKETCH.CREATE", [][]byte{[]byte("sketch_inc")}, s)
		_ = runHandler(t, router, "SKETCH.UPDATE", [][]byte{
			[]byte("sketch_inc"),
			[]byte("item1"),
			[]byte("increment"),
		}, s)
	})
}

func TestLowCoverageBatch14_SpatialCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMVCCCommands(router)

	t.Run("SPATIAL.CREATE with precision", func(t *testing.T) {
		_ = runHandler(t, router, "SPATIAL.CREATE", [][]byte{
			[]byte("spatial_prec"),
			[]byte("12"),
		}, s)
	})

	t.Run("SPATIAL.ADD with coordinates", func(t *testing.T) {
		_ = runHandler(t, router, "SPATIAL.CREATE", [][]byte{[]byte("spatial_add")}, s)
		_ = runHandler(t, router, "SPATIAL.ADD", [][]byte{
			[]byte("spatial_add"),
			[]byte("point1"),
			[]byte("13.361389"),
			[]byte("38.115556"),
		}, s)
	})

	t.Run("SPATIAL.WITHIN with radius", func(t *testing.T) {
		_ = runHandler(t, router, "SPATIAL.CREATE", [][]byte{[]byte("spatial_within")}, s)
		_ = runHandler(t, router, "SPATIAL.ADD", [][]byte{
			[]byte("spatial_within"),
			[]byte("point1"),
			[]byte("0"),
			[]byte("0"),
		}, s)
		_ = runHandler(t, router, "SPATIAL.WITHIN", [][]byte{
			[]byte("spatial_within"),
			[]byte("0"),
			[]byte("0"),
			[]byte("100"),
			[]byte("km"),
		}, s)
	})
}

func TestLowCoverageBatch14_RollupCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMVCCCommands(router)

	t.Run("ROLLUP.ADD with tags", func(t *testing.T) {
		_ = runHandler(t, router, "ROLLUP.ADD", [][]byte{
			[]byte("rollup_tags"),
			[]byte("100"),
			[]byte("hourly"),
			[]byte("api"),
			[]byte("production"),
		}, s)
	})

	t.Run("ROLLUP.GET with aggregation", func(t *testing.T) {
		_ = runHandler(t, router, "ROLLUP.ADD", [][]byte{
			[]byte("rollup_agg"),
			[]byte("100"),
			[]byte("hourly"),
		}, s)
		_ = runHandler(t, router, "ROLLUP.GET", [][]byte{
			[]byte("rollup_agg"),
			[]byte("sum"),
		}, s)
	})

	t.Run("ROLLUP.GET with avg", func(t *testing.T) {
		_ = runHandler(t, router, "ROLLUP.ADD", [][]byte{
			[]byte("rollup_avg"),
			[]byte("50"),
			[]byte("hourly"),
		}, s)
		_ = runHandler(t, router, "ROLLUP.GET", [][]byte{
			[]byte("rollup_avg"),
			[]byte("avg"),
		}, s)
	})
}

func TestLowCoverageBatch14_MemoryCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	t.Run("MEMORYX.FREE with force", func(t *testing.T) {
		_ = runHandler(t, router, "MEMORYX.FREE", [][]byte{[]byte("force")}, s)
	})

	t.Run("MEMORYX.FREE with all", func(t *testing.T) {
		_ = runHandler(t, router, "MEMORYX.FREE", [][]byte{[]byte("all")}, s)
	})

	t.Run("MEMORYX.STATS with detailed", func(t *testing.T) {
		_ = runHandler(t, router, "MEMORYX.STATS", [][]byte{[]byte("detailed")}, s)
	})
}

func TestLowCoverageBatch14_DumpCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterServerCommands(router)

	t.Run("DUMP string with ttl", func(t *testing.T) {
		s.Set("dump_ttl", &store.StringValue{Data: []byte("value")}, store.SetOptions{TTL: 60000})
		_ = runHandler(t, router, "DUMP", [][]byte{[]byte("dump_ttl")}, s)
	})

	t.Run("DUMP hash with multiple fields", func(t *testing.T) {
		hv := &store.HashValue{Fields: map[string][]byte{}}
		hv.Fields["f1"] = []byte("v1")
		hv.Fields["f2"] = []byte("v2")
		hv.Fields["f3"] = []byte("v3")
		s.Set("dump_hash_multi", hv, store.SetOptions{})
		_ = runHandler(t, router, "DUMP", [][]byte{[]byte("dump_hash_multi")}, s)
	})

	t.Run("DUMP list with multiple elements", func(t *testing.T) {
		lv := &store.ListValue{Elements: [][]byte{
			[]byte("a"),
			[]byte("b"),
			[]byte("c"),
			[]byte("d"),
		}}
		s.Set("dump_list_multi", lv, store.SetOptions{})
		_ = runHandler(t, router, "DUMP", [][]byte{[]byte("dump_list_multi")}, s)
	})

	t.Run("DUMP set with multiple members", func(t *testing.T) {
		sv := &store.SetValue{Members: map[string]struct{}{}}
		sv.Members["m1"] = struct{}{}
		sv.Members["m2"] = struct{}{}
		sv.Members["m3"] = struct{}{}
		s.Set("dump_set_multi", sv, store.SetOptions{})
		_ = runHandler(t, router, "DUMP", [][]byte{[]byte("dump_set_multi")}, s)
	})

	t.Run("DUMP zset with multiple members", func(t *testing.T) {
		ssv := &store.SortedSetValue{Members: map[string]float64{}}
		ssv.Add("m1", 1.0)
		ssv.Add("m2", 2.0)
		ssv.Add("m3", 3.0)
		s.Set("dump_zset_multi", ssv, store.SetOptions{})
		_ = runHandler(t, router, "DUMP", [][]byte{[]byte("dump_zset_multi")}, s)
	})
}

func TestLowCoverageBatch14_RestoreCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterServerCommands(router)

	t.Run("RESTORE string with ttl", func(t *testing.T) {
		_ = runHandler(t, router, "RESTORE", [][]byte{
			[]byte("restore_ttl"),
			[]byte("60000"),
			[]byte("CACHSTORM001" + string([]byte{0}) + "60000:restored_value"),
		}, s)
	})

	t.Run("RESTORE hash with fields", func(t *testing.T) {
		_ = runHandler(t, router, "RESTORE", [][]byte{
			[]byte("restore_hash_fields"),
			[]byte("0"),
			[]byte("CACHSTORM001" + string([]byte{1}) + "0:f1=v1&f2=v2&f3=v3&"),
		}, s)
	})

	t.Run("RESTORE list with elements", func(t *testing.T) {
		_ = runHandler(t, router, "RESTORE", [][]byte{
			[]byte("restore_list_elems"),
			[]byte("0"),
			[]byte("CACHSTORM001" + string([]byte{2}) + "0:a,b,c,d,e,"),
		}, s)
	})

	t.Run("RESTORE set with members", func(t *testing.T) {
		_ = runHandler(t, router, "RESTORE", [][]byte{
			[]byte("restore_set_members"),
			[]byte("0"),
			[]byte("CACHSTORM001" + string([]byte{3}) + "0:m1,m2,m3,m4,"),
		}, s)
	})

	t.Run("RESTORE with REPLACE", func(t *testing.T) {
		s.Set("restore_replace", &store.StringValue{Data: []byte("existing")}, store.SetOptions{})
		_ = runHandler(t, router, "RESTORE", [][]byte{
			[]byte("restore_replace"),
			[]byte("0"),
			[]byte("CACHSTORM001" + string([]byte{0}) + "0:new_value"),
			[]byte("REPLACE"),
		}, s)
	})

	t.Run("RESTORE with ABSTTL", func(t *testing.T) {
		_ = runHandler(t, router, "RESTORE", [][]byte{
			[]byte("restore_absttl"),
			[]byte("9999999999000"),
			[]byte("CACHSTORM001" + string([]byte{0}) + "9999999999000:value"),
			[]byte("ABSTTL"),
		}, s)
	})
}

func TestLowCoverageBatch14_CopyCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterServerCommands(router)

	t.Run("COPY with DB option", func(t *testing.T) {
		s.Set("copy_db_src", &store.StringValue{Data: []byte("value")}, store.SetOptions{})
		_ = runHandler(t, router, "COPY", [][]byte{
			[]byte("copy_db_src"),
			[]byte("copy_db_dst"),
			[]byte("DB"),
			[]byte("2"),
		}, s)
	})

	t.Run("COPY with REPLACE and DB", func(t *testing.T) {
		s.Set("copy_replace_db_src", &store.StringValue{Data: []byte("value")}, store.SetOptions{})
		s.Set("copy_replace_db_dst", &store.StringValue{Data: []byte("existing")}, store.SetOptions{})
		_ = runHandler(t, router, "COPY", [][]byte{
			[]byte("copy_replace_db_src"),
			[]byte("copy_replace_db_dst"),
			[]byte("REPLACE"),
			[]byte("DB"),
			[]byte("1"),
		}, s)
	})

	t.Run("COPY hash", func(t *testing.T) {
		hv := &store.HashValue{Fields: map[string][]byte{}}
		hv.Fields["f1"] = []byte("v1")
		s.Set("copy_hash_src", hv, store.SetOptions{})
		_ = runHandler(t, router, "COPY", [][]byte{
			[]byte("copy_hash_src"),
			[]byte("copy_hash_dst"),
		}, s)
	})

	t.Run("COPY list", func(t *testing.T) {
		lv := &store.ListValue{Elements: [][]byte{[]byte("a"), []byte("b")}}
		s.Set("copy_list_src", lv, store.SetOptions{})
		_ = runHandler(t, router, "COPY", [][]byte{
			[]byte("copy_list_src"),
			[]byte("copy_list_dst"),
		}, s)
	})

	t.Run("COPY set", func(t *testing.T) {
		sv := &store.SetValue{Members: map[string]struct{}{}}
		sv.Members["m1"] = struct{}{}
		s.Set("copy_set_src", sv, store.SetOptions{})
		_ = runHandler(t, router, "COPY", [][]byte{
			[]byte("copy_set_src"),
			[]byte("copy_set_dst"),
		}, s)
	})

	t.Run("COPY zset", func(t *testing.T) {
		ssv := &store.SortedSetValue{Members: map[string]float64{}}
		ssv.Add("m1", 1.0)
		s.Set("copy_zset_src", ssv, store.SetOptions{})
		_ = runHandler(t, router, "COPY", [][]byte{
			[]byte("copy_zset_src"),
			[]byte("copy_zset_dst"),
		}, s)
	})
}
