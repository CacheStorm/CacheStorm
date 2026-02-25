package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestLowCoverageBatch13_CachewarmCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterCacheWarmingCommands(router)

	t.Run("KEYOBJECT with string", func(t *testing.T) {
		s.Set("kostr", &store.StringValue{Data: []byte("value")}, store.SetOptions{})
		_ = runHandler(t, router, "KEYOBJECT", [][]byte{[]byte("kostr")}, s)
	})

	t.Run("KEYOBJECT with hash", func(t *testing.T) {
		hv := &store.HashValue{Fields: map[string][]byte{}}
		hv.Fields["field1"] = []byte("value1")
		s.Set("kohash", hv, store.SetOptions{})
		_ = runHandler(t, router, "KEYOBJECT", [][]byte{[]byte("kohash")}, s)
	})

	t.Run("KEYOBJECT with list", func(t *testing.T) {
		lv := &store.ListValue{Elements: [][]byte{[]byte("a"), []byte("b")}}
		s.Set("kolist", lv, store.SetOptions{})
		_ = runHandler(t, router, "KEYOBJECT", [][]byte{[]byte("kolist")}, s)
	})

	t.Run("KEYOBJECT with set", func(t *testing.T) {
		sv := &store.SetValue{Members: map[string]struct{}{}}
		sv.Members["member1"] = struct{}{}
		s.Set("koset", sv, store.SetOptions{})
		_ = runHandler(t, router, "KEYOBJECT", [][]byte{[]byte("koset")}, s)
	})

	t.Run("KEYOBJECT with sorted set", func(t *testing.T) {
		ssv := &store.SortedSetValue{Members: map[string]float64{}}
		ssv.Add("member1", 1.0)
		s.Set("kozset", ssv, store.SetOptions{})
		_ = runHandler(t, router, "KEYOBJECT", [][]byte{[]byte("kozset")}, s)
	})

	t.Run("KEYOBJECT non-existent", func(t *testing.T) {
		_ = runHandler(t, router, "KEYOBJECT", [][]byte{[]byte("nonexistent")}, s)
	})
}

func TestLowCoverageBatch13_FunctionCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterFunctionCommands(router)

	t.Run("FUNCTION.LOAD complex", func(t *testing.T) {
		_ = runHandler(t, router, "FUNCTION.LOAD", [][]byte{
			[]byte("complex_func"),
			[]byte("Lua"),
			[]byte("local x = 1 + 2; return x"),
		}, s)
	})

	t.Run("FCALL with multiple args", func(t *testing.T) {
		_ = runHandler(t, router, "FUNCTION.LOAD", [][]byte{
			[]byte("multi_arg_func"),
			[]byte("Lua"),
			[]byte("return table.concat(ARGV, ',')"),
		}, s)
		_ = runHandler(t, router, "FCALL", [][]byte{
			[]byte("multi_arg_func"),
			[]byte("0"),
			[]byte("a"),
			[]byte("b"),
			[]byte("c"),
		}, s)
	})

	t.Run("FCALL with keys and args", func(t *testing.T) {
		s.Set("fcallkey", &store.StringValue{Data: []byte("value")}, store.SetOptions{})
		_ = runHandler(t, router, "FUNCTION.LOAD", [][]byte{
			[]byte("key_func"),
			[]byte("Lua"),
			[]byte("return redis.call('GET', KEYS[1]) .. ARGV[1]"),
		}, s)
		_ = runHandler(t, router, "FCALL", [][]byte{
			[]byte("key_func"),
			[]byte("1"),
			[]byte("fcallkey"),
			[]byte("_suffix"),
		}, s)
	})
}

