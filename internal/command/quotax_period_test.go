package command

import (
	"bytes"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// quotaxExec runs a command through the router, the same path a client takes,
// and returns the raw RESP reply.
func quotaxExec(t *testing.T, router *Router, s *store.Store, cmd string, args ...string) string {
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

// TestQuotaXCheckEnforcesLimitWithoutPeriod pins the quota contract for a
// quota whose period carries no usable length: "0" and a non-numeric period
// both reach the handler as a zero-length window.
//
// A zero-length window is over the moment the clock moves on, so the
// rollover reset the counter on every check: the limit was never enforced
// and the quota allowed unlimited use.
func TestQuotaXCheckEnforcesLimitWithoutPeriod(t *testing.T) {
	for _, period := range []string{"0", "abc"} {
		t.Run("period="+period, func(t *testing.T) {
			router := NewRouter()
			RegisterMoreCommands(router)
			s := store.NewStore()

			key := "quotax-no-period-proof-" + period
			t.Cleanup(func() { quotaxExec(t, router, s, "QUOTAX.DELETE", key) })

			if got := quotaxExec(t, router, s, "QUOTAX.CREATE", key, "1", period); got != "+OK\r\n" {
				t.Fatalf("QUOTAX.CREATE reply = %q, want %q", got, "+OK\r\n")
			}

			// The first use fits inside the limit of 1.
			if got := quotaxExec(t, router, s, "QUOTAX.CHECK", key, "1"); got != "*2\r\n:1\r\n:0\r\n" {
				t.Fatalf("QUOTAX.CHECK #1 reply = %q, want %q", got, "*2\r\n:1\r\n:0\r\n")
			}

			// A zero-length window is over as soon as the clock moves on.
			time.Sleep(20 * time.Millisecond)

			// The second use must be refused: the limit is 1.
			if got := quotaxExec(t, router, s, "QUOTAX.CHECK", key, "1"); got != "*2\r\n:0\r\n:0\r\n" {
				t.Fatalf("QUOTAX.CHECK #2 reply = %q, want %q: the limit must be enforced even when "+
					"the quota has no usable period", got, "*2\r\n:0\r\n:0\r\n")
			}
		})
	}
}

// TestQuotaXLimitHoldsWhilePeriodIsOpen is the control for the case above: a
// quota whose window has NOT expired keeps refusing uses beyond its limit.
func TestQuotaXLimitHoldsWhilePeriodIsOpen(t *testing.T) {
	router := NewRouter()
	RegisterMoreCommands(router)
	s := store.NewStore()

	key := "quotax-period-open-control"
	t.Cleanup(func() { quotaxExec(t, router, s, "QUOTAX.DELETE", key) })

	if got := quotaxExec(t, router, s, "QUOTAX.CREATE", key, "1", "60000"); got != "+OK\r\n" {
		t.Fatalf("QUOTAX.CREATE reply = %q, want %q", got, "+OK\r\n")
	}
	if got := quotaxExec(t, router, s, "QUOTAX.CHECK", key, "1"); got != "*2\r\n:1\r\n:0\r\n" {
		t.Fatalf("QUOTAX.CHECK #1 reply = %q, want %q", got, "*2\r\n:1\r\n:0\r\n")
	}
	if got := quotaxExec(t, router, s, "QUOTAX.CHECK", key, "1"); got != "*2\r\n:0\r\n:0\r\n" {
		t.Fatalf("QUOTAX.CHECK #2 (limit reached) reply = %q, want %q", got, "*2\r\n:0\r\n:0\r\n")
	}
}
