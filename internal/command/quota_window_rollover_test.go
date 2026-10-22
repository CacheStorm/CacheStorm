package command

import (
	"bytes"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// quotaExec runs a command through the router, the same path a client takes,
// and returns the raw RESP reply.
func quotaExec(t *testing.T, router *Router, s *store.Store, cmd string, args ...string) string {
	t.Helper()

	argBytes := make([][]byte, len(args))
	for i, a := range args {
		argBytes[i] = []byte(a)
	}

	buf := &bytes.Buffer{}
	ctx := NewContext(cmd, argBytes, s, resp.NewWriter(buf))
	if err := router.Execute(ctx); err != nil {
		t.Fatalf("%s returned an error: %v", cmd, err)
	}
	return buf.String()
}

// TestQuotaUseEnforcesLimitInEveryWindow pins the quota window contract:
// a fixed window grants at most `limit` uses per window, and when the window
// rolls over the limit starts counting again from zero.
//
// Before the fix the expired window was never rolled forward: QUOTA.USE reset
// the counter when the window had expired but left ResetAt in the past, so
// EVERY later use started a fresh counter and the limit was never enforced
// again.
func TestQuotaUseEnforcesLimitInEveryWindow(t *testing.T) {
	router := NewRouter()
	RegisterMVCCCommands(router)
	s := store.NewStore()

	key := "quota-window-rollover-proof"
	t.Cleanup(func() { quotaExec(t, router, s, "QUOTA.DELETE", key) })

	if got := quotaExec(t, router, s, "QUOTA.SET", key, "2", "1000"); got != "+OK\r\n" {
		t.Fatalf("QUOTA.SET reply = %q, want %q", got, "+OK\r\n")
	}

	// First window: two uses fit inside the limit, the third is refused.
	if got := quotaExec(t, router, s, "QUOTA.USE", key, "1"); got != ":1\r\n" {
		t.Fatalf("QUOTA.USE #1 reply = %q, want %q", got, ":1\r\n")
	}
	if got := quotaExec(t, router, s, "QUOTA.USE", key, "1"); got != ":1\r\n" {
		t.Fatalf("QUOTA.USE #2 reply = %q, want %q", got, ":1\r\n")
	}
	if got := quotaExec(t, router, s, "QUOTA.USE", key, "1"); got != ":0\r\n" {
		t.Fatalf("QUOTA.USE #3 (limit reached) reply = %q, want %q", got, ":0\r\n")
	}

	// Let the window expire.
	time.Sleep(1200 * time.Millisecond)

	// Second window: the allowance starts again, and the limit applies again.
	if got := quotaExec(t, router, s, "QUOTA.USE", key, "1"); got != ":1\r\n" {
		t.Fatalf("QUOTA.USE after window rollover reply = %q, want %q", got, ":1\r\n")
	}
	if got := quotaExec(t, router, s, "QUOTA.USE", key, "1"); got != ":1\r\n" {
		t.Fatalf("QUOTA.USE #2 after window rollover reply = %q, want %q", got, ":1\r\n")
	}
	if got := quotaExec(t, router, s, "QUOTA.USE", key, "1"); got != ":0\r\n" {
		t.Fatalf("QUOTA.USE #3 after window rollover reply = %q, want %q: the limit must be "+
			"enforced in the new window too, not only in the first one", got, ":0\r\n")
	}

	// Third window: rolling over again must not leave the quota unenforced.
	time.Sleep(1200 * time.Millisecond)

	if got := quotaExec(t, router, s, "QUOTA.USE", key, "1"); got != ":1\r\n" {
		t.Fatalf("QUOTA.USE after second window rollover reply = %q, want %q", got, ":1\r\n")
	}
	if got := quotaExec(t, router, s, "QUOTA.USE", key, "1"); got != ":1\r\n" {
		t.Fatalf("QUOTA.USE #2 after second window rollover reply = %q, want %q", got, ":1\r\n")
	}
	if got := quotaExec(t, router, s, "QUOTA.USE", key, "1"); got != ":0\r\n" {
		t.Fatalf("QUOTA.USE #3 after second window rollover reply = %q, want %q: the limit must be "+
			"enforced in every window", got, ":0\r\n")
	}
}

// TestQuotaLimitHoldsWhileWindowIsOpen is the control for the case above: a
// quota whose window has NOT expired keeps refusing uses beyond its limit.
func TestQuotaLimitHoldsWhileWindowIsOpen(t *testing.T) {
	router := NewRouter()
	RegisterMVCCCommands(router)
	s := store.NewStore()

	key := "quota-window-open-control"
	t.Cleanup(func() { quotaExec(t, router, s, "QUOTA.DELETE", key) })

	if got := quotaExec(t, router, s, "QUOTA.SET", key, "2", "60000"); got != "+OK\r\n" {
		t.Fatalf("QUOTA.SET reply = %q, want %q", got, "+OK\r\n")
	}
	if got := quotaExec(t, router, s, "QUOTA.USE", key, "1"); got != ":1\r\n" {
		t.Fatalf("QUOTA.USE #1 reply = %q, want %q", got, ":1\r\n")
	}
	if got := quotaExec(t, router, s, "QUOTA.USE", key, "1"); got != ":1\r\n" {
		t.Fatalf("QUOTA.USE #2 reply = %q, want %q", got, ":1\r\n")
	}
	if got := quotaExec(t, router, s, "QUOTA.USE", key, "1"); got != ":0\r\n" {
		t.Fatalf("QUOTA.USE #3 (limit reached) reply = %q, want %q", got, ":0\r\n")
	}
}
