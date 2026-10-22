package command

import (
	"bytes"
	"strconv"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Proof for the HGETEX EX/PX TTL overflow defect.
//
// internal/command/hash_commands.go (before the fix)
//	  case "EX":
//	      sec, err := strconv.ParseInt(ctx.ArgString(i), 10, 64)
//	      ...
//	      ctx.Store.SetTTL(key, time.Duration(sec)*time.Second)   // UNBOUNDED
//	  case "PX":
//	      ms, err := strconv.ParseInt(ctx.ArgString(i), 10, 64)
//	      ...
//	      ctx.Store.SetTTL(key, time.Duration(ms)*time.Millisecond) // UNBOUNDED
//
// sec*1e9 and ms*1e6 overflow int64 nanoseconds and wrap NEGATIVE.
// Store.SetTTL -> nowNSPlus then computes now + <negative>, i.e. an expiry
// instant in the PAST, so the hash is born expired and the very next read
// reaps it. The HGETEX reply itself reports success, so the client is told
// the hash was read fine while the data is destroyed underneath it.
//
// The SAME handler already bounds the absolute-instant options:
//	  case "EXAT": if ts > maxExpireAtSeconds || ts < -maxExpireAtSeconds {
//	                    return ctx.WriteError(errInvalidExpireTime) }
//	  case "PXAT": if ms > maxExpireAtMillis || ms < -maxExpireAtMillis { ... }
// and the sibling cmdGETEX (string_commands.go:721,:734) bounds the very same
// relative argument:
//	  case "EX": if sec <= 0 || sec > maxExpireAtSeconds { ... }
//	  case "PX": if ms <= 0 || ms > maxExpireAtMillis { ... }
//
// So the established contract for a relative EX/PX expiry in this codebase is:
// reject a non-positive or out-of-range value with "ERR invalid expire time".
// HGETEX's EX/PX cases never applied that guard.

// hgetexProofRun runs one command through the real router and returns the raw
// RESP reply.
func hgetexTTLRun(t *testing.T, s *store.Store, router *Router, cmd string, args ...string) string {
	t.Helper()
	parsed := make([][]byte, len(args))
	for i, a := range args {
		parsed[i] = []byte(a)
	}
	buf := &bytes.Buffer{}
	if err := router.Execute(NewContext(cmd, parsed, s, resp.NewWriter(buf))); err != nil {
		t.Fatalf("%s %v: Execute returned error: %v", cmd, args, err)
	}
	return buf.String()
}

// bulkReply renders the RESP wire form of a bulk string, so the assertions
// compare against what the client actually receives.
func hgetexTTLBulk(s string) string {
	return "$" + strconv.Itoa(len(s)) + "\r\n" + s + "\r\n"
}

func hgetexTTLRouter() *Router {
	router := NewRouter()
	RegisterHashCommands(router)
	RegisterKeyCommands(router)
	return router
}

// TestHGetExRejectsOutOfRangeRelativeTTL is the primary case: a
// non-positive or overflowing relative EX/PX must be refused with an error,
// and the hash must survive. Before the fix HGETEX accepted the value, replied
// success, and left the hash with an expiry in the past.
func TestHGetExRejectsOutOfRangeRelativeTTL(t *testing.T) {
	router := hgetexTTLRouter()

	cases := []struct {
		opt, arg, name string
	}{
		{"EX", "9223372036854775807", "HGETEX EX MaxInt64"},
		{"EX", "-9223372036854775808", "HGETEX EX MinInt64"},
		{"EX", "9223372037", "HGETEX EX just over the int64-nanosecond limit"},
		{"EX", "0", "HGETEX EX 0"},
		{"EX", "-1", "HGETEX EX -1"},
		{"PX", "9223372036854775807", "HGETEX PX MaxInt64"},
		{"PX", "-9223372036854775808", "HGETEX PX MinInt64"},
		{"PX", "9223372036854775808", "HGETEX PX just over the int64-nanosecond limit"},
		{"PX", "0", "HGETEX PX 0"},
		{"PX", "-1", "HGETEX PX -1"},
	}

	for _, c := range cases {
		s := store.NewStore()
		hgetexTTLRun(t, s, router, "HSET", "h", "f", "v")

		out := hgetexTTLRun(t, s, router, "HGETEX", "h", "f", c.opt, c.arg)
		if !strings.HasPrefix(out, "-") {
			// It accepted the value. Now check whether the hash survived.
			ttl := hgetexTTLRun(t, s, router, "TTL", "h")
			got := hgetexTTLRun(t, s, router, "HGET", "h", "f")
			t.Fatalf("FAIL: %s: HGETEX replied %q instead of an error; TTL is now %q and "+
				"HGET returns %q -- the unbounded %s argument wrapped the expiry into the past, "+
				"so the hash is born expired and its data is destroyed behind a success reply. "+
				"Redis answers \"ERR invalid expire time in 'hgetex'\"",
				c.name, out, ttl, got, c.opt)
		}

		// The hash must still be there after the refusal.
		got := hgetexTTLRun(t, s, router, "HGET", "h", "f")
		if got != hgetexTTLBulk("v") {
			t.Fatalf("FAIL: %s: the hash was destroyed by a REFUSED %s argument: HGET returns %q, want %q",
				c.name, c.opt, got, hgetexTTLBulk("v"))
		}
	}
}

// TestHGetExRelativeTTLControls is the control: a normal relative EX/PX
// must still be accepted, must still apply the expiry, and the absolute
// options (already guarded before this round) must keep behaving.
func TestHGetExRelativeTTLControls(t *testing.T) {
	router := hgetexTTLRouter()

	for _, c := range []struct {
		opt, arg, name string
	}{
		{"EX", "100", "HGETEX EX 100"},
		{"PX", "100000", "HGETEX PX 100000"},
		{"EX", "9223372036", "HGETEX EX at the int64-nanosecond limit"},
	} {
		s := store.NewStore()
		hgetexTTLRun(t, s, router, "HSET", "h", "f", "v")

		out := hgetexTTLRun(t, s, router, "HGETEX", "h", "f", c.opt, c.arg)
		if out != hgetexTTLBulk("v") {
			t.Fatalf("CONTROL FAIL: %s: HGETEX replied %q, want the field value %q", c.name, out, hgetexTTLBulk("v"))
		}
		ttl := hgetexTTLRun(t, s, router, "TTL", "h")
		if !strings.HasPrefix(ttl, ":") {
			t.Fatalf("CONTROL FAIL: %s: TTL replied %q, want an integer expiry", c.name, ttl)
		}
		if strings.HasPrefix(ttl, ":-") {
			t.Fatalf("CONTROL FAIL: %s: TTL replied %q, want a POSITIVE expiry -- a valid relative "+
				"EX/PX must still set a TTL", c.name, ttl)
		}
		if got := hgetexTTLRun(t, s, router, "HGET", "h", "f"); got != hgetexTTLBulk("v") {
			t.Fatalf("CONTROL FAIL: %s: HGET returns %q, want %q -- a valid relative EX/PX must not "+
				"destroy the hash", c.name, got, hgetexTTLBulk("v"))
		}
	}

	// The absolute-instant options were already guarded before this round and
	// must keep rejecting out-of-range values.
	s := store.NewStore()
	hgetexTTLRun(t, s, router, "HSET", "h", "f", "v")
	for _, c := range []struct{ opt, arg, name string }{
		{"EXAT", "99999999999999", "HGETEX EXAT out of range"},
		{"PXAT", "99999999999999999", "HGETEX PXAT out of range"},
	} {
		out := hgetexTTLRun(t, s, router, "HGETEX", "h", "f", c.opt, c.arg)
		if !strings.HasPrefix(out, "-") {
			t.Fatalf("CONTROL FAIL: %s: HGETEX replied %q, want an error", c.name, out)
		}
	}
}