func TestLowCoverageBatch13_IntegrationCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterIntegrationCommands(router)

	t.Run("ARRAY.MERGE with concat", func(t *testing.T) {
		_ = runHandler(t, router, "ARRAY.CREATE", [][]byte{[]byte("arr1")}, s)
		_ = runHandler(t, router, "ARRAY.CREATE", [][]byte{[]byte("arr2")}, s)
		_ = runHandler(t, router, "ARRAY.PUSH", [][]byte{[]byte("arr1"), []byte("a")}, s)
		_ = runHandler(t, router, "ARRAY.PUSH", [][]byte{[]byte("arr2"), []byte("b")}, s)
		_ = runHandler(t, router, "ARRAY.MERGE", [][]byte{[]byte("arr1"), []byte("arr2"), []byte("concat")}, s)
	})

	t.Run("OBJECT.MERGE with deep", func(t *testing.T) {
		_ = runHandler(t, router, "OBJECT.CREATE", [][]byte{[]byte("obj1")}, s)
		_ = runHandler(t, router, "OBJECT.CREATE", [][]byte{[]byte("obj2")}, s)
		_ = runHandler(t, router, "OBJECT.SET", [][]byte{[]byte("obj1"), []byte("a"), []byte("1")}, s)
		_ = runHandler(t, router, "OBJECT.SET", [][]byte{[]byte("obj2"), []byte("b"), []byte("2")}, s)
		_ = runHandler(t, router, "OBJECT.MERGE", [][]byte{[]byte("obj1"), []byte("obj2"), []byte("deep")}, s)
	})
}

func TestLowCoverageBatch13_MonitoringCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMonitoringCommands(router)

	t.Run("METRICS with format", func(t *testing.T) {
		_ = runHandler(t, router, "METRICS", [][]byte{[]byte("json")}, s)
	})

	t.Run("METRICS all", func(t *testing.T) {
		_ = runHandler(t, router, "METRICS", [][]byte{[]byte("all")}, s)
	})
}

func TestLowCoverageBatch13_MoreCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	t.Run("QUOTA.XCREATE with strict", func(t *testing.T) {
		_ = runHandler(t, router, "QUOTA.XCREATE", [][]byte{
			[]byte("strict_quota"),
			[]byte("1000"),
			[]byte("3600"),
			[]byte("strict"),
		}, s)
	})

	t.Run("LEASE.CREATE with options", func(t *testing.T) {
		_ = runHandler(t, router, "LEASE.CREATE", [][]byte{
			[]byte("lease1"),
			[]byte("3600"),
			[]byte("renewable"),
		}, s)
	})

	t.Run("SKETCH.CREATE with error rate", func(t *testing.T) {
		_ = runHandler(t, router, "SKETCH.CREATE", [][]byte{
			[]byte("sketch1"),
			[]byte("0.001"),
		}, s)
	})

	t.Run("SKETCH.UPDATE multiple", func(t *testing.T) {
		_ = runHandler(t, router, "SKETCH.CREATE", [][]byte{[]byte("sketch2")}, s)
		_ = runHandler(t, router, "SKETCH.UPDATE", [][]byte{[]byte("sketch2"), []byte("item1")}, s)
		_ = runHandler(t, router, "SKETCH.UPDATE", [][]byte{[]byte("sketch2"), []byte("item2")}, s)
		_ = runHandler(t, router, "SKETCH.UPDATE", [][]byte{[]byte("sketch2"), []byte("item3")}, s)
	})
}

func TestLowCoverageBatch13_MVCCCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMVCCCommands(router)

	t.Run("SPATIAL.WITHIN with points", func(t *testing.T) {
		_ = runHandler(t, router, "SPATIAL.CREATE", [][]byte{[]byte("spatial1")}, s)
		_ = runHandler(t, router, "SPATIAL.ADD", [][]byte{
			[]byte("spatial1"),
			[]byte("point1"),
			[]byte("0"),
			[]byte("0"),
		}, s)
		_ = runHandler(t, router, "SPATIAL.ADD", [][]byte{
			[]byte("spatial1"),
			[]byte("point2"),
			[]byte("10"),
			[]byte("10"),
		}, s)
		_ = runHandler(t, router, "SPATIAL.WITHIN", [][]byte{
			[]byte("spatial1"),
			[]byte("0"),
			[]byte("0"),
			[]byte("100"),
		}, s)
	})

	t.Run("ROLLUP.GET with data", func(t *testing.T) {
		_ = runHandler(t, router, "ROLLUP.ADD", [][]byte{
			[]byte("rollup1"),
			[]byte("100"),
			[]byte("hourly"),
		}, s)
		_ = runHandler(t, router, "ROLLUP.GET", [][]byte{[]byte("rollup1")}, s)
	})
}

func TestLowCoverageBatch13_ResilienceCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	t.Run("MEMORYX.FREE", func(t *testing.T) {
		_ = runHandler(t, router, "MEMORYX.FREE", [][]byte{[]byte("force")}, s)
	})
}

