package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestLowCoverageBatch11_ExtraRouteCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtraCommands(router)

	t.Run("ROUTE.ADD and ROUTE.REMOVE", func(t *testing.T) {
		_ = runHandler(t, router, "ROUTE.ADD", [][]byte{[]byte("routetable"), []byte("users:*"), []byte("shard1")}, s)
		_ = runHandler(t, router, "ROUTE.REMOVE", [][]byte{[]byte("routetable"), []byte("users:*")}, s)
	})

	t.Run("ROUTE.REMOVE non-existent", func(t *testing.T) {
		_ = runHandler(t, router, "ROUTE.REMOVE", [][]byte{[]byte("nonexistent"), []byte("pattern")}, s)
	})

	t.Run("ROUTE.MATCH with match", func(t *testing.T) {
		_ = runHandler(t, router, "ROUTE.ADD", [][]byte{[]byte("routetable2"), []byte("users:*"), []byte("shard1")}, s)
		_ = runHandler(t, router, "ROUTE.MATCH", [][]byte{[]byte("routetable2"), []byte("users:123")}, s)
	})

	t.Run("ROUTE.MATCH no routes", func(t *testing.T) {
		_ = runHandler(t, router, "ROUTE.MATCH", [][]byte{[]byte("notable"), []byte("users:123")}, s)
	})

	t.Run("ROUTE.MATCH exact match", func(t *testing.T) {
		_ = runHandler(t, router, "ROUTE.ADD", [][]byte{[]byte("routetable3"), []byte("exactkey"), []byte("shard2")}, s)
		_ = runHandler(t, router, "ROUTE.MATCH", [][]byte{[]byte("routetable3"), []byte("exactkey")}, s)
	})

	t.Run("ROUTE.LIST", func(t *testing.T) {
		_ = runHandler(t, router, "ROUTE.ADD", [][]byte{[]byte("listtable"), []byte("pattern1"), []byte("target1")}, s)
		_ = runHandler(t, router, "ROUTE.LIST", [][]byte{[]byte("listtable")}, s)
	})
}

func TestLowCoverageBatch11_VectorClockCompare(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtraCommands(router)

	t.Run("VECTOR_CLOCK.COMPARE equal", func(t *testing.T) {
		_ = runHandler(t, router, "VECTOR_CLOCK.CREATE", [][]byte{[]byte("vc1")}, s)
		_ = runHandler(t, router, "VECTOR_CLOCK.INCREMENT", [][]byte{[]byte("vc1"), []byte("node1")}, s)
		_ = runHandler(t, router, "VECTOR_CLOCK.CREATE", [][]byte{[]byte("vc2")}, s)
		_ = runHandler(t, router, "VECTOR_CLOCK.INCREMENT", [][]byte{[]byte("vc2"), []byte("node1")}, s)
		_ = runHandler(t, router, "VECTOR_CLOCK.COMPARE", [][]byte{[]byte("vc1"), []byte("vc2")}, s)
	})

	t.Run("VECTOR_CLOCK.COMPARE with different clocks", func(t *testing.T) {
		_ = runHandler(t, router, "VECTOR_CLOCK.CREATE", [][]byte{[]byte("vc3")}, s)
		_ = runHandler(t, router, "VECTOR_CLOCK.INCREMENT", [][]byte{[]byte("vc3"), []byte("node1")}, s)
		_ = runHandler(t, router, "VECTOR_CLOCK.INCREMENT", [][]byte{[]byte("vc3"), []byte("node1")}, s)
		_ = runHandler(t, router, "VECTOR_CLOCK.CREATE", [][]byte{[]byte("vc4")}, s)
		_ = runHandler(t, router, "VECTOR_CLOCK.INCREMENT", [][]byte{[]byte("vc4"), []byte("node2")}, s)
		_ = runHandler(t, router, "VECTOR_CLOCK.COMPARE", [][]byte{[]byte("vc3"), []byte("vc4")}, s)
	})
}

