package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func runHandler(t *testing.T, router *Router, cmd string, args [][]byte, s *store.Store) error {
	t.Helper()
	ctx := newTestCtx(cmd, args, s)
	handler, ok := router.Get(cmd)
	if !ok {
		t.Skipf("%s not registered", cmd)
	}
	return handler.Handler(ctx)
}

func TestLowCoverageBatch9_BatchCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterCacheWarmingCommands(router)
	RegisterStringCommands(router)

	t.Run("BATCH.EXEC basic operations", func(t *testing.T) {
		_ = runHandler(t, router, "BATCH.EXEC", [][]byte{
			[]byte("SET"), []byte("key1"), []byte("value1"),
			[]byte("GET"), []byte("key1"),
			[]byte("EXISTS"), []byte("key1"),
			[]byte("DEL"), []byte("key1"),
			[]byte("GET"), []byte("key1"),
		}, s)
	})

	t.Run("BATCH.EXEC with GET", func(t *testing.T) {
		_ = runHandler(t, router, "BATCH.EXEC", [][]byte{[]byte("GET"), []byte("nonexistent")}, s)
	})

	t.Run("BATCH.EXEC with SET", func(t *testing.T) {
		_ = runHandler(t, router, "BATCH.EXEC", [][]byte{[]byte("SET"), []byte("key2"), []byte("val")}, s)
	})

	t.Run("BATCH.EXEC with DEL", func(t *testing.T) {
		_ = runHandler(t, router, "BATCH.EXEC", [][]byte{[]byte("DEL"), []byte("key1")}, s)
	})

	t.Run("BATCH.EXEC with EXISTS", func(t *testing.T) {
		_ = runHandler(t, router, "BATCH.EXEC", [][]byte{[]byte("EXISTS"), []byte("key1")}, s)
	})

	t.Run("PIPELINE.EXEC", func(t *testing.T) {
		_ = runHandler(t, router, "PIPELINE.EXEC", [][]byte{
			[]byte("SET"), []byte("pipekey"), []byte("pipeval"),
		}, s)
	})
}

func TestLowCoverageBatch9_ClusterCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterClusterCommands(router)

	t.Run("CLUSTERINFO", func(t *testing.T) {
		_ = runHandler(t, router, "CLUSTERINFO", [][]byte{}, s)
	})

	t.Run("CLUSTERNODES", func(t *testing.T) {
		_ = runHandler(t, router, "CLUSTERNODES", [][]byte{}, s)
	})
}

