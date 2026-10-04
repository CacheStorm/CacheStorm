package command

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// xrevRangeStore builds a store holding stream "s" with entries 1-0, 2-0, 3-0.
func xrevRangeStore(t *testing.T) *store.Store {
	t.Helper()
	s := store.NewStore()
	stream := store.NewStreamValue(0)
	for i, f := range []string{"a", "b", "c"} {
		id := string(rune('1'+i)) + "-0"
		if _, err := stream.Add(id, map[string][]byte{"f": []byte(f)}); err != nil {
			t.Fatalf("fixture Add(%s) failed: %v", id, err)
		}
	}
	s.Set("s", stream, store.SetOptions{})
	return s
}

// xrevRangeRun executes cmd through the stream router and returns the parsed reply.
func xrevRangeRun(t *testing.T, s *store.Store, cmd string, args ...[]byte) *resp.Value {
	t.Helper()
	router := NewRouter()
	RegisterStreamCommands(router)

	var buf bytes.Buffer
	ctx := NewContext(cmd, args, s, resp.NewWriter(&buf))
	if err := router.Execute(ctx); err != nil {
		t.Fatalf("%s: Execute returned error: %v", cmd, err)
	}

	reply, err := resp.NewReader(bytes.NewReader(buf.Bytes())).ReadValue()
	if err != nil {
		t.Fatalf("%s: could not parse RESP reply %q: %v", cmd, buf.String(), err)
	}
	return reply
}

// xrevRangeIDs extracts stream entry IDs from an XRANGE/XREVRANGE reply array.
func xrevRangeIDs(v *resp.Value) []string {
	out := make([]string, 0, len(v.Array))
	for _, e := range v.Array {
		out = append(out, string(e.Array[0].Bulk))
	}
	return out
}

func xrevRangeEqual(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// XREVRANGE must expand a bare-millisecond bound the way Redis does: "2" as a
// start bound means 2-0, "2" as an end bound means 2-<max seq>. It previously
// forwarded raw bounds to GetRange, where parseStreamIDPair rejected anything
// that was not "ms-seq", so the whole range came back empty.
func TestXREVRANGEMsOnlyStartBound(t *testing.T) {
	s := xrevRangeStore(t)

	got := xrevRangeIDs(xrevRangeRun(t, s, "XREVRANGE", []byte("s"), []byte("+"), []byte("2")))
	want := []string{"3-0", "2-0"}

	if !xrevRangeEqual(got, want) {
		t.Fatalf("XREVRANGE s + 2 (partial start ID): got %v, want %v", got, want)
	}
}

func TestXREVRANGEMsOnlyEndBound(t *testing.T) {
	s := xrevRangeStore(t)

	got := xrevRangeIDs(xrevRangeRun(t, s, "XREVRANGE", []byte("s"), []byte("2"), []byte("-")))
	want := []string{"2-0", "1-0"}

	if !xrevRangeEqual(got, want) {
		t.Fatalf("XREVRANGE s 2 - (partial end ID): got %v, want %v", got, want)
	}
}

// COUNT must still compose with a normalized partial bound.
func TestXREVRANGECountWithPartialID(t *testing.T) {
	s := xrevRangeStore(t)

	got := xrevRangeIDs(xrevRangeRun(t, s, "XREVRANGE", []byte("s"), []byte("+"), []byte("2"), []byte("COUNT"), []byte("2")))
	want := []string{"3-0", "2-0"}

	if !xrevRangeEqual(got, want) {
		t.Fatalf("XREVRANGE s + 2 COUNT 2: got %v, want %v", got, want)
	}
}

// A malformed bound is a client error and must be reported as one, not silently
// yield an empty range.
func TestXREVRANGEInvalidBoundIsError(t *testing.T) {
	s := xrevRangeStore(t)

	reply := xrevRangeRun(t, s, "XREVRANGE", []byte("s"), []byte("+"), []byte("notanid"))

	if reply.Type != resp.TypeError {
		t.Fatalf("XREVRANGE s + notanid: got reply type %q, want an error reply", reply.Type)
	}
}

// Control: XRANGE already normalized its bounds, so the identical input works.
func TestXRANGEMsOnlyStartBoundControl(t *testing.T) {
	s := xrevRangeStore(t)

	got := xrevRangeIDs(xrevRangeRun(t, s, "XRANGE", []byte("s"), []byte("2"), []byte("+")))
	want := []string{"2-0", "3-0"}

	if !xrevRangeEqual(got, want) {
		t.Fatalf("XRANGE s 2 + control: got %v, want %v", got, want)
	}
}

// Control: the "+"/"-" sentinels are handled inside GetRange and stay unaffected.
func TestXREVRANGESentinelBoundsControl(t *testing.T) {
	s := xrevRangeStore(t)

	got := xrevRangeIDs(xrevRangeRun(t, s, "XREVRANGE", []byte("s"), []byte("+"), []byte("-")))
	want := []string{"3-0", "2-0", "1-0"}

	if !xrevRangeEqual(got, want) {
		t.Fatalf("XREVRANGE s + - sentinel control: got %v, want %v", got, want)
	}
}