func TestLowCoverageBatch13_SchedulerCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterSchedulerCommands(router)

	t.Run("JOB.CREATE with options", func(t *testing.T) {
		_ = runHandler(t, router, "JOB.CREATE", [][]byte{
			[]byte("job1"),
			[]byte("*/5 * * * *"),
			[]byte("echo hello"),
			[]byte("retries"),
			[]byte("3"),
		}, s)
	})

	t.Run("CIRCUIT.CREATE with options", func(t *testing.T) {
		_ = runHandler(t, router, "CIRCUIT.CREATE", [][]byte{
			[]byte("circuit1"),
			[]byte("5"),
			[]byte("30"),
			[]byte("halfopen"),
			[]byte("3"),
		}, s)
	})

	t.Run("SESSION.REFRESH with data", func(t *testing.T) {
		_ = runHandler(t, router, "SESSION.CREATE", [][]byte{[]byte("session1"), []byte("3600")}, s)
		_ = runHandler(t, router, "SESSION.REFRESH", [][]byte{[]byte("session1"), []byte("7200")}, s)
	})
}

func TestLowCoverageBatch13_ServerCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterServerCommands(router)

	t.Run("DUMP with all types", func(t *testing.T) {
		s.Set("dumpstr", &store.StringValue{Data: []byte("value")}, store.SetOptions{})
		_ = runHandler(t, router, "DUMP", [][]byte{[]byte("dumpstr")}, s)

		hv := &store.HashValue{Fields: map[string][]byte{}}
		hv.Fields["f1"] = []byte("v1")
		s.Set("dumphash", hv, store.SetOptions{})
		_ = runHandler(t, router, "DUMP", [][]byte{[]byte("dumphash")}, s)

		lv := &store.ListValue{Elements: [][]byte{[]byte("a"), []byte("b")}}
		s.Set("dumplist", lv, store.SetOptions{})
		_ = runHandler(t, router, "DUMP", [][]byte{[]byte("dumplist")}, s)

		sv := &store.SetValue{Members: map[string]struct{}{}}
		sv.Members["m1"] = struct{}{}
		s.Set("dumpset", sv, store.SetOptions{})
		_ = runHandler(t, router, "DUMP", [][]byte{[]byte("dumpset")}, s)

		ssv := &store.SortedSetValue{Members: map[string]float64{}}
		ssv.Add("m1", 1.0)
		s.Set("dumpzset", ssv, store.SetOptions{})
		_ = runHandler(t, router, "DUMP", [][]byte{[]byte("dumpzset")}, s)
	})

	t.Run("RESTORE with all types", func(t *testing.T) {
		_ = runHandler(t, router, "RESTORE", [][]byte{
			[]byte("restorestr"),
			[]byte("0"),
			[]byte("CACHSTORM001" + string([]byte{0}) + "0:restored"),
		}, s)

		_ = runHandler(t, router, "RESTORE", [][]byte{
			[]byte("restorehash"),
			[]byte("0"),
			[]byte("CACHSTORM001" + string([]byte{1}) + "0:f1=v1&f2=v2&"),
		}, s)

		_ = runHandler(t, router, "RESTORE", [][]byte{
			[]byte("restorelist"),
			[]byte("0"),
			[]byte("CACHSTORM001" + string([]byte{2}) + "0:a,b,c,"),
		}, s)

		_ = runHandler(t, router, "RESTORE", [][]byte{
			[]byte("restoreset"),
			[]byte("0"),
			[]byte("CACHSTORM001" + string([]byte{3}) + "0:m1,m2,m3,"),
		}, s)
	})

	t.Run("COPY with all options", func(t *testing.T) {
		s.Set("copysrc", &store.StringValue{Data: []byte("value")}, store.SetOptions{})
		_ = runHandler(t, router, "COPY", [][]byte{[]byte("copysrc"), []byte("copydst")}, s)
		_ = runHandler(t, router, "COPY", [][]byte{[]byte("copysrc"), []byte("copydst2"), []byte("REPLACE")}, s)
		_ = runHandler(t, router, "COPY", [][]byte{[]byte("copysrc"), []byte("copydst3"), []byte("DB"), []byte("1")}, s)
	})
}