func TestLowCoverageBatch11_FunctionCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterFunctionCommands(router)

	t.Run("FUNCTION.LOAD Lua", func(t *testing.T) {
		_ = runHandler(t, router, "FUNCTION.LOAD", [][]byte{
			[]byte("lua_func1"),
			[]byte("Lua"),
			[]byte("return redis.call('GET', KEYS[1])"),
		}, s)
	})

	t.Run("FUNCTION.LOAD JavaScript", func(t *testing.T) {
		_ = runHandler(t, router, "FUNCTION.LOAD", [][]byte{
			[]byte("js_func1"),
			[]byte("JavaScript"),
			[]byte("return 'hello';"),
		}, s)
	})

	t.Run("FCALL with args", func(t *testing.T) {
		_ = runHandler(t, router, "FUNCTION.LOAD", [][]byte{
			[]byte("lua_func2"),
			[]byte("Lua"),
			[]byte("return ARGV[1]"),
		}, s)
		_ = runHandler(t, router, "FCALL", [][]byte{[]byte("lua_func2"), []byte("0"), []byte("arg1")}, s)
	})

	t.Run("FCALL with keys", func(t *testing.T) {
		s.Set("fcallkey1", &store.StringValue{Data: []byte("value1")}, store.SetOptions{})
		_ = runHandler(t, router, "FUNCTION.LOAD", [][]byte{
			[]byte("lua_func3"),
			[]byte("Lua"),
			[]byte("return redis.call('GET', KEYS[1])"),
		}, s)
		_ = runHandler(t, router, "FCALL", [][]byte{[]byte("lua_func3"), []byte("1"), []byte("fcallkey1")}, s)
	})

	t.Run("FUNCTION.LIST", func(t *testing.T) {
		_ = runHandler(t, router, "FUNCTION.LIST", [][]byte{}, s)
	})

	t.Run("FUNCTION.DELETE", func(t *testing.T) {
		_ = runHandler(t, router, "FUNCTION.LOAD", [][]byte{
			[]byte("lua_func4"),
			[]byte("Lua"),
			[]byte("return 1"),
		}, s)
		_ = runHandler(t, router, "FUNCTION.DELETE", [][]byte{[]byte("lua_func4")}, s)
	})
}

func TestLowCoverageBatch11_MonitoringCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMonitoringCommands(router)

	t.Run("METRICS with details", func(t *testing.T) {
		_ = runHandler(t, router, "METRICS", [][]byte{}, s)
	})

	t.Run("INFO all sections", func(t *testing.T) {
		_ = runHandler(t, router, "INFO", [][]byte{[]byte("all")}, s)
	})

	t.Run("INFO default", func(t *testing.T) {
		_ = runHandler(t, router, "INFO", [][]byte{[]byte("default")}, s)
	})

	t.Run("INFO server", func(t *testing.T) {
		_ = runHandler(t, router, "INFO", [][]byte{[]byte("server")}, s)
	})

	t.Run("DBSIZE", func(t *testing.T) {
		_ = runHandler(t, router, "DBSIZE", [][]byte{}, s)
	})

	t.Run("LASTSAVE", func(t *testing.T) {
		_ = runHandler(t, router, "LASTSAVE", [][]byte{}, s)
	})

	t.Run("TIME", func(t *testing.T) {
		_ = runHandler(t, router, "TIME", [][]byte{}, s)
	})
}

func TestLowCoverageBatch11_MoreTraceCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	t.Run("TRACE.SPAN with start and end", func(t *testing.T) {
		_ = runHandler(t, router, "TRACE.SPAN", [][]byte{[]byte("span1"), []byte("start")}, s)
		_ = runHandler(t, router, "TRACE.SPAN", [][]byte{[]byte("span1"), []byte("end")}, s)
	})

	t.Run("TRACE.SPAN with tags", func(t *testing.T) {
		_ = runHandler(t, router, "TRACE.SPAN", [][]byte{[]byte("span2"), []byte("start"), []byte("service"), []byte("api")}, s)
	})

	t.Run("TRACE.LIST", func(t *testing.T) {
		_ = runHandler(t, router, "TRACE.LIST", [][]byte{}, s)
	})

	t.Run("TRACE.GET", func(t *testing.T) {
		_ = runHandler(t, router, "TRACE.SPAN", [][]byte{[]byte("span3"), []byte("start")}, s)
		_ = runHandler(t, router, "TRACE.GET", [][]byte{[]byte("span3")}, s)
	})

	t.Run("TRACE.DELETE", func(t *testing.T) {
		_ = runHandler(t, router, "TRACE.SPAN", [][]byte{[]byte("span4"), []byte("start")}, s)
		_ = runHandler(t, router, "TRACE.DELETE", [][]byte{[]byte("span4")}, s)
	})
}

