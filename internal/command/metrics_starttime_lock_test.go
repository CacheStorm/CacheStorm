package command

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression: store.GlobalMetrics.StartTime is a plain time.Time -- a
// multi-word struct (wall uint64, ext int64, loc *Location) -- and one command
// reads it directly off the global, bypassing the mutex.
//
// WRITER — internal/store/metrics.go:224, inside Metrics.Reset(), LOCKED:
//	    m.mu.Lock()
//	    m.StartTime = time.Now()
//	    ...
//	    m.mu.Unlock()
//
// Bypassing READER — internal/command/monitoring_commands.go:264, cmdHEALTHCHECK,
// NO lock:
//	    resp.IntegerValue(int64(time.Since(store.GlobalMetrics.StartTime).Seconds()))
//
// Metrics.Reset() is client-reachable from two commands:
//	    internal/command/config_commands.go:276      CONFIG RESETSTAT
//	    internal/command/monitoring_commands.go:56    SLOWLOG.RESET / INFO RESET
//
// So `CONFIG RESETSTAT` on one connection races `HEALTHCHECK` on another while
// the race detector reports a torn read of a multi-word struct.
//
// Every OTHER StartTime reader is correctly locked -- Metrics.Snapshot() takes
// m.mu.RLock() before reading at metrics.go:189 -- which is what makes this one
// an outlier rather than a design choice.
//
// All twelve Metrics counters are atomic.Int64 and their Record* methods are
// bare Add(1) calls, so no other exported field or method bypasses the mutex.

// healthRouter builds a router able to serve HEALTHCHECK and RESETSTAT.
func healthRouter() *Router {
	r := NewRouter()
	RegisterMonitoringCommands(r) // HEALTH.CHECK
	RegisterConfigCommands(r)     // CONFIG RESETSTAT
	return r
}

// TestHealthCheckRacesResetOnStartTime is the primary case: the unlocked
// StartTime read racing the locked write in Metrics.Reset. Only meaningful
// under -race.
func TestHealthCheckRacesResetOnStartTime(t *testing.T) {
	s := store.NewStore()
	router := healthRouter()

	run := func(cmd string, args ...string) string {
		buf := &bytes.Buffer{}
		parsed := make([][]byte, len(args))
		for i, a := range args {
			parsed[i] = []byte(a)
		}
		if err := router.Execute(NewContext(cmd, parsed, s, resp.NewWriter(buf))); err != nil {
			t.Fatalf("%s: %v", cmd, err)
		}
		return buf.String()
	}

	done := make(chan struct{})
	// Writer: the locked StartTime write inside Metrics.Reset().
	go func() {
		defer close(done)
		for i := 0; i < 2000; i++ {
			run("CONFIG", "RESETSTAT")
		}
	}()
	// Reader: the bypassing unlocked read in cmdHEALTHCHECK.
	for i := 0; i < 2000; i++ {
		run("HEALTH.CHECK")
	}
	<-done
}

// TestHealthCheckUptimeControl is the control: HEALTHCHECK must report a
// plausible, non-negative uptime, and RESETSTAT must still succeed. This proves
// the race test above actually reaches the read rather than erroring early.
func TestHealthCheckUptimeControl(t *testing.T) {
	s := store.NewStore()
	router := healthRouter()

	run := func(cmd string, args ...string) string {
		buf := &bytes.Buffer{}
		parsed := make([][]byte, len(args))
		for i, a := range args {
			parsed[i] = []byte(a)
		}
		if err := router.Execute(NewContext(cmd, parsed, s, resp.NewWriter(buf))); err != nil {
			t.Fatalf("%s: %v", cmd, err)
		}
		return buf.String()
	}

	out := run("HEALTH.CHECK")
	v, err := resp.NewReader(bytes.NewReader([]byte(out))).ReadValue()
	if err != nil || v.Type != resp.TypeArray || len(v.Array) < 6 {
		t.Fatalf("HEALTHCHECK returned %q (err %v), want an array of >=6 fields", out, err)
	}
	if string(v.Array[0].Bulk) != "status" {
		t.Fatalf("HEALTHCHECK field 1 = %q, want \"status\"", v.Array[0].Bulk)
	}
	// uptime_seconds is the field carrying the value read from StartTime.
	uptime := v.Array[5].Int
	if uptime < 0 {
		t.Fatalf("CONTROL FAIL: HEALTHCHECK uptime_seconds = %d, want >= 0", uptime)
	}
	if out := run("CONFIG", "RESETSTAT"); out != "+OK\r\n" {
		t.Fatalf("CONFIG RESETSTAT = %q, want +OK", out)
	}
	t.Log("CONTROL PASS: HEALTHCHECK reports a sane uptime and RESETSTAT succeeds")
}
