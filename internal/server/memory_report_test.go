package server

import (
	"strconv"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
)

func TestMemoryPeakStatsAndDoctor(t *testing.T) {
	s := startReplServer(t, nil)
	c := dialRepl(t, s)
	big := strings.Repeat("x", 1<<20)
	for i := 0; i < 8; i++ {
		c.do("SET", "big:"+strconv.Itoa(i), big)
	}
	for i := 0; i < 8; i++ {
		c.do("DEL", "big:"+strconv.Itoa(i))
	}
	mem := c.info("memory")
	used, _ := strconv.ParseInt(mem["used_memory"], 10, 64)
	peak, _ := strconv.ParseInt(mem["used_memory_peak"], 10, 64)
	if peak < 8<<20 || peak <= used {
		t.Fatalf("used_memory_peak = %d with used_memory %d, want the 8 MiB peak kept", peak, used)
	}
	if rss, _ := strconv.ParseInt(mem["used_memory_rss"], 10, 64); rss <= 0 {
		t.Fatalf("used_memory_rss = %q", mem["used_memory_rss"])
	}
	if mem["mem_fragmentation_ratio"] == "" || mem["used_memory_peak_perc"] == "" {
		t.Fatalf("INFO memory = %v", mem)
	}

	stats := c.do("MEMORY", "STATS")
	if stats.Type != resp.TypeArray || len(stats.Array)%2 != 0 {
		t.Fatalf("MEMORY STATS = %+v", stats)
	}
	got := map[string]*resp.Value{}
	for i := 0; i+1 < len(stats.Array); i += 2 {
		got[string(stats.Array[i].Bulk)] = stats.Array[i+1]
	}
	if got["peak.allocated"] == nil || got["peak.allocated"].Int < peak {
		t.Fatalf("MEMORY STATS peak.allocated = %+v, INFO peak %d", got["peak.allocated"], peak)
	}
	if got["keys.count"] == nil || got["keys.count"].Int != 0 {
		t.Fatalf("MEMORY STATS keys.count = %+v", got["keys.count"])
	}

	if doc := string(c.do("MEMORY", "DOCTOR").Bulk); !strings.Contains(doc, "very little memory") {
		t.Fatalf("MEMORY DOCTOR on an empty instance = %q", doc)
	}

	c.do("CONFIG", "RESETSTAT")
	after, _ := strconv.ParseInt(c.info("memory")["used_memory_peak"], 10, 64)
	if after >= peak {
		t.Fatalf("used_memory_peak after CONFIG RESETSTAT = %d, want it reset below %d", after, peak)
	}
}