func TestLowCoverageBatch11_MoreQuotaCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	t.Run("QUOTA.XCREATE with options", func(t *testing.T) {
		_ = runHandler(t, router, "QUOTA.XCREATE", [][]byte{[]byte("quota1"), []byte("1000"), []byte("3600"), []byte("strict")}, s)
	})

	t.Run("QUOTA.XCHECK", func(t *testing.T) {
		_ = runHandler(t, router, "QUOTA.XCREATE", [][]byte{[]byte("quota2"), []byte("100"), []byte("60")}, s)
		_ = runHandler(t, router, "QUOTA.XCHECK", [][]byte{[]byte("quota2"), []byte("user1")}, s)
	})

	t.Run("QUOTA.XSTATUS", func(t *testing.T) {
		_ = runHandler(t, router, "QUOTA.XCREATE", [][]byte{[]byte("quota3"), []byte("100"), []byte("60")}, s)
		_ = runHandler(t, router, "QUOTA.XSTATUS", [][]byte{[]byte("quota3")}, s)
	})

	t.Run("QUOTA.XRESET", func(t *testing.T) {
		_ = runHandler(t, router, "QUOTA.XCREATE", [][]byte{[]byte("quota4"), []byte("100"), []byte("60")}, s)
		_ = runHandler(t, router, "QUOTA.XRESET", [][]byte{[]byte("quota4")}, s)
	})
}

func TestLowCoverageBatch11_MoreMeterCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	t.Run("METER.CREATE with tags", func(t *testing.T) {
		_ = runHandler(t, router, "METER.CREATE", [][]byte{[]byte("meter1"), []byte("api"), []byte("requests")}, s)
	})

	t.Run("METER.RECORD", func(t *testing.T) {
		_ = runHandler(t, router, "METER.CREATE", [][]byte{[]byte("meter2")}, s)
		_ = runHandler(t, router, "METER.RECORD", [][]byte{[]byte("meter2"), []byte("100")}, s)
	})

	t.Run("METER.GET", func(t *testing.T) {
		_ = runHandler(t, router, "METER.CREATE", [][]byte{[]byte("meter3")}, s)
		_ = runHandler(t, router, "METER.RECORD", [][]byte{[]byte("meter3"), []byte("50")}, s)
		_ = runHandler(t, router, "METER.GET", [][]byte{[]byte("meter3")}, s)
	})

	t.Run("METER.LIST", func(t *testing.T) {
		_ = runHandler(t, router, "METER.LIST", [][]byte{}, s)
	})

	t.Run("METER.DELETE", func(t *testing.T) {
		_ = runHandler(t, router, "METER.CREATE", [][]byte{[]byte("meter4")}, s)
		_ = runHandler(t, router, "METER.DELETE", [][]byte{[]byte("meter4")}, s)
	})
}

func TestLowCoverageBatch11_MoreTenantCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	t.Run("TENANT.CREATE with config", func(t *testing.T) {
		_ = runHandler(t, router, "TENANT.CREATE", [][]byte{[]byte("tenant1"), []byte("maxkeys"), []byte("1000")}, s)
	})

	t.Run("TENANT.INFO", func(t *testing.T) {
		_ = runHandler(t, router, "TENANT.CREATE", [][]byte{[]byte("tenant2")}, s)
		_ = runHandler(t, router, "TENANT.INFO", [][]byte{[]byte("tenant2")}, s)
	})

	t.Run("TENANT.LIST", func(t *testing.T) {
		_ = runHandler(t, router, "TENANT.LIST", [][]byte{}, s)
	})

	t.Run("TENANT.DELETE", func(t *testing.T) {
		_ = runHandler(t, router, "TENANT.CREATE", [][]byte{[]byte("tenant3")}, s)
		_ = runHandler(t, router, "TENANT.DELETE", [][]byte{[]byte("tenant3")}, s)
	})
}

