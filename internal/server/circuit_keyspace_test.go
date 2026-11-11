package server

import (
	"testing"
	"time"
)

// A CIRCUIT is a hash key: an open breaker stays open across a restart and
// moves to half-open once its timeout has passed.
func TestCircuitBreakersAreKeys(t *testing.T) {
	cfg := aofTxConfig(t)
	s := startToggleServer(t, cfg)
	c := dialRepl(t, s)
	state := func(want string) {
		t.Helper()
		if got := c.do("CIRCUIT.STATE", "payments"); string(got.Bulk) != want {
			t.Fatalf("CIRCUIT.STATE: %+v, want %s", got, want)
		}
	}
	allow := func(want int64) {
		t.Helper()
		if got := c.do("CIRCUIT.ALLOW", "payments"); got.Int != want {
			t.Fatalf("CIRCUIT.ALLOW: %+v, want %d", got, want)
		}
	}
	if got := c.do("CIRCUIT.CREATE", "payments", "2", "1", "300"); got.Type == '-' {
		t.Fatal(got.Err)
	}
	c.do("CIRCUIT.FAILURE", "payments")
	state("closed")
	c.do("CIRCUIT.FAILURE", "payments")
	state("open")
	allow(0)
	c.do("SET", "plain", "v")
	if got := c.do("CIRCUIT.STATE", "plain"); got.Type != '-' {
		t.Fatalf("CIRCUIT.STATE on a string: %v", got)
	}
	stopReplServer(s)

	s = startToggleServer(t, cfg)
	defer stopReplServer(s)
	c = dialRepl(t, s)
	state("open")
	allow(0)
	time.Sleep(350 * time.Millisecond)
	allow(1)
	state("half-open")
	c.do("CIRCUIT.SUCCESS", "payments")
	state("closed")
	if got := c.do("CIRCUIT.LIST"); len(got.Array) != 2 || string(got.Array[0].Bulk) != "payments" {
		t.Fatalf("CIRCUIT.LIST: %+v", got)
	}
	if got := c.do("CIRCUIT.STATS", "payments"); len(got.Array) != 20 || got.Array[5].Int != 0 {
		t.Fatalf("CIRCUIT.STATS: %+v", got)
	}
	if got := c.do("CIRCUIT.DELETE", "payments"); got.Int != 1 {
		t.Fatalf("CIRCUIT.DELETE: %v", got)
	}
}
