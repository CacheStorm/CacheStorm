package command

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func resetStatRun(t *testing.T, s *store.Store, args ...string) *resp.Value {
	t.Helper()
	router := NewRouter()
	RegisterConfigCommands(router)
	RegisterMonitoringCommands(router)
	RegisterKeyCommands(router)

	var buf bytes.Buffer
	ctx := NewContext(args[0], resetStatBytes(args[1:]), s, resp.NewWriter(&buf))
	if err := router.Execute(ctx); err != nil {
		t.Fatalf("%s: Execute returned error: %v", args[0], err)
	}
	v, err := resp.NewReader(bytes.NewReader(buf.Bytes())).ReadValue()
	if err != nil {
		t.Fatalf("%s: could not parse RESP reply %q: %v", args[0], buf.String(), err)
	}
	return v
}

func resetStatBytes(ss []string) [][]byte {
	out := make([][]byte, len(ss))
	for i, s := range ss {
		out[i] = []byte(s)
	}
	return out
}

// CONFIG RESETSTAT must zero the command counters. The handler used to reply
// +OK with an empty body — store.GlobalMetrics.Reset() was never called — so a
// client that trusted the +OK kept reading stale, ever-growing statistics.
func TestConfigResetStatActuallyResetsCounters(t *testing.T) {
	s := store.NewStore()

	// GlobalMetrics is a process-wide global shared across tests in this
	// binary, so establish a known-zero baseline first.
	store.GlobalMetrics.Reset()
	for i := 0; i < 5; i++ {
		store.GlobalMetrics.RecordCommand("GET", 1000)
	}
	if before := store.GlobalMetrics.TotalCommands.Load(); before != 5 {
		t.Fatalf("setup: total_commands = %d, want 5", before)
	}

	if v := resetStatRun(t, s, "CONFIG", "RESETSTAT"); v.Type != resp.TypeSimpleString {
		t.Fatalf("CONFIG RESETSTAT returned type %v, want a simple string", v.Type)
	}

	if after := store.GlobalMetrics.TotalCommands.Load(); after != 0 {
		t.Fatalf("CONFIG RESETSTAT replied +OK but total_commands is still %d, want 0", after)
	}
}

// Other counters must reset too, not just total_commands.
func TestConfigResetStatResetsOtherCounters(t *testing.T) {
	s := store.NewStore()

	store.GlobalMetrics.Reset()
	store.GlobalMetrics.RecordRead()
	store.GlobalMetrics.RecordRead()
	store.GlobalMetrics.RecordWrite()
	store.GlobalMetrics.RecordError()

	resetStatRun(t, s, "CONFIG", "RESETSTAT")

	if v := store.GlobalMetrics.TotalReads.Load(); v != 0 {
		t.Fatalf("TotalReads = %d after RESETSTAT, want 0", v)
	}
	if v := store.GlobalMetrics.TotalWrites.Load(); v != 0 {
		t.Fatalf("TotalWrites = %d after RESETSTAT, want 0", v)
	}
	if v := store.GlobalMetrics.TotalErrors.Load(); v != 0 {
		t.Fatalf("TotalErrors = %d after RESETSTAT, want 0", v)
	}
}

// Counters must still increment after a reset — the fix resets, it does not
// wedge the counter.
func TestCountersStillIncrementAfterResetControl(t *testing.T) {
	s := store.NewStore()

	store.GlobalMetrics.Reset()
	resetStatRun(t, s, "CONFIG", "RESETSTAT")
	store.GlobalMetrics.RecordCommand("GET", 1000)

	if after := store.GlobalMetrics.TotalCommands.Load(); after != 1 {
		t.Fatalf("after RESETSTAT one recorded command left total_commands at %d, want 1", after)
	}
}

// The sibling METRICS.RESET must keep doing the same reset — this is the
// in-repo basis for what CONFIG RESETSTAT is expected to do.
func TestMetricsResetSiblingStillResetsControl(t *testing.T) {
	s := store.NewStore()

	store.GlobalMetrics.Reset()
	store.GlobalMetrics.RecordCommand("GET", 1000)

	if v := resetStatRun(t, s, "METRICS.RESET"); v.Type != resp.TypeSimpleString {
		t.Fatalf("METRICS.RESET returned type %v, want a simple string", v.Type)
	}
	if after := store.GlobalMetrics.TotalCommands.Load(); after != 0 {
		t.Fatalf("METRICS.RESET left total_commands at %d, want 0", after)
	}
}

// Control: CONFIG GET is unaffected, and the statistics it reports still
// reflect recorded work after a reset.
func TestConfigGetAndStatsRemainUsableControl(t *testing.T) {
	s := store.NewStore()

	store.GlobalMetrics.Reset()
	store.GlobalMetrics.RecordCommand("GET", 1000)

	if v := resetStatRun(t, s, "CONFIG", "GET", "maxmemory"); v.Type == resp.TypeError {
		t.Fatalf("control: CONFIG GET maxmemory returned %q", v.Err)
	}
	if v := resetStatRun(t, s, "STATS.ALL"); v.Type != resp.TypeArray {
		t.Fatalf("control: STATS.ALL returned type %v, want an array", v.Type)
	}

	resetStatRun(t, s, "CONFIG", "RESETSTAT")

	if v := resetStatRun(t, s, "STATS.ALL"); v.Type != resp.TypeArray {
		t.Fatalf("control: STATS.ALL after RESETSTAT returned type %v, want an array", v.Type)
	}
}