func TestLowCoverageBatch11_MoreLeaseCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	t.Run("LEASE.CREATE with TTL", func(t *testing.T) {
		_ = runHandler(t, router, "LEASE.CREATE", [][]byte{[]byte("lease1"), []byte("3600")}, s)
	})

	t.Run("LEASE.GET", func(t *testing.T) {
		_ = runHandler(t, router, "LEASE.CREATE", [][]byte{[]byte("lease2"), []byte("60")}, s)
		_ = runHandler(t, router, "LEASE.GET", [][]byte{[]byte("lease2")}, s)
	})

	t.Run("LEASE.LIST", func(t *testing.T) {
		_ = runHandler(t, router, "LEASE.LIST", [][]byte{}, s)
	})

	t.Run("LEASE.DELETE", func(t *testing.T) {
		_ = runHandler(t, router, "LEASE.CREATE", [][]byte{[]byte("lease3"), []byte("60")}, s)
		_ = runHandler(t, router, "LEASE.DELETE", [][]byte{[]byte("lease3")}, s)
	})
}

func TestLowCoverageBatch11_MoreSketchCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	t.Run("SKETCH.CREATE with options", func(t *testing.T) {
		_ = runHandler(t, router, "SKETCH.CREATE", [][]byte{[]byte("sketch1"), []byte("0.01")}, s)
	})

	t.Run("SKETCH.UPDATE multiple", func(t *testing.T) {
		_ = runHandler(t, router, "SKETCH.CREATE", [][]byte{[]byte("sketch2")}, s)
		_ = runHandler(t, router, "SKETCH.UPDATE", [][]byte{[]byte("sketch2"), []byte("item1")}, s)
		_ = runHandler(t, router, "SKETCH.UPDATE", [][]byte{[]byte("sketch2"), []byte("item2")}, s)
	})

	t.Run("SKETCH.QUERY", func(t *testing.T) {
		_ = runHandler(t, router, "SKETCH.CREATE", [][]byte{[]byte("sketch3")}, s)
		_ = runHandler(t, router, "SKETCH.UPDATE", [][]byte{[]byte("sketch3"), []byte("item1")}, s)
		_ = runHandler(t, router, "SKETCH.QUERY", [][]byte{[]byte("sketch3"), []byte("item1")}, s)
	})

	t.Run("SKETCH.DELETE", func(t *testing.T) {
		_ = runHandler(t, router, "SKETCH.CREATE", [][]byte{[]byte("sketch4")}, s)
		_ = runHandler(t, router, "SKETCH.DELETE", [][]byte{[]byte("sketch4")}, s)
	})
}

func TestLowCoverageBatch11_MorePartitionCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	t.Run("PARTITION.ADD with options", func(t *testing.T) {
		_ = runHandler(t, router, "PARTITION.ADD", [][]byte{[]byte("part1"), []byte("node1"), []byte("0"), []byte("1000")}, s)
	})

	t.Run("PARTITION.LIST", func(t *testing.T) {
		_ = runHandler(t, router, "PARTITION.LIST", [][]byte{}, s)
	})

	t.Run("PARTITION.GET", func(t *testing.T) {
		_ = runHandler(t, router, "PARTITION.ADD", [][]byte{[]byte("part2"), []byte("node1")}, s)
		_ = runHandler(t, router, "PARTITION.GET", [][]byte{[]byte("part2")}, s)
	})

	t.Run("PARTITION.REMOVE", func(t *testing.T) {
		_ = runHandler(t, router, "PARTITION.ADD", [][]byte{[]byte("part3"), []byte("node1")}, s)
		_ = runHandler(t, router, "PARTITION.REMOVE", [][]byte{[]byte("part3")}, s)
	})
}

func TestLowCoverageBatch11_MVCCSpatialCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMVCCCommands(router)

	t.Run("SPATIAL.WITHIN with results", func(t *testing.T) {
		_ = runHandler(t, router, "SPATIAL.CREATE", [][]byte{[]byte("spatial1")}, s)
		_ = runHandler(t, router, "SPATIAL.ADD", [][]byte{[]byte("spatial1"), []byte("point1"), []byte("0"), []byte("0")}, s)
		_ = runHandler(t, router, "SPATIAL.WITHIN", [][]byte{[]byte("spatial1"), []byte("0"), []byte("0"), []byte("100")}, s)
	})

	t.Run("SPATIAL.CREATE with options", func(t *testing.T) {
		_ = runHandler(t, router, "SPATIAL.CREATE", [][]byte{[]byte("spatial2"), []byte("10")}, s)
	})
}