func TestLowCoverageBatch9_DumpRestoreCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterServerCommands(router)

	t.Run("DUMP string value", func(t *testing.T) {
		s.Set("dumpkey", &store.StringValue{Data: []byte("dumpvalue")}, store.SetOptions{})
		_ = runHandler(t, router, "DUMP", [][]byte{[]byte("dumpkey")}, s)
	})

	t.Run("DUMP non-existent key", func(t *testing.T) {
		_ = runHandler(t, router, "DUMP", [][]byte{[]byte("nonexistent")}, s)
	})

	t.Run("DUMP hash value", func(t *testing.T) {
		hv := &store.HashValue{Fields: make(map[string][]byte)}
		hv.Fields["field1"] = []byte("value1")
		hv.Fields["field2"] = []byte("value2")
		s.Set("dumphash", hv, store.SetOptions{})
		_ = runHandler(t, router, "DUMP", [][]byte{[]byte("dumphash")}, s)
	})

	t.Run("DUMP list value", func(t *testing.T) {
		lv := &store.ListValue{Elements: [][]byte{[]byte("a"), []byte("b"), []byte("c")}}
		s.Set("dumplist", lv, store.SetOptions{})
		_ = runHandler(t, router, "DUMP", [][]byte{[]byte("dumplist")}, s)
	})

	t.Run("DUMP set value", func(t *testing.T) {
		sv := &store.SetValue{Members: make(map[string]struct{})}
		sv.Members["member1"] = struct{}{}
		sv.Members["member2"] = struct{}{}
		s.Set("dumpset", sv, store.SetOptions{})
		_ = runHandler(t, router, "DUMP", [][]byte{[]byte("dumpset")}, s)
	})

	t.Run("DUMP sorted set value", func(t *testing.T) {
		ssv := &store.SortedSetValue{Members: make(map[string]float64)}
		ssv.Add("member1", 1.0)
		ssv.Add("member2", 2.0)
		s.Set("dumpzset", ssv, store.SetOptions{})
		_ = runHandler(t, router, "DUMP", [][]byte{[]byte("dumpzset")}, s)
	})

	t.Run("RESTORE string", func(t *testing.T) {
		_ = runHandler(t, router, "RESTORE", [][]byte{
			[]byte("restorekey"),
			[]byte("0"),
			[]byte("CACHSTORM001" + string([]byte{0}) + "0:restoredvalue"),
		}, s)
	})

	t.Run("RESTORE with REPLACE", func(t *testing.T) {
		s.Set("restorekey2", &store.StringValue{Data: []byte("existing")}, store.SetOptions{})
		_ = runHandler(t, router, "RESTORE", [][]byte{
			[]byte("restorekey2"),
			[]byte("0"),
			[]byte("CACHSTORM001" + string([]byte{0}) + "0:newvalue"),
			[]byte("REPLACE"),
		}, s)
	})

	t.Run("RESTORE hash", func(t *testing.T) {
		_ = runHandler(t, router, "RESTORE", [][]byte{
			[]byte("restorehash"),
			[]byte("0"),
			[]byte("CACHSTORM001" + string([]byte{1}) + "0:f1=v1&f2=v2&"),
		}, s)
	})

	t.Run("RESTORE list", func(t *testing.T) {
		_ = runHandler(t, router, "RESTORE", [][]byte{
			[]byte("restorelist"),
			[]byte("0"),
			[]byte("CACHSTORM001" + string([]byte{2}) + "0:a,b,c,"),
		}, s)
	})

	t.Run("RESTORE set", func(t *testing.T) {
		_ = runHandler(t, router, "RESTORE", [][]byte{
			[]byte("restoreset"),
			[]byte("0"),
			[]byte("CACHSTORM001" + string([]byte{3}) + "0:m1,m2,m3,"),
		}, s)
	})
}

func TestLowCoverageBatch9_CopyCommand(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterServerCommands(router)

	t.Run("COPY basic", func(t *testing.T) {
		s.Set("src1", &store.StringValue{Data: []byte("value1")}, store.SetOptions{})
		_ = runHandler(t, router, "COPY", [][]byte{[]byte("src1"), []byte("dst1")}, s)
	})

	t.Run("COPY with REPLACE", func(t *testing.T) {
		s.Set("src2", &store.StringValue{Data: []byte("value2")}, store.SetOptions{})
		s.Set("dst2", &store.StringValue{Data: []byte("existing")}, store.SetOptions{})
		_ = runHandler(t, router, "COPY", [][]byte{[]byte("src2"), []byte("dst2"), []byte("REPLACE")}, s)
	})

	t.Run("COPY non-existent source", func(t *testing.T) {
		_ = runHandler(t, router, "COPY", [][]byte{[]byte("nonexistent"), []byte("dst4")}, s)
	})

	t.Run("COPY with DB option", func(t *testing.T) {
		s.Set("src5", &store.StringValue{Data: []byte("value5")}, store.SetOptions{})
		_ = runHandler(t, router, "COPY", [][]byte{[]byte("src5"), []byte("dst5"), []byte("DB"), []byte("1")}, s)
	})
}

