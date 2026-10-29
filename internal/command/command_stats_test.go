package command

import (
	"regexp"
	"strings"
	"testing"
)

func TestLatencyHistogramMatchesRedis(t *testing.T) {
	// Redis records whole microseconds; the values below are what Redis 7.4
	// reports for single calls of that duration.
	for _, c := range []struct {
		usec, p50, bucket int64
	}{
		{0, 1, 1},
		{4, 4015, 4},
		{6, 6015, 8},
		{13, 13055, 16},
		{178, 178175, 264},
	} {
		var h latencyHistogram
		h.record(c.usec * 1000)
		if got := h.percentile(50); got != c.p50 {
			t.Errorf("%dus: p50 %d, want %d", c.usec, got, c.p50)
		}
		cdf := h.cdf()
		points := cdf.Array[3].Array
		if len(points) != 2 || points[0].Int != c.bucket || points[1].Int != 1 {
			t.Errorf("%dus: histogram %+v, want {%d: 1}", c.usec, points, c.bucket)
		}
	}
	var h latencyHistogram
	for _, us := range []int64{13, 37, 50} {
		h.record(us * 1000)
	}
	points := h.cdf().Array[3].Array
	var got []int64
	for _, p := range points {
		got = append(got, p.Int)
	}
	if want := []int64{16, 1, 66, 3}; len(got) != 4 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] || got[3] != want[3] {
		t.Errorf("cumulative histogram %v, want %v", got, want)
	}
	if got := h.percentile(99); got != 50175 {
		t.Errorf("p99 %d, want 50175", got)
	}
}

func TestCommandAndErrorStats(t *testing.T) {
	h := newLuaHarness(t)
	RegisterConfigCommands(h.router)
	h.do("CONFIG", "RESETSTAT")
	t.Cleanup(resetCommandStats)
	h.do("SET", "a", "1")
	h.do("GET", "a")
	h.do("GET")
	h.do("LPUSH", "a", "x")
	h.do("CONFIG", "GET", "maxmemory")
	h.do("CONFIG", "NOSUCH")
	h.do("EVAL", "return redis.call('lpush','a')", "0")
	h.do("EVAL", "return redis.pcall('lpush','a','x')", "0")
	h.do("EVAL", "return redis.call('lpush','a','x')", "0")
	h.do("EVAL", "return redis.error_reply('MY bad')", "0")

	info := h.do("INFO", "commandstats", "errorstats")
	usec := regexp.MustCompile(`usec=\d+,usec_per_call=[\d.]+`)
	info = usec.ReplaceAllString(info, "U")
	for _, want := range []string{
		"cmdstat_set:calls=1,U,rejected_calls=0,failed_calls=0",
		"cmdstat_get:calls=1,U,rejected_calls=1,failed_calls=0",
		"cmdstat_lpush:calls=3,U,rejected_calls=1,failed_calls=3",
		"cmdstat_eval:calls=4,U,rejected_calls=0,failed_calls=4",
		"cmdstat_config|get:calls=1,U,rejected_calls=0,failed_calls=0",
		"errorstat_ERR:count=3",
		"errorstat_MY:count=1",
		"errorstat_WRONGTYPE:count=3",
	} {
		if !strings.Contains(info, want+"\r\n") {
			t.Errorf("INFO lacks %q:\n%s", want, info)
		}
	}
	if strings.Contains(info, "config|nosuch") {
		t.Errorf("an unknown subcommand got statistics:\n%s", info)
	}
	if got := h.do("INFO"); strings.Contains(got, "# Commandstats") || !strings.Contains(got, "# Errorstats") {
		t.Errorf("default INFO sections:\n%s", got)
	}

	hist := h.do("LATENCY", "HISTOGRAM", "config", "set", "nosuch")
	if !strings.HasPrefix(hist, "*6\r\n$10\r\nconfig|get\r\n*4\r\n$5\r\ncalls\r\n:1\r\n$14\r\nhistogram_usec\r\n") || !strings.Contains(hist, "$3\r\nset\r\n") {
		t.Errorf("LATENCY HISTOGRAM: %q", hist)
	}
	if got := h.do("INFO", "latencystats"); !regexp.MustCompile(`latency_percentiles_usec_set:p50=\d+\.\d{3},p99=\d+\.\d{3},p99\.9=\d+\.\d{3}\r\n`).MatchString(got) {
		t.Errorf("INFO latencystats:\n%s", got)
	}

	h.do("CONFIG", "RESETSTAT")
	if got := h.do("INFO", "errorstats"); strings.Contains(got, "errorstat_") {
		t.Errorf("CONFIG RESETSTAT kept error stats:\n%s", got)
	}
}

func TestDirtyCountsSuccessfulWrites(t *testing.T) {
	h := newLuaHarness(t)
	start := Dirty()
	h.do("SET", "d", "1")
	h.do("GET", "d")
	h.do("LPUSH", "d", "x")
	h.do("APPEND", "d", "2")
	if got := Dirty() - start; got != 2 {
		t.Fatalf("dirty grew by %d, want 2", got)
	}
	SaveDone(2)
	if Dirty() != start {
		t.Fatalf("SaveDone left %d, want %d", Dirty(), start)
	}
}

func TestMemoryStatsFields(t *testing.T) {
	h := newLuaHarness(t)
	RegisterDebugCommands(h.router)
	h.do("SET", "a", "1")
	got := h.do("MEMORY", "STATS")
	var names []string
	for _, line := range strings.Split(got, "\r\n") {
		if strings.ContainsAny(line, "abcdefghijklmnopqrstuvwxyz") && !strings.HasPrefix(line, "$") {
			names = append(names, line)
		}
	}
	want := "peak.allocated total.allocated startup.allocated replication.backlog clients.slaves clients.normal cluster.links aof.buffer lua.caches functions.caches db.0 overhead.hashtable.main overhead.hashtable.expires overhead.db.hashtable.lut overhead.db.hashtable.rehashing overhead.total db.dict.rehashing.count keys.count keys.bytes-per-key dataset.bytes dataset.percentage peak.percentage allocator.allocated allocator.active allocator.resident allocator.muzzy allocator-fragmentation.ratio allocator-fragmentation.bytes allocator-rss.ratio allocator-rss.bytes rss-overhead.ratio rss-overhead.bytes fragmentation fragmentation.bytes"
	if strings.Join(names, " ") != want {
		t.Fatalf("MEMORY STATS fields:\n got: %s\nwant: %s", strings.Join(names, " "), want)
	}
}
