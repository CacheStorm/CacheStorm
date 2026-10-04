package command

import (
	"bytes"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// setExpireRun executes a command through the real router and returns the
// parsed RESP reply.
func setExpireRun(t *testing.T, s *store.Store, args ...[]byte) *resp.Value {
	t.Helper()
	router := NewRouter()
	RegisterStringCommands(router)
	RegisterKeyCommands(router)

	var buf bytes.Buffer
	ctx := NewContext(string(args[0]), args[1:], s, resp.NewWriter(&buf))
	if err := router.Execute(ctx); err != nil {
		t.Fatalf("%s: Execute returned error: %v", args[0], err)
	}

	reply, err := resp.NewReader(bytes.NewReader(buf.Bytes())).ReadValue()
	if err != nil {
		t.Fatalf("%s: could not parse RESP reply %q: %v", args[0], buf.String(), err)
	}
	return reply
}

func setExpireBytes(ss ...string) [][]byte {
	out := make([][]byte, len(ss))
	for i, s := range ss {
		out[i] = []byte(s)
	}
	return out
}

func setExpireExists(t *testing.T, s *store.Store, key string) int64 {
	t.Helper()
	return setExpireRun(t, s, setExpireBytes("EXISTS", key)...).Int
}

// A non-positive EX/PX is a client error and must be reported as one. The
// handlers never validated it, and Store.Set only applies a TTL when
// opts.TTL > 0 — so "EX 0" silently became a permanent key.
func TestSETRejectsNonPositiveExpiry(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"EX 0", []string{"SET", "k", "v", "EX", "0"}},
		{"EX -1", []string{"SET", "k", "v", "EX", "-1"}},
		{"PX 0", []string{"SET", "k", "v", "PX", "0"}},
		{"PX -1", []string{"SET", "k", "v", "PX", "-1"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := store.NewStore()

			reply := setExpireRun(t, s, setExpireBytes(tc.args...)...)
			if reply.Type != resp.TypeError {
				t.Fatalf("SET k v %s: got reply type %v, want an error", tc.args[3], reply.Type)
			}
			if n := setExpireExists(t, s, "k"); n != 0 {
				t.Fatalf("SET k v %s stored the key anyway (EXISTS=%d)", tc.args[3], n)
			}
		})
	}
}

// A rejected expiry must not disturb an existing key.
func TestSETRejectsNonPositiveExpiryLeavesExistingKey(t *testing.T) {
	s := store.NewStore()

	if reply := setExpireRun(t, s, setExpireBytes("SET", "k", "original")...); reply.Str != "OK" {
		t.Fatalf("setup SET k original: got %q, want +OK", reply.Str)
	}

	reply := setExpireRun(t, s, setExpireBytes("SET", "k", "new", "EX", "0")...)
	if reply.Type != resp.TypeError {
		t.Fatalf("SET k new EX 0: got reply type %v, want an error", reply.Type)
	}

	got, found := s.Get("k")
	if !found {
		t.Fatal("key was lost after the rejected SET")
	}
	sv, ok := got.Value.(*store.StringValue)
	if !ok {
		t.Fatalf("key type changed to %T", got.Value)
	}
	if string(sv.Data) != "original" {
		t.Fatalf("value = %q, want %q (the rejected SET must not apply)", sv.Data, "original")
	}
}

// Control: a positive EX still stores the key and sets a live TTL.
func TestSETPositiveExpiryControl(t *testing.T) {
	s := store.NewStore()

	reply := setExpireRun(t, s, setExpireBytes("SET", "k", "v", "EX", "100")...)
	if reply.Type != resp.TypeSimpleString || reply.Str != "OK" {
		t.Fatalf("SET k v EX 100: got type %v str %q, want +OK", reply.Type, reply.Str)
	}
	if n := setExpireExists(t, s, "k"); n != 1 {
		t.Fatalf("SET k v EX 100 did not store the key (EXISTS=%d)", n)
	}
	if d := s.TTL("k"); d <= 0 || d > 100*time.Second {
		t.Fatalf("SET k v EX 100 produced TTL %v, want (0, 100s]", d)
	}
}

// Control: plain SET stays permanent, and the sibling handlers keep their
// existing behaviour on the same options.
func TestSETExpiryControls(t *testing.T) {
	s := store.NewStore()

	if reply := setExpireRun(t, s, setExpireBytes("SET", "k", "v")...); reply.Str != "OK" {
		t.Fatalf("SET k v: got %q, want +OK", reply.Str)
	}
	if d := s.TTL("k"); d >= 0 {
		t.Fatalf("SET k v produced TTL %v, want a negative (permanent) sentinel", d)
	}

	if reply := setExpireRun(t, s, setExpireBytes("SET", "k_nx", "v", "NX")...); reply.Type != resp.TypeSimpleString {
		t.Fatalf("SET k_nx v NX: got type %v, want +OK", reply.Type)
	}
	if reply := setExpireRun(t, s, setExpireBytes("SET", "k_nx", "v", "XX")...); reply.Type != resp.TypeSimpleString {
		t.Fatalf("SET k_nx v XX: got type %v, want +OK", reply.Type)
	}
	if reply := setExpireRun(t, s, setExpireBytes("SET", "k2", "v", "PX", "100")...); reply.Type != resp.TypeSimpleString {
		t.Fatalf("SET k2 v PX 100: got type %v, want +OK", reply.Type)
	}
	if reply := setExpireRun(t, s, setExpireBytes("SET", "k3", "v", "KEEPTTL")...); reply.Type != resp.TypeSimpleString {
		t.Fatalf("SET k3 v KEEPTTL: got type %v, want +OK", reply.Type)
	}
}