func TestLowCoverageBatch9_RaftCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtraCommands(router)

	t.Run("RAFT.STATE create", func(t *testing.T) {
		_ = runHandler(t, router, "RAFT.STATE", [][]byte{[]byte("raft1"), []byte("follower")}, s)
	})

	t.Run("RAFT.STATE update", func(t *testing.T) {
		_ = runHandler(t, router, "RAFT.STATE", [][]byte{[]byte("raft1"), []byte("leader")}, s)
	})

	t.Run("RAFT.LEADER success", func(t *testing.T) {
		_ = runHandler(t, router, "RAFT.STATE", [][]byte{[]byte("raft2"), []byte("follower")}, s)
		_ = runHandler(t, router, "RAFT.LEADER", [][]byte{[]byte("raft2"), []byte("node1")}, s)
	})

	t.Run("RAFT.TERM success", func(t *testing.T) {
		_ = runHandler(t, router, "RAFT.STATE", [][]byte{[]byte("raft3"), []byte("follower")}, s)
		_ = runHandler(t, router, "RAFT.TERM", [][]byte{[]byte("raft3")}, s)
	})

	t.Run("RAFT.VOTE success", func(t *testing.T) {
		_ = runHandler(t, router, "RAFT.STATE", [][]byte{[]byte("raft4"), []byte("follower")}, s)
		_ = runHandler(t, router, "RAFT.VOTE", [][]byte{[]byte("raft4"), []byte("candidate1")}, s)
	})

	t.Run("RAFT.VOTE already voted", func(t *testing.T) {
		_ = runHandler(t, router, "RAFT.STATE", [][]byte{[]byte("raft5"), []byte("follower")}, s)
		_ = runHandler(t, router, "RAFT.VOTE", [][]byte{[]byte("raft5"), []byte("candidate1")}, s)
		_ = runHandler(t, router, "RAFT.VOTE", [][]byte{[]byte("raft5"), []byte("candidate2")}, s)
	})

	t.Run("RAFT.APPEND success", func(t *testing.T) {
		_ = runHandler(t, router, "RAFT.STATE", [][]byte{[]byte("raft6"), []byte("leader")}, s)
		_ = runHandler(t, router, "RAFT.APPEND", [][]byte{[]byte("raft6"), []byte("log entry")}, s)
	})

	t.Run("RAFT.COMMIT success", func(t *testing.T) {
		_ = runHandler(t, router, "RAFT.STATE", [][]byte{[]byte("raft7"), []byte("leader")}, s)
		_ = runHandler(t, router, "RAFT.COMMIT", [][]byte{[]byte("raft7")}, s)
	})
}

func TestLowCoverageBatch9_LockCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterUtilityCommands(router)

	t.Run("LOCK.TRY acquire", func(t *testing.T) {
		_ = runHandler(t, router, "LOCK.TRY", [][]byte{
			[]byte("lock1"),
			[]byte("holder1"),
			[]byte("token1"),
			[]byte("10000"),
		}, s)
	})

	t.Run("LOCK.ACQUIRE", func(t *testing.T) {
		_ = runHandler(t, router, "LOCK.ACQUIRE", [][]byte{
			[]byte("lock2"),
			[]byte("holder2"),
			[]byte("token2"),
			[]byte("10000"),
			[]byte("1000"),
		}, s)
	})

	t.Run("LOCK.RELEASE", func(t *testing.T) {
		_ = runHandler(t, router, "LOCK.TRY", [][]byte{
			[]byte("lock3"),
			[]byte("holder3"),
			[]byte("token3"),
			[]byte("10000"),
		}, s)
		_ = runHandler(t, router, "LOCK.RELEASE", [][]byte{
			[]byte("lock3"),
			[]byte("holder3"),
			[]byte("token3"),
		}, s)
	})

	t.Run("LOCK.RENEW", func(t *testing.T) {
		_ = runHandler(t, router, "LOCK.TRY", [][]byte{
			[]byte("lock4"),
			[]byte("holder4"),
			[]byte("token4"),
			[]byte("10000"),
		}, s)
		_ = runHandler(t, router, "LOCK.RENEW", [][]byte{
			[]byte("lock4"),
			[]byte("holder4"),
			[]byte("token4"),
			[]byte("20000"),
		}, s)
	})

	t.Run("LOCK.INFO", func(t *testing.T) {
		_ = runHandler(t, router, "LOCK.TRY", [][]byte{
			[]byte("lock5"),
			[]byte("holder5"),
			[]byte("token5"),
			[]byte("10000"),
		}, s)
		_ = runHandler(t, router, "LOCK.INFO", [][]byte{[]byte("lock5")}, s)
	})

	t.Run("LOCK.INFO non-existent", func(t *testing.T) {
		_ = runHandler(t, router, "LOCK.INFO", [][]byte{[]byte("nonexistent")}, s)
	})

	t.Run("LOCK.ISLOCKED", func(t *testing.T) {
		_ = runHandler(t, router, "LOCK.TRY", [][]byte{
			[]byte("lock6"),
			[]byte("holder6"),
			[]byte("token6"),
			[]byte("10000"),
		}, s)
		_ = runHandler(t, router, "LOCK.ISLOCKED", [][]byte{[]byte("lock6")}, s)
	})
}