func TestLowCoverageBatch11_MVCCRollupCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMVCCCommands(router)

	t.Run("ROLLUP.ADD with tags", func(t *testing.T) {
		_ = runHandler(t, router, "ROLLUP.ADD", [][]byte{[]byte("rollup1"), []byte("100"), []byte("hourly"), []byte("api")}, s)
	})

	t.Run("ROLLUP.GET with results", func(t *testing.T) {
		_ = runHandler(t, router, "ROLLUP.ADD", [][]byte{[]byte("rollup2"), []byte("50"), []byte("minute")}, s)
		_ = runHandler(t, router, "ROLLUP.GET", [][]byte{[]byte("rollup2")}, s)
	})

	t.Run("ROLLUP.LIST", func(t *testing.T) {
		_ = runHandler(t, router, "ROLLUP.LIST", [][]byte{}, s)
	})

	t.Run("ROLLUP.DELETE", func(t *testing.T) {
		_ = runHandler(t, router, "ROLLUP.ADD", [][]byte{[]byte("rollup3"), []byte("10"), []byte("second")}, s)
		_ = runHandler(t, router, "ROLLUP.DELETE", [][]byte{[]byte("rollup3")}, s)
	})
}

func TestLowCoverageBatch11_MVCCQuotaCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMVCCCommands(router)

	t.Run("QUOTA.SET with options", func(t *testing.T) {
		_ = runHandler(t, router, "QUOTA.SET", [][]byte{[]byte("mvccquota1"), []byte("1000"), []byte("strict")}, s)
	})

	t.Run("QUOTA.GET", func(t *testing.T) {
		_ = runHandler(t, router, "QUOTA.SET", [][]byte{[]byte("mvccquota2"), []byte("500")}, s)
		_ = runHandler(t, router, "QUOTA.GET", [][]byte{[]byte("mvccquota2")}, s)
	})
}

func TestLowCoverageBatch11_ResilienceDiagnosticCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	t.Run("DIAGNOSTIC.RUN with checks", func(t *testing.T) {
		_ = runHandler(t, router, "DIAGNOSTIC.RUN", [][]byte{[]byte("diag1"), []byte("all")}, s)
	})

	t.Run("DIAGNOSTIC.RUN specific check", func(t *testing.T) {
		_ = runHandler(t, router, "DIAGNOSTIC.RUN", [][]byte{[]byte("diag2"), []byte("memory")}, s)
	})

	t.Run("DIAGNOSTIC.LIST", func(t *testing.T) {
		_ = runHandler(t, router, "DIAGNOSTIC.LIST", [][]byte{}, s)
	})
}

func TestLowCoverageBatch11_ResilienceMemoryCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	t.Run("MEMORYX.FREE", func(t *testing.T) {
		_ = runHandler(t, router, "MEMORYX.FREE", [][]byte{}, s)
	})

	t.Run("MEMORYX.STATS with details", func(t *testing.T) {
		_ = runHandler(t, router, "MEMORYX.STATS", [][]byte{[]byte("detailed")}, s)
	})

	t.Run("MEMORYX.USAGE", func(t *testing.T) {
		_ = runHandler(t, router, "MEMORYX.USAGE", [][]byte{}, s)
	})
}

func TestLowCoverageBatch11_SchedulerSessionCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterSchedulerCommands(router)

	t.Run("SESSION.REFRESH with data", func(t *testing.T) {
		_ = runHandler(t, router, "SESSION.CREATE", [][]byte{[]byte("session1"), []byte("3600")}, s)
		_ = runHandler(t, router, "SESSION.REFRESH", [][]byte{[]byte("session1")}, s)
	})

	t.Run("SESSION.GET", func(t *testing.T) {
		_ = runHandler(t, router, "SESSION.CREATE", [][]byte{[]byte("session2"), []byte("60")}, s)
		_ = runHandler(t, router, "SESSION.GET", [][]byte{[]byte("session2")}, s)
	})

	t.Run("SESSION.DELETE", func(t *testing.T) {
		_ = runHandler(t, router, "SESSION.CREATE", [][]byte{[]byte("session3"), []byte("60")}, s)
		_ = runHandler(t, router, "SESSION.DELETE", [][]byte{[]byte("session3")}, s)
	})
}

