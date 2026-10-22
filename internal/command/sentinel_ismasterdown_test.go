package command

import (
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/store"
)

// closedPort binds a loopback port, releases it, and returns it. Dialling the
// result is refused immediately, so isReachable reports false without a timeout.
func closedPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return port
}

// openPort keeps a listener alive so isReachable's dial succeeds and the master
// stays MasterStateOK.
func openPort(t *testing.T) (int, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	return ln.Addr().(*net.TCPAddr).Port, func() { ln.Close() }
}

// TestSentinelIsMasterDownReportsTheNamedMaster pins the contract of
// SENTINEL IS-MASTER-DOWN <name> <epoch>: the master named in the request must
// be the one inspected, and the reply must be 1 when that master is down.
//
// The handler read the name from ctx.ArgString(0) — the subcommand slot, which
// is always the literal "ISMASTERDOWN" — instead of ArgString(1). Every other
// handler in this file reads ArgString(1), and the handler's own ArgCount() < 3
// guard matches Redis's <name> <epoch> shape. The lookup therefore always missed,
// the not-found branch ran, and the command answered 0 (not down) for every
// master including ones that were genuinely down.
func TestSentinelIsMasterDownReportsTheNamedMaster(t *testing.T) {
	previous := globalSentinel
	globalSentinel = nil
	defer func() { globalSentinel = previous }()

	s := store.NewStore()
	router := NewRouter()
	RegisterSentinelCommands(router)

	downPort := closedPort(t)
	upPort, closeUp := openPort(t)
	defer closeUp()

	// Quorum 2: with only this sentinel alive, checkODownLocked yields
	// downCount+1 >= 2 -> false, so no failover starts and the masters keep the
	// states the monitor gives them.
	if got := runCmd(t, s, router, "SENTINEL", "MONITOR", "downmaster", "127.0.0.1", strconv.Itoa(downPort), "2"); got != "+OK\r\n" {
		t.Fatalf("SENTINEL MONITOR downmaster = %q, want +OK", strings.TrimSpace(got))
	}
	if got := runCmd(t, s, router, "SENTINEL", "MONITOR", "upmaster", "127.0.0.1", strconv.Itoa(upPort), "2"); got != "+OK\r\n" {
		t.Fatalf("SENTINEL MONITOR upmaster = %q, want +OK", strings.TrimSpace(got))
	}

	if err := StartSentinel(); err != nil {
		t.Fatalf("StartSentinel: %v", err)
	}
	defer StopSentinel()

	// The monitor ticks every second; wait (bounded) for downmaster to flip to
	// SDown and upmaster to settle on OK.
	deadline := time.Now().Add(10 * time.Second)
	var downSettled, upSettled bool
	for time.Now().Before(deadline) && !(downSettled && upSettled) {
		downSettled, upSettled = false, false
		for _, m := range GetSentinel().Masters() {
			flags := strings.Join(m.Flags, ",")
			if m.Name == "downmaster" && strings.Contains(flags, "s_down") {
				downSettled = true
			}
			if m.Name == "upmaster" && flags == "master" {
				upSettled = true
			}
		}
		if !downSettled || !upSettled {
			time.Sleep(100 * time.Millisecond)
		}
	}
	if !downSettled {
		t.Fatalf("setup failed: downmaster never reached s_down; flags were %s", masterFlags(t))
	}
	if !upSettled {
		t.Fatalf("setup failed: upmaster never reached the ok state; flags were %s", masterFlags(t))
	}

	// Control: the reachable master is up. This passes both before and after
	// the fix, so a broken harness cannot masquerade as the defect.
	if got := runCmd(t, s, router, "SENTINEL", "ISMASTERDOWN", "upmaster", "1"); !strings.HasPrefix(got, "*2\r\n:0\r\n") {
		t.Fatalf("control failed: ISMASTERDOWN for a reachable master = %q, want a leading :0",
			strings.TrimSpace(got))
	}

	// The contract: the master named in the request is down, so report 1.
	if got := runCmd(t, s, router, "SENTINEL", "ISMASTERDOWN", "downmaster", "1"); !strings.HasPrefix(got, "*2\r\n:1\r\n") {
		t.Fatalf("FAIL: SENTINEL IS-MASTER-DOWN downmaster returned %q — the handler reads "+
			"the name from ArgString(0), the subcommand slot, so the lookup always misses and "+
			"the command reports every master as up", strings.TrimSpace(got))
	}

	// Secondary branch: an unmonitored name is still reported as unknown/not down.
	if got := runCmd(t, s, router, "SENTINEL", "ISMASTERDOWN", "nosuchmaster", "1"); !strings.HasPrefix(got, "*2\r\n:0\r\n") {
		t.Fatalf("SENTINEL ISMASTERDOWN for an unknown master = %q, want a leading :0",
			strings.TrimSpace(got))
	}
}

func masterFlags(t *testing.T) string {
	t.Helper()
	var sb strings.Builder
	for _, m := range GetSentinel().Masters() {
		sb.WriteString(m.Name + "=[" + strings.Join(m.Flags, ",") + "] ")
	}
	return sb.String()
}
