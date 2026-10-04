package command

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func expiryOverflowRun(t *testing.T, s *store.Store, args ...string) *resp.Value {
	t.Helper()
	router := NewRouter()
	RegisterStringCommands(router)
	RegisterKeyCommands(router)

	var buf bytes.Buffer
	ctx := NewContext(args[0], expiryOverflowBytes(args[1:]), s, resp.NewWriter(&buf))
	if err := router.Execute(ctx); err != nil {
		t.Fatalf("%s: Execute returned error: %v", args[0], err)
	}
	v, err := resp.NewReader(bytes.NewReader(buf.Bytes())).ReadValue()
	if err != nil {
		t.Fatalf("%s: could not parse RESP reply %q: %v", args[0], buf.String(), err)
	}
	return v
}

func expiryOverflowBytes(ss []string) [][]byte {
	out := make([][]byte, len(ss))
	for i, s := range ss {
		out[i] = []byte(s)
	}
	return out
}

// mustRejectAndNotStore asserts an error reply AND that nothing was written.
// Store.Set reads a non-positive TTL as "no expiry", so an unrepresentable
// expiry silently produced a PERMANENT key behind a +OK reply.
func mustRejectAndNotStore(t *testing.T, args ...string) {
	t.Helper()
	s := store.NewStore()

	if v := expiryOverflowRun(t, s, args...); v.Type != resp.TypeError {
		t.Fatalf("%v returned type %v, want an error", args, v.Type)
	}
	if n := expiryOverflowRun(t, s, "EXISTS", "k").Int; n != 0 {
		t.Fatalf("%v stored a PERMANENT key (EXISTS=%d) instead of rejecting", args, n)
	}
}

// SET's EXAT/PXAT and SETEX/PSETEX multiply the supplied value into a
// time.Duration. An unrepresentable value wraps to a non-positive duration,
// which the store treats as "no expiry" — a permanent key behind +OK.
func TestSetAndSetexRejectOverflowingExpiry(t *testing.T) {
	cases := [][]string{
		{"SET", "k", "v", "EXAT", "1000000000"},   // a past absolute time
		{"SET", "k", "v", "PXAT", "90000000000000"}, // ms * 1e6 overflows
		{"SET", "k", "v", "PXAT", "1000000000"},    // a past absolute time
		{"SETEX", "k", "9223372036854775807", "v"},
		{"SETEX", "k", "9223372037", "v"}, // one second past the bound
		{"PSETEX", "k", "9223372036854775807", "v"},
		{"PSETEX", "k", "9223372036855", "v"}, // one millisecond past the bound
	}
	for _, args := range cases {
		mustRejectAndNotStore(t, args...)
	}
}

// Control: in-range future EXAT/PXAT still set a real TTL.
func TestSetInRangeExatControls(t *testing.T) {
	s := store.NewStore()
	if v := expiryOverflowRun(t, s, "SET", "k", "v", "EXAT", "4102444800"); v.Type != resp.TypeSimpleString {
		t.Fatalf("SET EXAT year2100 returned type %v, want +OK", v.Type)
	}
	if ttl := expiryOverflowRun(t, s, "TTL", "k").Int; ttl <= 0 {
		t.Fatalf("TTL after future EXAT = %d, want a positive value", ttl)
	}

	s2 := store.NewStore()
	if v := expiryOverflowRun(t, s2, "SET", "k", "v", "PXAT", "4102444800000"); v.Type != resp.TypeSimpleString {
		t.Fatalf("SET PXAT year2100 returned type %v, want +OK", v.Type)
	}
	if ttl := expiryOverflowRun(t, s2, "TTL", "k").Int; ttl <= 0 {
		t.Fatalf("TTL after future PXAT = %d, want a positive value", ttl)
	}
}

// Control: normal SETEX/PSETEX still set real TTLs and the value reads back.
func TestSetexValidControls(t *testing.T) {
	s := store.NewStore()
	if v := expiryOverflowRun(t, s, "SETEX", "k", "1000", "hello"); v.Type != resp.TypeSimpleString {
		t.Fatalf("SETEX k 1000 hello returned type %v, want +OK", v.Type)
	}
	if ttl := expiryOverflowRun(t, s, "TTL", "k").Int; ttl <= 0 || ttl > 1000 {
		t.Fatalf("TTL after SETEX 1000 = %d, want (0, 1000]", ttl)
	}
	// GET replies as a RESP bulk string.
	if got := expiryOverflowRun(t, s, "GET", "k").Bulk; string(got) != "hello" {
		t.Fatalf("GET after SETEX = %q, want %q", got, "hello")
	}

	s2 := store.NewStore()
	if v := expiryOverflowRun(t, s2, "PSETEX", "k", "100000", "v"); v.Type != resp.TypeSimpleString {
		t.Fatalf("PSETEX k 100000 v returned type %v, want +OK", v.Type)
	}
	if ttl := expiryOverflowRun(t, s2, "TTL", "k").Int; ttl <= 0 {
		t.Fatalf("TTL after PSETEX = %d, want a positive value", ttl)
	}
}

// Control: the non-positive guards already in place must survive, and a plain
// SET must still work.
func TestNonPositiveAndPlainSetControls(t *testing.T) {
	for _, args := range [][]string{
		{"SET", "k", "v", "EX", "0"},
		{"SET", "k", "v", "EX", "-1"},
		{"SET", "k", "v", "PX", "0"},
		{"SETEX", "k", "0", "v"},
		{"PSETEX", "k", "0", "v"},
	} {
		mustRejectAndNotStore(t, args...)
	}

	s := store.NewStore()
	if v := expiryOverflowRun(t, s, "SET", "k", "v"); v.Type != resp.TypeSimpleString {
		t.Fatalf("plain SET returned type %v, want +OK", v.Type)
	}
	if got := expiryOverflowRun(t, s, "GET", "k").Bulk; string(got) != "v" {
		t.Fatalf("GET after plain SET = %q, want %q", got, "v")
	}
	// Assert permanence via the store's raw sentinel. The TTL *command*
	// divides the store's bare -1 nanosecond sentinel by time.Second and so
	// reports 0 rather than Redis's -1; that is a pre-existing, separate
	// deviation (see the round report) and is not what this control is about.
	if d := s.TTL("k"); d >= 0 {
		t.Fatalf("store TTL after plain SET = %v, want a negative (permanent) sentinel", d)
	}
}