func TestLowCoverageBatch13_StatsCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStatsCommands(router)

	t.Run("HISTOGRAM.CREATE with buckets", func(t *testing.T) {
		_ = runHandler(t, router, "HISTOGRAM.CREATE", [][]byte{
			[]byte("hist1"),
			[]byte("exponential"),
			[]byte("10"),
		}, s)
	})

	t.Run("HISTOGRAM.OBSERVE multiple", func(t *testing.T) {
		_ = runHandler(t, router, "HISTOGRAM.CREATE", [][]byte{[]byte("hist2")}, s)
		_ = runHandler(t, router, "HISTOGRAM.OBSERVE", [][]byte{[]byte("hist2"), []byte("10")}, s)
		_ = runHandler(t, router, "HISTOGRAM.OBSERVE", [][]byte{[]byte("hist2"), []byte("20")}, s)
		_ = runHandler(t, router, "HISTOGRAM.OBSERVE", [][]byte{[]byte("hist2"), []byte("30")}, s)
	})
}

func TestLowCoverageBatch13_ExtraCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtraCommands(router)

	t.Run("SHARD.CREATE", func(t *testing.T) {
		_ = runHandler(t, router, "SHARD.CREATE", [][]byte{[]byte("shard1"), []byte("3")}, s)
	})

	t.Run("SHARD.MAP with shard", func(t *testing.T) {
		_ = runHandler(t, router, "SHARD.CREATE", [][]byte{[]byte("shard2"), []byte("3")}, s)
		_ = runHandler(t, router, "SHARD.MAP", [][]byte{[]byte("shard2"), []byte("key1")}, s)
	})

	t.Run("SHARD.STATUS with shard", func(t *testing.T) {
		_ = runHandler(t, router, "SHARD.CREATE", [][]byte{[]byte("shard3"), []byte("3")}, s)
		_ = runHandler(t, router, "SHARD.STATUS", [][]byte{[]byte("shard3")}, s)
	})

	t.Run("SHARD.REBALANCE with shard", func(t *testing.T) {
		_ = runHandler(t, router, "SHARD.CREATE", [][]byte{[]byte("shard4"), []byte("3")}, s)
		_ = runHandler(t, router, "SHARD.REBALANCE", [][]byte{[]byte("shard4")}, s)
	})

	t.Run("ROUTE.ADD and ROUTE.LIST", func(t *testing.T) {
		_ = runHandler(t, router, "ROUTE.ADD", [][]byte{[]byte("users:*"), []byte("shard1")}, s)
		_ = runHandler(t, router, "ROUTE.LIST", [][]byte{}, s)
	})

	t.Run("ROUTE.MATCH", func(t *testing.T) {
		_ = runHandler(t, router, "ROUTE.ADD", [][]byte{[]byte("users:*"), []byte("shard1")}, s)
		_ = runHandler(t, router, "ROUTE.MATCH", [][]byte{[]byte("users:123")}, s)
	})

	t.Run("ROUTE.REMOVE", func(t *testing.T) {
		_ = runHandler(t, router, "ROUTE.ADD", [][]byte{[]byte("test:*"), []byte("shard1")}, s)
		_ = runHandler(t, router, "ROUTE.REMOVE", [][]byte{[]byte("test:*")}, s)
	})

	t.Run("VECTOR_CLOCK operations", func(t *testing.T) {
		_ = runHandler(t, router, "VECTOR_CLOCK.CREATE", [][]byte{[]byte("vc1")}, s)
		_ = runHandler(t, router, "VECTOR_CLOCK.INCREMENT", [][]byte{[]byte("vc1"), []byte("node1")}, s)
		_ = runHandler(t, router, "VECTOR_CLOCK.GET", [][]byte{[]byte("vc1")}, s)
	})
}

func TestLowCoverageBatch13_WorkflowCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterWorkflowCommands(router)

	t.Run("TEMPLATE.CREATE with complex template", func(t *testing.T) {
		_ = runHandler(t, router, "TEMPLATE.CREATE", [][]byte{
			[]byte("tpl1"),
			[]byte("Hello {{name}}, you have {{count}} messages"),
		}, s)
	})

	t.Run("TEMPLATE.GET with data", func(t *testing.T) {
		_ = runHandler(t, router, "TEMPLATE.CREATE", [][]byte{
			[]byte("tpl2"),
			[]byte("Hello {{name}}"),
		}, s)
		_ = runHandler(t, router, "TEMPLATE.GET", [][]byte{
			[]byte("tpl2"),
			[]byte(`{"name":"World"}`),
		}, s)
	})

	t.Run("CHAINED operations", func(t *testing.T) {
		_ = runHandler(t, router, "CHAINED.SET", [][]byte{
			[]byte("root1"),
			[]byte("path1"),
			[]byte("value1"),
		}, s)
		_ = runHandler(t, router, "CHAINED.GET", [][]byte{
			[]byte("root1"),
			[]byte("path1"),
		}, s)
		_ = runHandler(t, router, "CHAINED.DEL", [][]byte{
			[]byte("root1"),
			[]byte("path1"),
		}, s)
	})

	t.Run("REACTIVE operations", func(t *testing.T) {
		_ = runHandler(t, router, "REACTIVE.WATCH", [][]byte{
			[]byte("key1"),
			[]byte("callback1"),
		}, s)
		_ = runHandler(t, router, "REACTIVE.UNWATCH", [][]byte{
			[]byte("key1"),
			[]byte("callback1"),
		}, s)
	})
}