func TestLowCoverageBatch11_StatsCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStatsCommands(router)

	t.Run("SAMPLE.CREATE with tags", func(t *testing.T) {
		_ = runHandler(t, router, "SAMPLE.CREATE", [][]byte{[]byte("sample1"), []byte("api"), []byte("requests")}, s)
	})

	t.Run("SAMPLE.RECORD", func(t *testing.T) {
		_ = runHandler(t, router, "SAMPLE.CREATE", [][]byte{[]byte("sample2")}, s)
		_ = runHandler(t, router, "SAMPLE.RECORD", [][]byte{[]byte("sample2"), []byte("100")}, s)
	})

	t.Run("SAMPLE.GET", func(t *testing.T) {
		_ = runHandler(t, router, "SAMPLE.CREATE", [][]byte{[]byte("sample3")}, s)
		_ = runHandler(t, router, "SAMPLE.RECORD", [][]byte{[]byte("sample3"), []byte("50")}, s)
		_ = runHandler(t, router, "SAMPLE.GET", [][]byte{[]byte("sample3")}, s)
	})

	t.Run("HISTOGRAM.CREATE with options", func(t *testing.T) {
		_ = runHandler(t, router, "HISTOGRAM.CREATE", [][]byte{[]byte("hist1"), []byte("linear")}, s)
	})

	t.Run("HISTOGRAM.OBSERVE", func(t *testing.T) {
		_ = runHandler(t, router, "HISTOGRAM.CREATE", [][]byte{[]byte("hist2")}, s)
		_ = runHandler(t, router, "HISTOGRAM.OBSERVE", [][]byte{[]byte("hist2"), []byte("50")}, s)
	})

	t.Run("HISTOGRAM.GET", func(t *testing.T) {
		_ = runHandler(t, router, "HISTOGRAM.CREATE", [][]byte{[]byte("hist3")}, s)
		_ = runHandler(t, router, "HISTOGRAM.OBSERVE", [][]byte{[]byte("hist3"), []byte("100")}, s)
		_ = runHandler(t, router, "HISTOGRAM.GET", [][]byte{[]byte("hist3")}, s)
	})
}

func TestLowCoverageBatch11_ServerDumpRestoreCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterServerCommands(router)

	t.Run("DUMP all types", func(t *testing.T) {
		s.Set("dumpstr", &store.StringValue{Data: []byte("value")}, store.SetOptions{})
		s.Set("dumphash", &store.HashValue{Fields: map[string][]byte{"f": []byte("v")}}, store.SetOptions{})
		sv := &store.SetValue{Members: map[string]struct{}{}}
		sv.Members["m"] = struct{}{}
		s.Set("dumpset", sv, store.SetOptions{})
		lv := &store.ListValue{Elements: [][]byte{[]byte("a"), []byte("b")}}
		s.Set("dumplist", lv, store.SetOptions{})
		ssv := &store.SortedSetValue{Members: map[string]float64{}}
		ssv.Add("m", 1.0)
		s.Set("dumpzset", ssv, store.SetOptions{})

		_ = runHandler(t, router, "DUMP", [][]byte{[]byte("dumpstr")}, s)
		_ = runHandler(t, router, "DUMP", [][]byte{[]byte("dumphash")}, s)
		_ = runHandler(t, router, "DUMP", [][]byte{[]byte("dumplist")}, s)
		_ = runHandler(t, router, "DUMP", [][]byte{[]byte("dumpset")}, s)
		_ = runHandler(t, router, "DUMP", [][]byte{[]byte("dumpzset")}, s)
	})

	t.Run("RESTORE all types", func(t *testing.T) {
		_ = runHandler(t, router, "RESTORE", [][]byte{
			[]byte("restorestr"),
			[]byte("0"),
			[]byte("CACHSTORM001" + string([]byte{0}) + "0:restored"),
		}, s)
		_ = runHandler(t, router, "RESTORE", [][]byte{
			[]byte("restorehash"),
			[]byte("0"),
			[]byte("CACHSTORM001" + string([]byte{2}) + "0:f=v&"),
		}, s)
	})
}