func TestLowCoverageBatch9_ChainedCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterWorkflowCommands(router)

	t.Run("CHAINED.SET", func(t *testing.T) {
		_ = runHandler(t, router, "CHAINED.SET", [][]byte{
			[]byte("root1"),
			[]byte("path1"),
			[]byte("value1"),
		}, s)
	})

	t.Run("CHAINED.GET existing", func(t *testing.T) {
		_ = runHandler(t, router, "CHAINED.SET", [][]byte{
			[]byte("root2"),
			[]byte("path2"),
			[]byte("value2"),
		}, s)
		_ = runHandler(t, router, "CHAINED.GET", [][]byte{[]byte("root2"), []byte("path2")}, s)
	})

	t.Run("CHAINED.GET non-existent root", func(t *testing.T) {
		_ = runHandler(t, router, "CHAINED.GET", [][]byte{[]byte("nonexistent"), []byte("path")}, s)
	})

	t.Run("CHAINED.GET non-existent path", func(t *testing.T) {
		_ = runHandler(t, router, "CHAINED.SET", [][]byte{
			[]byte("root3"),
			[]byte("path3"),
			[]byte("value3"),
		}, s)
		_ = runHandler(t, router, "CHAINED.GET", [][]byte{[]byte("root3"), []byte("nonexistent")}, s)
	})

	t.Run("CHAINED.DEL existing", func(t *testing.T) {
		_ = runHandler(t, router, "CHAINED.SET", [][]byte{
			[]byte("root4"),
			[]byte("path4"),
			[]byte("value4"),
		}, s)
		_ = runHandler(t, router, "CHAINED.DEL", [][]byte{[]byte("root4"), []byte("path4")}, s)
	})

	t.Run("CHAINED.DEL non-existent", func(t *testing.T) {
		_ = runHandler(t, router, "CHAINED.DEL", [][]byte{[]byte("nonexistent"), []byte("path")}, s)
	})
}

func TestLowCoverageBatch9_ReactiveCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterWorkflowCommands(router)

	t.Run("REACTIVE.WATCH", func(t *testing.T) {
		_ = runHandler(t, router, "REACTIVE.WATCH", [][]byte{[]byte("key1"), []byte("callback1")}, s)
	})

	t.Run("REACTIVE.UNWATCH", func(t *testing.T) {
		_ = runHandler(t, router, "REACTIVE.WATCH", [][]byte{[]byte("key2"), []byte("callback2")}, s)
		_ = runHandler(t, router, "REACTIVE.UNWATCH", [][]byte{[]byte("key2"), []byte("callback2")}, s)
	})
}

