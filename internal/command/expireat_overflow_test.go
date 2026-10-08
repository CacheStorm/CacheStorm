package command

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func expireAtRun(t *testing.T, s *store.Store, args ...string) *resp.Value {
	t.Helper()
	router := NewRouter()
	RegisterServerCommands(router)
	RegisterStringCommands(router)
	RegisterKeyCommands(router)

	var buf bytes.Buffer
	ctx := NewContext(args[0], expireAtBytes(args[1:]), s, resp.NewWriter(&buf))
	if err := router.Execute(ctx); err != nil {
		t.Fatalf("%s: Execute returned error: %v", args[0], err)
	}
	v, err := resp.NewReader(bytes.NewReader(buf.Bytes())).ReadValue()
	if err != nil {
		t.Fatalf("%s: could not parse RESP reply %q: %v", args[0], buf.String(), err)
	}
	return v
}

func expireAtBytes(ss []string) [][]byte {
	out := make([][]byte, len(ss))
	for i, s := range ss {
		out[i] = []byte(s)
	}
	return out
}

// An absolute expiry is stored as a Unix nanosecond timestamp. Converting a
// timestamp outside the int64 nanosecond range overflows into a PAST instant,
// which expired the key immediately while the command reported success —
// silent data loss. Out-of-range values must be rejected instead.
func TestExpireAtRejectsOutOfRangeTimestamp(t *testing.T) {
	seconds := []string{
		"9223372037",          // one past the bound
		"16725225600",         // year 2500
		"99999999999999",      // far future
		"-9223372037",         // one past the lower bound
		"9223372036854775807", // MaxInt64
	}
	for _, ts := range seconds {
		s := store.NewStore()
		expireAtRun(t, s, "SET", "k", "v")

		if v := expireAtRun(t, s, "EXPIREAT", "k", ts); v.Type != resp.TypeError {
			t.Fatalf("EXPIREAT k %s returned type %v, want an error (out of nanosecond range)", ts, v.Type)
		}
		if n := expireAtRun(t, s, "EXISTS", "k").Int; n != 1 {
			t.Fatalf("EXPIREAT k %s destroyed the key (EXISTS=%d) — a rejected expiry must never delete data", ts, n)
		}
	}
}

func TestPExpireAtRejectsOutOfRangeTimestamp(t *testing.T) {
	millis := []string{
		"9223372036855",       // one past the bound
		"90000000000000",      // overflows when multiplied by 1e6
		"9223372036854775807", // MaxInt64
		"-9223372036855",      // one past the lower bound
	}
	for _, ts := range millis {
		s := store.NewStore()
		expireAtRun(t, s, "SET", "k", "v")

		if v := expireAtRun(t, s, "PEXPIREAT", "k", ts); v.Type != resp.TypeError {
			t.Fatalf("PEXPIREAT k %s returned type %v, want an error (out of nanosecond range)", ts, v.Type)
		}
		if n := expireAtRun(t, s, "EXISTS", "k").Int; n != 1 {
			t.Fatalf("PEXPIREAT k %s destroyed the key (EXISTS=%d)", ts, n)
		}
	}
}

// Control: in-range far-future timestamps still work — this is the boundary
// the new guard must not over-reject.
func TestExpireAtInRangeFutureControls(t *testing.T) {
	s := store.NewStore()
	expireAtRun(t, s, "SET", "k", "v")

	if r := expireAtRun(t, s, "EXPIREAT", "k", "4102444800"); r.Type != resp.TypeInteger || r.Int != 1 {
		t.Fatalf("EXPIREAT year2100 returned %v/%d, want integer 1", r.Type, r.Int)
	}
	if n := expireAtRun(t, s, "EXISTS", "k").Int; n != 1 {
		t.Fatal("the key must survive an in-range future EXPIREAT")
	}
	if ttl := expireAtRun(t, s, "TTL", "k").Int; ttl <= 0 {
		t.Fatalf("TTL after year2100 EXPIREAT = %d, want a positive value", ttl)
	}

	s2 := store.NewStore()
	expireAtRun(t, s2, "SET", "k", "v")
	if r := expireAtRun(t, s2, "PEXPIREAT", "k", "4102444800000"); r.Type != resp.TypeInteger || r.Int != 1 {
		t.Fatalf("PEXPIREAT year2100 returned %v/%d, want integer 1", r.Type, r.Int)
	}
	if n := expireAtRun(t, s2, "EXISTS", "k").Int; n != 1 {
		t.Fatal("the key must survive an in-range future PEXPIREAT")
	}
}

// Control: a genuinely past timestamp still deletes the key — correct Redis
// behaviour that the new guard must not regress.
func TestExpireAtPastStillExpiresControl(t *testing.T) {
	s := store.NewStore()
	expireAtRun(t, s, "SET", "k", "v")

	if r := expireAtRun(t, s, "EXPIREAT", "k", "1000000000"); r.Type != resp.TypeInteger {
		t.Fatalf("EXPIREAT with a past timestamp returned %v, want an integer", r.Type)
	}
	if n := expireAtRun(t, s, "EXISTS", "k").Int; n != 0 {
		t.Fatal("a past EXPIREAT must delete the key")
	}
}

// Control: relative EXPIRE/PEXPIRE and non-numeric input are unchanged.
func TestRelativeExpireAndErrorControls(t *testing.T) {
	s := store.NewStore()
	expireAtRun(t, s, "SET", "a", "v")
	if r := expireAtRun(t, s, "EXPIRE", "a", "1000"); r.Type != resp.TypeInteger || r.Int != 1 {
		t.Fatalf("EXPIRE a 1000 returned %v/%d, want integer 1", r.Type, r.Int)
	}
	if n := expireAtRun(t, s, "EXISTS", "a").Int; n != 1 {
		t.Fatal("the key must survive a relative EXPIRE")
	}

	s2 := store.NewStore()
	expireAtRun(t, s2, "SET", "b", "v")
	if r := expireAtRun(t, s2, "PEXPIRE", "b", "100000"); r.Type != resp.TypeInteger || r.Int != 1 {
		t.Fatalf("PEXPIRE b 100000 returned %v/%d, want integer 1", r.Type, r.Int)
	}

	s3 := store.NewStore()
	expireAtRun(t, s3, "SET", "c", "v")
	if r := expireAtRun(t, s3, "EXPIREAT", "c", "notanumber"); r.Type != resp.TypeError {
		t.Fatalf("EXPIREAT with a non-numeric argument returned %v, want an error", r.Type)
	}
	if n := expireAtRun(t, s3, "EXISTS", "c").Int; n != 1 {
		t.Fatal("a rejected EXPIREAT must not delete the key")
	}
}

// Control: the bound constants themselves must not be off by a factor.
func TestExpireAtBoundConstantsControls(t *testing.T) {
	if got, want := int64(maxExpireAtSeconds), int64(9223372036); got != want {
		t.Fatalf("maxExpireAtSeconds = %d, want %d (MaxInt64 / time.Second)", got, want)
	}
	if got, want := int64(maxExpireAtMillis), int64(9223372036854); got != want {
		t.Fatalf("maxExpireAtMillis = %d, want %d (MaxInt64 / time.Millisecond)", got, want)
	}
	// The in-range maximum must not overflow when converted to nanoseconds.
	if int64(maxExpireAtMillis)*int64(1000000) < 0 {
		t.Fatalf("maxExpireAtMillis still overflows when multiplied into nanoseconds")
	}
}