func TestLowCoverageBatch11_ServerCopyCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterServerCommands(router)

	t.Run("COPY all scenarios", func(t *testing.T) {
		s.Set("copysrc", &store.StringValue{Data: []byte("value")}, store.SetOptions{})
		_ = runHandler(t, router, "COPY", [][]byte{[]byte("copysrc"), []byte("copydst")}, s)
		_ = runHandler(t, router, "COPY", [][]byte{[]byte("copysrc"), []byte("copydst2"), []byte("REPLACE")}, s)
		_ = runHandler(t, router, "COPY", [][]byte{[]byte("nonexistent"), []byte("copydst3")}, s)
	})

	t.Run("COPY with TTL", func(t *testing.T) {
		s.Set("copyttl", &store.StringValue{Data: []byte("value")}, store.SetOptions{TTL: 60000})
		_ = runHandler(t, router, "COPY", [][]byte{[]byte("copyttl"), []byte("copyttldst"), []byte("REPLACE")}, s)
	})
}

func TestLowCoverageBatch11_UtilityExtFlagCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterUtilityExtCommands(router)

	t.Run("FLAG.ADDRULE with options", func(t *testing.T) {
		_ = runHandler(t, router, "FLAG.ADDRULE", [][]byte{[]byte("flag1"), []byte("percentage"), []byte("50"), []byte("true")}, s)
	})

	t.Run("FLAG.GETRULE", func(t *testing.T) {
		_ = runHandler(t, router, "FLAG.ADDRULE", [][]byte{[]byte("flag2"), []byte("percentage"), []byte("25")}, s)
		_ = runHandler(t, router, "FLAG.GETRULE", [][]byte{[]byte("flag2")}, s)
	})

	t.Run("FLAG.CHECK", func(t *testing.T) {
		_ = runHandler(t, router, "FLAG.ADDRULE", [][]byte{[]byte("flag3"), []byte("percentage"), []byte("50")}, s)
		_ = runHandler(t, router, "FLAG.CHECK", [][]byte{[]byte("flag3"), []byte("user1")}, s)
	})

	t.Run("FLAG.LIST", func(t *testing.T) {
		_ = runHandler(t, router, "FLAG.LIST", [][]byte{}, s)
	})
}

func TestLowCoverageBatch11_SortedSetCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterSortedSetCommands(router)

	t.Run("ZRANDMEMBER", func(t *testing.T) {
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
		ssv1.Add("b", 2.0)
		s.Set("zunion1", ssv1, store.SetOptions{})

		ssv2 := &store.SortedSetValue{Members: map[string]float64{}}
		ssv2.Add("c", 3.0)
		s.Set("zunion2", ssv2, store.SetOptions{})

		_ = runHandler(t, router, "ZUNIONSTORE", [][]byte{[]byte("zuniondest"), []byte("2"), []byte("zunion1"), []byte("zunion2")}, s)
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

		_ = runHandler(t, router, "ZINTERSTORE", [][]byte{[]byte("zinterdest"), []byte("2"), []byte("zinter1"), []byte("zinter2")}, s)
	})

	t.Run("ZDIFFSTORE", func(t *testing.T) {
		ssv1 := &store.SortedSetValue{Members: map[string]float64{}}
		ssv1.Add("a", 1.0)
		ssv1.Add("b", 2.0)
		s.Set("zdiff1", ssv1, store.SetOptions{})

		ssv2 := &store.SortedSetValue{Members: map[string]float64{}}
		ssv2.Add("a", 3.0)
		s.Set("zdiff2", ssv2, store.SetOptions{})

		_ = runHandler(t, router, "ZDIFFSTORE", [][]byte{[]byte("zdiffdest"), []byte("2"), []byte("zdiff1"), []byte("zdiff2")}, s)
	})
}