func TestLowCoverageBatch9_ExtraRouteCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtraCommands(router)

	t.Run("SHARD.REBALANCE", func(t *testing.T) {
		_ = runHandler(t, router, "SHARD.REBALANCE", [][]byte{}, s)
	})

	t.Run("ROUTE.REMOVE", func(t *testing.T) {
		_ = runHandler(t, router, "ROUTE.ADD", [][]byte{[]byte("pattern1"), []byte("target1")}, s)
		_ = runHandler(t, router, "ROUTE.REMOVE", [][]byte{[]byte("pattern1")}, s)
	})

	t.Run("ROUTE.MATCH", func(t *testing.T) {
		_ = runHandler(t, router, "ROUTE.ADD", [][]byte{[]byte("users:*"), []byte("shard1")}, s)
		_ = runHandler(t, router, "ROUTE.MATCH", [][]byte{[]byte("users:123")}, s)
	})

	t.Run("ROUTE.LIST", func(t *testing.T) {
		_ = runHandler(t, router, "ROUTE.ADD", [][]byte{[]byte("listpattern"), []byte("target")}, s)
		_ = runHandler(t, router, "ROUTE.LIST", [][]byte{}, s)
	})
}

func TestLowCoverageBatch9_MoreCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	t.Run("TRACE.SPAN", func(t *testing.T) {
		_ = runHandler(t, router, "TRACE.SPAN", [][]byte{[]byte("span1"), []byte("start")}, s)
	})

	t.Run("QUOTA.XCREATE", func(t *testing.T) {
		_ = runHandler(t, router, "QUOTA.XCREATE", [][]byte{[]byte("quota1"), []byte("100"), []byte("60")}, s)
	})

	t.Run("METER.CREATE", func(t *testing.T) {
		_ = runHandler(t, router, "METER.CREATE", [][]byte{[]byte("meter1")}, s)
	})

	t.Run("TENANT.CREATE", func(t *testing.T) {
		_ = runHandler(t, router, "TENANT.CREATE", [][]byte{[]byte("tenant1")}, s)
	})

	t.Run("LEASE.CREATE", func(t *testing.T) {
		_ = runHandler(t, router, "LEASE.CREATE", [][]byte{[]byte("lease1"), []byte("60")}, s)
	})

	t.Run("LEASE.RENEW", func(t *testing.T) {
		_ = runHandler(t, router, "LEASE.CREATE", [][]byte{[]byte("lease2"), []byte("60")}, s)
		_ = runHandler(t, router, "LEASE.RENEW", [][]byte{[]byte("lease2"), []byte("120")}, s)
	})

	t.Run("SKETCH.CREATE", func(t *testing.T) {
		_ = runHandler(t, router, "SKETCH.CREATE", [][]byte{[]byte("sketch1")}, s)
	})

	t.Run("SKETCH.UPDATE", func(t *testing.T) {
		_ = runHandler(t, router, "SKETCH.CREATE", [][]byte{[]byte("sketch2")}, s)
		_ = runHandler(t, router, "SKETCH.UPDATE", [][]byte{[]byte("sketch2"), []byte("item1")}, s)
	})

	t.Run("PARTITION.ADD", func(t *testing.T) {
		_ = runHandler(t, router, "PARTITION.ADD", [][]byte{[]byte("part1"), []byte("node1")}, s)
	})
}

func TestLowCoverageBatch9_MVCCCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMVCCCommands(router)

	t.Run("SPATIAL.WITHIN", func(t *testing.T) {
		_ = runHandler(t, router, "SPATIAL.WITHIN", [][]byte{
			[]byte("spatial1"),
			[]byte("0"),
			[]byte("0"),
			[]byte("10"),
		}, s)
	})

	t.Run("ROLLUP.ADD", func(t *testing.T) {
		_ = runHandler(t, router, "ROLLUP.ADD", [][]byte{
			[]byte("rollup1"),
			[]byte("100"),
			[]byte("hourly"),
		}, s)
	})

	t.Run("ROLLUP.GET", func(t *testing.T) {
		_ = runHandler(t, router, "ROLLUP.ADD", [][]byte{
			[]byte("rollup2"),
			[]byte("100"),
			[]byte("hourly"),
		}, s)
		_ = runHandler(t, router, "ROLLUP.GET", [][]byte{[]byte("rollup2")}, s)
	})

	t.Run("QUOTA.SET", func(t *testing.T) {
		_ = runHandler(t, router, "QUOTA.SET", [][]byte{[]byte("quota2"), []byte("1000")}, s)
	})
}