func TestLowCoverageBatch13_UtilityExtCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterUtilityExtCommands(router)

	t.Run("FLAG.ADDRULE with multiple options", func(t *testing.T) {
		_ = runHandler(t, router, "FLAG.ADDRULE", [][]byte{
			[]byte("flag1"),
			[]byte("percentage"),
			[]byte("50"),
			[]byte("true"),
		}, s)
	})

	t.Run("FLAG.CHECK", func(t *testing.T) {
		_ = runHandler(t, router, "FLAG.ADDRULE", [][]byte{
			[]byte("flag2"),
			[]byte("percentage"),
			[]byte("50"),
		}, s)
		_ = runHandler(t, router, "FLAG.CHECK", [][]byte{
			[]byte("flag2"),
			[]byte("user1"),
		}, s)
	})

	t.Run("BACKUP.RESTORE", func(t *testing.T) {
		_ = runHandler(t, router, "BACKUP.RESTORE", [][]byte{
			[]byte("backup1"),
			[]byte("force"),
		}, s)
	})
}

func TestLowCoverageBatch13_MLCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMLCommands(router)

	t.Run("TENSOR.CREATE with dimensions", func(t *testing.T) {
		_ = runHandler(t, router, "TENSOR.CREATE", [][]byte{
			[]byte("tensor1"),
			[]byte("3"),
			[]byte("3"),
			[]byte("2"),
		}, s)
	})

	t.Run("TENSOR.GET", func(t *testing.T) {
		_ = runHandler(t, router, "TENSOR.CREATE", [][]byte{[]byte("tensor2"), []byte("2"), []byte("2")}, s)
		_ = runHandler(t, router, "TENSOR.GET", [][]byte{[]byte("tensor2")}, s)
	})

	t.Run("MODEL.CREATE", func(t *testing.T) {
		_ = runHandler(t, router, "MODEL.CREATE", [][]byte{
			[]byte("model1"),
			[]byte("linear"),
		}, s)
	})

	t.Run("MODEL.TRAIN", func(t *testing.T) {
		_ = runHandler(t, router, "MODEL.CREATE", [][]byte{[]byte("model2"), []byte("linear")}, s)
		_ = runHandler(t, router, "MODEL.TRAIN", [][]byte{
			[]byte("model2"),
			[]byte("x"),
			[]byte("y"),
		}, s)
	})

	t.Run("MODEL.PREDICT", func(t *testing.T) {
		_ = runHandler(t, router, "MODEL.CREATE", [][]byte{[]byte("model3"), []byte("linear")}, s)
		_ = runHandler(t, router, "MODEL.PREDICT", [][]byte{[]byte("model3"), []byte("10")}, s)
	})
}

func TestLowCoverageBatch13_ActorCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterActorCommands(router)

	t.Run("ACTOR.CREATE", func(t *testing.T) {
		_ = runHandler(t, router, "ACTOR.CREATE", [][]byte{[]byte("actor1")}, s)
	})

	t.Run("ACTOR.SEND", func(t *testing.T) {
		_ = runHandler(t, router, "ACTOR.CREATE", [][]byte{[]byte("actor2")}, s)
		_ = runHandler(t, router, "ACTOR.SEND", [][]byte{
			[]byte("actor2"),
			[]byte("message"),
			[]byte("hello"),
		}, s)
	})

	t.Run("ACTOR.RECEIVE", func(t *testing.T) {
		_ = runHandler(t, router, "ACTOR.CREATE", [][]byte{[]byte("actor3")}, s)
		_ = runHandler(t, router, "ACTOR.RECEIVE", [][]byte{[]byte("actor3")}, s)
	})
}