func TestLowCoverageBatch11_StreamCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStreamCommands(router)

	t.Run("XAUTOCLAIM", func(t *testing.T) {
		_ = runHandler(t, router, "XADD", [][]byte{[]byte("xacstream1"), []byte("*"), []byte("f"), []byte("v")}, s)
		_ = runHandler(t, router, "XAUTOCLAIM", [][]byte{
			[]byte("xacstream1"),
			[]byte("group1"),
			[]byte("consumer1"),
			[]byte("0"),
			[]byte("0-0"),
		}, s)
	})

	t.Run("XGROUP CREATE", func(t *testing.T) {
		_ = runHandler(t, router, "XADD", [][]byte{[]byte("xgstream1"), []byte("*"), []byte("f"), []byte("v")}, s)
		_ = runHandler(t, router, "XGROUP", [][]byte{[]byte("CREATE"), []byte("xgstream1"), []byte("group1"), []byte("$")}, s)
	})

	t.Run("XREADGROUP", func(t *testing.T) {
		_ = runHandler(t, router, "XADD", [][]byte{[]byte("xrgstream1"), []byte("*"), []byte("f"), []byte("v")}, s)
		_ = runHandler(t, router, "XGROUP", [][]byte{[]byte("CREATE"), []byte("xrgstream1"), []byte("group1"), []byte("$")}, s)
		_ = runHandler(t, router, "XREADGROUP", [][]byte{
			[]byte("GROUP"),
			[]byte("group1"),
			[]byte("consumer1"),
			[]byte("STREAMS"),
			[]byte("xrgstream1"),
			[]byte(">"),
		}, s)
	})
}

func TestLowCoverageBatch11_ClientTrackingCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterServerCommands(router)

	t.Run("CLIENT TRACKING on", func(t *testing.T) {
		_ = runHandler(t, router, "CLIENT", [][]byte{[]byte("TRACKING"), []byte("ON")}, s)
	})

	t.Run("CLIENT TRACKING off", func(t *testing.T) {
		_ = runHandler(t, router, "CLIENT", [][]byte{[]byte("TRACKING"), []byte("OFF")}, s)
	})

	t.Run("CLIENT CACHING", func(t *testing.T) {
		_ = runHandler(t, router, "CLIENT", [][]byte{[]byte("CACHING"), []byte("YES")}, s)
	})

	t.Run("CLIENT GETREDIR", func(t *testing.T) {
		_ = runHandler(t, router, "CLIENT", [][]byte{[]byte("GETREDIR")}, s)
	})
}

func TestLowCoverageBatch11_ServerCommandGetKeys(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterServerCommands(router)

	t.Run("COMMAND GETKEYS SET", func(t *testing.T) {
		_ = runHandler(t, router, "COMMAND", [][]byte{[]byte("GETKEYS"), []byte("SET"), []byte("key1"), []byte("value1")}, s)
	})

	t.Run("COMMAND GETKEYS GET", func(t *testing.T) {
		_ = runHandler(t, router, "COMMAND", [][]byte{[]byte("GETKEYS"), []byte("GET"), []byte("key1")}, s)
	})

	t.Run("COMMAND GETKEYS MSET", func(t *testing.T) {
		_ = runHandler(t, router, "COMMAND", [][]byte{[]byte("GETKEYS"), []byte("MSET"), []byte("k1"), []byte("v1"), []byte("k2"), []byte("v2")}, s)
	})
}

func TestLowCoverageBatch11_NamespaceInfoCommand(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterNamespaceCommands(router)

	t.Run("NAMESPACE.INFO detailed", func(t *testing.T) {
		_ = runHandler(t, router, "NAMESPACE.CREATE", [][]byte{[]byte("nsinfo1")}, s)
		_ = runHandler(t, router, "NAMESPACE.INFO", [][]byte{[]byte("nsinfo1")}, s)
	})
}

func TestLowCoverageBatch11_BackupRestoreCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterUtilityExtCommands(router)

	t.Run("BACKUP.RESTORE", func(t *testing.T) {
		_ = runHandler(t, router, "BACKUP.RESTORE", [][]byte{[]byte("backup1")}, s)
	})
}

func TestLowCoverageBatch11_TemplateCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterWorkflowCommands(router)

	t.Run("TEMPLATE.GET", func(t *testing.T) {
		_ = runHandler(t, router, "TEMPLATE.CREATE", [][]byte{[]byte("tpl1"), []byte("Hello {{name}}")}, s)
		_ = runHandler(t, router, "TEMPLATE.GET", [][]byte{[]byte("tpl1"), []byte("{\"name\":\"World\"}")}, s)
	})
}