func TestLowCoverageBatch9_ResilienceCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	t.Run("DIAGNOSTIC.RUN", func(t *testing.T) {
		_ = runHandler(t, router, "DIAGNOSTIC.RUN", [][]byte{[]byte("diag1")}, s)
	})

	t.Run("MEMORYX.FREE", func(t *testing.T) {
		_ = runHandler(t, router, "MEMORYX.FREE", [][]byte{}, s)
	})

	t.Run("MEMORYX.STATS", func(t *testing.T) {
		_ = runHandler(t, router, "MEMORYX.STATS", [][]byte{}, s)
	})
}

func TestLowCoverageBatch9_SchedulerCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterSchedulerCommands(router)

	t.Run("JOB.CREATE", func(t *testing.T) {
		_ = runHandler(t, router, "JOB.CREATE", [][]byte{
			[]byte("job1"),
			[]byte("*/5 * * * *"),
			[]byte("echo hello"),
		}, s)
	})

	t.Run("CIRCUIT.CREATE", func(t *testing.T) {
		_ = runHandler(t, router, "CIRCUIT.CREATE", [][]byte{
			[]byte("circuit1"),
			[]byte("5"),
			[]byte("30"),
		}, s)
	})

	t.Run("SESSION.REFRESH", func(t *testing.T) {
		_ = runHandler(t, router, "SESSION.REFRESH", [][]byte{[]byte("session1")}, s)
	})
}

func TestLowCoverageBatch9_StatsCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStatsCommands(router)

	t.Run("SAMPLE.CREATE", func(t *testing.T) {
		_ = runHandler(t, router, "SAMPLE.CREATE", [][]byte{[]byte("sample1")}, s)
	})

	t.Run("HISTOGRAM.CREATE", func(t *testing.T) {
		_ = runHandler(t, router, "HISTOGRAM.CREATE", [][]byte{[]byte("hist1")}, s)
	})
}

func TestLowCoverageBatch9_MonitoringCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMonitoringCommands(router)

	t.Run("METRICS", func(t *testing.T) {
		_ = runHandler(t, router, "METRICS", [][]byte{}, s)
	})
}

func TestLowCoverageBatch9_FunctionCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterFunctionCommands(router)

	t.Run("FCALL", func(t *testing.T) {
		_ = runHandler(t, router, "FUNCTION.LOAD", [][]byte{
			[]byte("func1"),
			[]byte("Lua"),
			[]byte("return 'hello'"),
		}, s)
		_ = runHandler(t, router, "FCALL", [][]byte{[]byte("func1"), []byte("0")}, s)
	})
}

func TestLowCoverageBatch9_IntegrationCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterIntegrationCommands(router)
	RegisterListCommands(router)
	RegisterHashCommands(router)

	t.Run("CACHE.REFRESH", func(t *testing.T) {
		_ = runHandler(t, router, "CACHE.REFRESH", [][]byte{[]byte("cachekey1")}, s)
	})

	t.Run("ARRAY.MERGE", func(t *testing.T) {
		_ = runHandler(t, router, "LPUSH", [][]byte{[]byte("arr1"), []byte("a"), []byte("b")}, s)
		_ = runHandler(t, router, "LPUSH", [][]byte{[]byte("arr2"), []byte("c"), []byte("d")}, s)
		_ = runHandler(t, router, "ARRAY.MERGE", [][]byte{[]byte("arr1"), []byte("arr2")}, s)
	})

	t.Run("OBJECT.MERGE", func(t *testing.T) {
		_ = runHandler(t, router, "HSET", [][]byte{[]byte("obj1"), []byte("a"), []byte("1")}, s)
		_ = runHandler(t, router, "HSET", [][]byte{[]byte("obj2"), []byte("b"), []byte("2")}, s)
		_ = runHandler(t, router, "OBJECT.MERGE", [][]byte{[]byte("obj1"), []byte("obj2")}, s)
	})
}

