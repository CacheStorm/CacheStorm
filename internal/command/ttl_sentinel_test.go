package command

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func ttlSentinelRun(t *testing.T, s *store.Store, args ...string) *resp.Value {
	t.Helper()
	router := NewRouter()
	RegisterServerCommands(router)
	RegisterStringCommands(router)
	RegisterKeyCommands(router)

	var buf bytes.Buffer
	ctx := NewContext(args[0], ttlSentinelBytes(args[1:]), s, resp.NewWriter(&buf))
	if err := router.Execute(ctx); err != nil {
		t.Fatalf("%s: Execute returned error: %v", args[0], err)
	}
	v, err := resp.NewReader(bytes.NewReader(buf.Bytes())).ReadValue()
	if err != nil {
		t.Fatalf("%s: could not parse RESP reply %q: %v", args[0], buf.String(), err)
	}
	return v
}

func ttlSentinelBytes(ss []string) [][]byte {
	out := make([][]byte, len(ss))
	for i, s := range ss {
		out[i] = []byte(s)
	}
	return out
}

// Store.TTL returns bare -1 (no expiry) and -2 (no key) as NANOSECOND
// sentinels. cmdTTL divided them by time.Second, collapsing both to 0, so a
// client could not tell a permanent key from a missing one. Redis returns -1
// and -2 respectively.
func TestTTLDistinguishesPermanentFromMissing(t *testing.T) {
	s := store.NewStore()
	ttlSentinelRun(t, s, "SET", "perm", "v")

	if got := ttlSentinelRun(t, s, "TTL", "perm").Int; got != -1 {
		t.Fatalf("TTL on a permanent key = %d, want -1", got)
	}
	if got := ttlSentinelRun(t, s, "TTL", "nosuchkey").Int; got != -2 {
		t.Fatalf("TTL on a missing key = %d, want -2", got)
	}
}

// PTTL carries the same defect through the millisecond divisor.
func TestPTTLDistinguishesPermanentFromMissing(t *testing.T) {
	s := store.NewStore()
	ttlSentinelRun(t, s, "SET", "perm", "v")

	if got := ttlSentinelRun(t, s, "PTTL", "perm").Int; got != -1 {
		t.Fatalf("PTTL on a permanent key = %d, want -1", got)
	}
	if got := ttlSentinelRun(t, s, "PTTL", "nosuchkey").Int; got != -2 {
		t.Fatalf("PTTL on a missing key = %d, want -2", got)
	}
}

// Control: real TTLs are still reported in whole seconds / milliseconds.
func TestTTLRealValueControls(t *testing.T) {
	s := store.NewStore()
	ttlSentinelRun(t, s, "SETEX", "k", "100", "v")

	if ttl := ttlSentinelRun(t, s, "TTL", "k").Int; ttl <= 0 || ttl > 100 {
		t.Fatalf("TTL after SETEX 100 = %d, want (0, 100]", ttl)
	}

	s2 := store.NewStore()
	ttlSentinelRun(t, s2, "PSETEX", "k", "100000", "v")
	if ttl := ttlSentinelRun(t, s2, "PTTL", "k").Int; ttl <= 0 || ttl > 100000 {
		t.Fatalf("PTTL after PSETEX 100000 = %d, want (0, 100000]", ttl)
	}
}

// Control: a key whose expiry has already passed reports -2, not a negative
// duration scaled into a spurious value.
func TestTTLExpiredKeyControl(t *testing.T) {
	s := store.NewStore()
	ttlSentinelRun(t, s, "SET", "k", "v")
	ttlSentinelRun(t, s, "PEXPIREAT", "k", "1000000000000") // 2001

	if got := ttlSentinelRun(t, s, "TTL", "k").Int; got != -2 {
		t.Fatalf("TTL on an already-expired key = %d, want -2", got)
	}
}

// Control: the store-level contract is unchanged — the fix belongs in the
// command layer, and Store.TTL keeps returning its raw sentinels.
func TestStoreTTLSentinelsUnchangedControl(t *testing.T) {
	s := store.NewStore()
	s.Set("perm", &store.StringValue{Data: []byte("v")}, store.SetOptions{})

	if d := s.TTL("perm"); d != -1 {
		t.Fatalf("Store.TTL(permanent) = %v, want the bare -1 sentinel", d)
	}
	if d := s.TTL("nosuchkey"); d != -2 {
		t.Fatalf("Store.TTL(missing) = %v, want the bare -2 sentinel", d)
	}
}