func TestLowCoverageBatch13_EventCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterEventCommands(router)

	t.Run("EVENT.PUBLISH", func(t *testing.T) {
		_ = runHandler(t, router, "EVENT.PUBLISH", [][]byte{
			[]byte("channel1"),
			[]byte("message1"),
		}, s)
	})

	t.Run("EVENT.SUBSCRIBE", func(t *testing.T) {
		_ = runHandler(t, router, "EVENT.SUBSCRIBE", [][]byte{[]byte("channel2")}, s)
	})

	t.Run("EVENT.UNSUBSCRIBE", func(t *testing.T) {
		_ = runHandler(t, router, "EVENT.SUBSCRIBE", [][]byte{[]byte("channel3")}, s)
		_ = runHandler(t, router, "EVENT.UNSUBSCRIBE", [][]byte{[]byte("channel3")}, s)
	})
}

func TestLowCoverageBatch13_AdvancedCommands2(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterAdvancedCommands2(router)

	t.Run("GHOST.CREATE", func(t *testing.T) {
		_ = runHandler(t, router, "GHOST.CREATE", [][]byte{[]byte("ghost1")}, s)
	})

	t.Run("DEDUP.ADD", func(t *testing.T) {
		_ = runHandler(t, router, "DEDUP.ADD", [][]byte{[]byte("dedup1"), []byte("id1")}, s)
	})

	t.Run("DEDUP.CHECK", func(t *testing.T) {
		_ = runHandler(t, router, "DEDUP.ADD", [][]byte{[]byte("dedup2"), []byte("id1")}, s)
		_ = runHandler(t, router, "DEDUP.CHECK", [][]byte{[]byte("dedup2"), []byte("id1")}, s)
	})
}

func TestLowCoverageBatch13_SortedSetCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterSortedSetCommands(router)

	t.Run("ZRANDMEMBER with count", func(t *testing.T) {
		ssv := &store.SortedSetValue{Members: map[string]float64{}}
		ssv.Add("a", 1.0)
		ssv.Add("b", 2.0)
		ssv.Add("c", 3.0)
		s.Set("zrand1", ssv, store.SetOptions{})
		_ = runHandler(t, router, "ZRANDMEMBER", [][]byte{[]byte("zrand1"), []byte("2")}, s)
	})

	t.Run("ZUNIONSTORE", func(t *testing.T) {
		ssv1 := &store.SortedSetValue{Members: map[string]float64{}}
		ssv1.Add("a", 1.0)
		s.Set("zunion1", ssv1, store.SetOptions{})
		ssv2 := &store.SortedSetValue{Members: map[string]float64{}}
		ssv2.Add("b", 2.0)
		s.Set("zunion2", ssv2, store.SetOptions{})
		_ = runHandler(t, router, "ZUNIONSTORE", [][]byte{
			[]byte("zuniondest"),
			[]byte("2"),
			[]byte("zunion1"),
			[]byte("zunion2"),
		}, s)
	})

	t.Run("ZINTERSTORE", func(t *testing.T) {
		ssv1 := &store.SortedSetValue{Members: map[string]float64{}}
		ssv1.Add("a", 1.0)
		ssv1.Add("b", 2.0)
		s.Set("zinter1", ssv1, store.SetOptions{})
		ssv2 := &store.SortedSetValue{Members: map[string]float64{}}
		ssv2.Add("a", 3.0)
		ssv2.Add("c", 4.0)
		s.Set("zinter2", ssv2, store.SetOptions{})
		_ = runHandler(t, router, "ZINTERSTORE", [][]byte{
			[]byte("zinterdest"),
			[]byte("2"),
			[]byte("zinter1"),
			[]byte("zinter2"),
		}, s)
	})

	t.Run("ZDIFFSTORE", func(t *testing.T) {
		ssv1 := &store.SortedSetValue{Members: map[string]float64{}}
		ssv1.Add("a", 1.0)
		ssv1.Add("b", 2.0)
		s.Set("zdiff1", ssv1, store.SetOptions{})
		ssv2 := &store.SortedSetValue{Members: map[string]float64{}}
		ssv2.Add("a", 3.0)
		s.Set("zdiff2", ssv2, store.SetOptions{})
		_ = runHandler(t, router, "ZDIFFSTORE", [][]byte{
			[]byte("zdiffdest"),
			[]byte("2"),
			[]byte("zdiff1"),
			[]byte("zdiff2"),
		}, s)
	})
}