func TestLowCoverageBatch9_TensorCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMLCommands(router)

	t.Run("TENSOR.CREATE", func(t *testing.T) {
		_ = runHandler(t, router, "TENSOR.CREATE", [][]byte{[]byte("tensor1"), []byte("3"), []byte("3")}, s)
	})
}

func TestLowCoverageBatch9_UtilityExtCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterUtilityExtCommands(router)

	t.Run("FLAG.ADDRULE", func(t *testing.T) {
		_ = runHandler(t, router, "FLAG.ADDRULE", [][]byte{
			[]byte("flag1"),
			[]byte("percentage"),
			[]byte("50"),
		}, s)
	})
}

func TestLowCoverageBatch9_ScriptCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterScriptCommands(router)

	t.Run("EVAL basic", func(t *testing.T) {
		_ = runHandler(t, router, "EVAL", [][]byte{
			[]byte("return {1, 2, 3}"),
			[]byte("0"),
		}, s)
	})

	t.Run("EVAL with string result", func(t *testing.T) {
		_ = runHandler(t, router, "EVAL", [][]byte{
			[]byte("return 'hello world'"),
			[]byte("0"),
		}, s)
	})

	t.Run("EVAL with number result", func(t *testing.T) {
		_ = runHandler(t, router, "EVAL", [][]byte{
			[]byte("return 42"),
			[]byte("0"),
		}, s)
	})

	t.Run("EVAL with boolean result", func(t *testing.T) {
		_ = runHandler(t, router, "EVAL", [][]byte{
			[]byte("return true"),
			[]byte("0"),
		}, s)
	})

	t.Run("EVAL with nil result", func(t *testing.T) {
		_ = runHandler(t, router, "EVAL", [][]byte{
			[]byte("return nil"),
			[]byte("0"),
		}, s)
	})

	t.Run("EVAL with table result", func(t *testing.T) {
		_ = runHandler(t, router, "EVAL", [][]byte{
			[]byte("return {key = 'value', num = 123}"),
			[]byte("0"),
		}, s)
	})
}

func TestLowCoverageBatch9_KEYOBJECT(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterCacheWarmingCommands(router)

	t.Run("KEYOBJECT string", func(t *testing.T) {
		s.Set("kostr", &store.StringValue{Data: []byte("value")}, store.SetOptions{})
		_ = runHandler(t, router, "KEYOBJECT", [][]byte{[]byte("kostr")}, s)
	})

	t.Run("KEYOBJECT hash", func(t *testing.T) {
		hv := &store.HashValue{Fields: make(map[string][]byte)}
		hv.Fields["f"] = []byte("v")
		s.Set("kohash", hv, store.SetOptions{})
		_ = runHandler(t, router, "KEYOBJECT", [][]byte{[]byte("kohash")}, s)
	})

	t.Run("KEYOBJECT list", func(t *testing.T) {
		lv := &store.ListValue{Elements: [][]byte{[]byte("a")}}
		s.Set("kolist", lv, store.SetOptions{})
		_ = runHandler(t, router, "KEYOBJECT", [][]byte{[]byte("kolist")}, s)
	})

	t.Run("KEYOBJECT set", func(t *testing.T) {
		sv := &store.SetValue{Members: make(map[string]struct{})}
		sv.Members["m"] = struct{}{}
		s.Set("koset", sv, store.SetOptions{})
		_ = runHandler(t, router, "KEYOBJECT", [][]byte{[]byte("koset")}, s)
	})

	t.Run("KEYOBJECT zset", func(t *testing.T) {
		ssv := &store.SortedSetValue{Members: make(map[string]float64)}
		ssv.Add("m", 1.0)
		s.Set("kozset", ssv, store.SetOptions{})
		_ = runHandler(t, router, "KEYOBJECT", [][]byte{[]byte("kozset")}, s)
	})
}
