package command

import (
	"net"
	"strconv"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/sentinel"
	"github.com/cachestorm/cachestorm/internal/store"
)

// sentinelPort picks a port that refuses connections: bind it, then release it.
// Monitoring it is safe because the dial is refused immediately, and the test
// never asserts on the resulting master STATE — only on the master being
// registered and listed.
func sentinelPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to reserve a port: %v", err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

// TestSentinelWiringEndToEnd proves SENTINEL commands work from a cold start:
// no InitSentinel call anywhere, the singleton is constructed on first use, a
// master can be registered, the monitor loops can be started, and MASTERS lists
// what was registered.
//
// Before the wiring, cmdSENTINEL returned "ERR sentinel not initialized"
// because nothing in production ever called InitSentinel, so this file is the
// regression guard for that: the very first assertion below fails without the
// EnsureSentinel call.
func TestSentinelWiringEndToEnd(t *testing.T) {
	// globalSentinel is a package-level singleton shared by the whole package's
	// test binary. Save and restore it so this test neither inherits nor leaks
	// state to the tests that run alongside it.
	previous := globalSentinel
	globalSentinel = nil
	defer func() { globalSentinel = previous }()

	s := store.NewStore()
	router := NewRouter()
	RegisterSentinelCommands(router)

	// ---- STEP 1: the singleton comes up on first use, with no InitSentinel.
	// This is the assertion that fails on the unwired code.
	if got := runCmd(t, s, router, "SENTINEL", "MASTERS"); strings.Contains(got, "not initialized") {
		t.Fatalf("SENTINEL MASTERS reported %q — the singleton was never constructed on "+
			"use, so every SENTINEL subcommand is dead at runtime", strings.TrimSpace(got))
	}
	if globalSentinel == nil {
		t.Fatal("SENTINEL MASTERS did not construct the singleton")
	}
	if GetSentinel() != globalSentinel {
		t.Error("GetSentinel and the package singleton disagree")
	}
	t.Log("STEP 1 ok: SENTINEL works from a cold start with no InitSentinel")

	// ---- STEP 2: register a master through the command surface.
	port := sentinelPort(t)
	portStr := strconv.Itoa(port)
	if got := runCmd(t, s, router, "SENTINEL", "MONITOR", "mymaster", "127.0.0.1", portStr, "2"); got != "+OK\r\n" {
		t.Fatalf("SENTINEL MONITOR returned %q, want +OK", strings.TrimSpace(got))
	}
	if _, _, err := GetSentinel().GetMasterAddr("mymaster"); err != nil {
		t.Errorf("master was not registered on the sentinel after MONITOR returned +OK: %v", err)
	}
	t.Log("STEP 2 ok: SENTINEL MONITOR registered mymaster")

	// ---- STEP 3: start the monitor/gossip loops.
	if err := StartSentinel(); err != nil {
		t.Fatalf("StartSentinel failed: %v", err)
	}
	// Stop joins both loops via wg.Wait, so no goroutine outlives the test.
	defer StopSentinel()
	if masters := GetSentinel().Masters(); len(masters) != 1 || masters[0].Name != "mymaster" {
		t.Fatalf("expected exactly [mymaster] after Start, got %+v", masters)
	}
	t.Log("STEP 3 ok: StartSentinel started the monitor loops")

	// ---- STEP 4: MASTERS lists the registered master over the command surface.
	reply := runCmd(t, s, router, "SENTINEL", "MASTERS")
	if !strings.Contains(reply, "mymaster") {
		t.Fatalf("SENTINEL MASTERS did not list mymaster; reply=%q", strings.TrimSpace(reply))
	}
	if !strings.Contains(reply, portStr) {
		t.Errorf("SENTINEL MASTERS did not report the monitored port %s; reply=%q",
			portStr, strings.TrimSpace(reply))
	}
	t.Log("STEP 4 ok: SENTINEL MASTERS lists mymaster and its port")
}

// TestEnsureSentinelIsIdempotentAndDoesNotClobber guards the two properties the
// wiring relies on: repeated calls reuse the singleton, and an explicit
// InitSentinel configuration is never replaced.
func TestEnsureSentinelIsIdempotentAndDoesNotClobber(t *testing.T) {
	previous := globalSentinel
	defer func() { globalSentinel = previous }()

	globalSentinel = nil
	first := EnsureSentinel()
	if first == nil {
		t.Fatal("EnsureSentinel returned nil on a cold start")
	}
	if second := EnsureSentinel(); second != first {
		t.Error("EnsureSentinel returned a different sentinel on its second call; it must reuse " +
			"the existing one")
	}

	// An explicit configuration must win over the lazy default.
	explicit := sentinel.New(sentinel.Config{ID: "explicit-sentinel"})
	globalSentinel = explicit
	if got := EnsureSentinel(); got != explicit {
		t.Error("EnsureSentinel replaced a sentinel installed by InitSentinel")
	}
}