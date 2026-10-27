package command

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func bitfieldCaseRun(t *testing.T, s *store.Store, router *Router, cmd string, args ...string) string {
	t.Helper()
	parsed := make([][]byte, len(args))
	for i, a := range args {
		parsed[i] = []byte(a)
	}
	buf := &bytes.Buffer{}
	if err := router.Execute(NewContext(cmd, parsed, s, resp.NewWriter(buf))); err != nil {
		t.Fatalf("%s: Execute returned error: %v", cmd, err)
	}
	return buf.String()
}

// bitfieldCaseGet runs BITFIELD <key> GET <enc> <off> and returns the integer.
func bitfieldCaseGet(t *testing.T, s *store.Store, router *Router, key, enc string, off int) int64 {
	t.Helper()
	out := bitfieldCaseRun(t, s, router, "BITFIELD", key, "GET", enc, fmtInt(off))
	v, err := resp.NewReader(bytes.NewReader([]byte(out))).ReadValue()
	if err != nil {
		t.Fatalf("unparsable BITFIELD GET %s reply %q: %v", enc, out, err)
	}
	if v.Type != resp.TypeArray || len(v.Array) != 1 || v.Array[0].Type != resp.TypeInteger {
		t.Fatalf("BITFIELD GET %s returned %q, want a 1-element integer array", enc, out)
	}
	return v.Array[0].Int
}

func fmtInt(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}

func bitfieldCaseRouter() *Router {
	router := NewRouter()
	RegisterBitmapCommands(router)
	return router
}

func TestBitfieldEncodingRejectsUppercase(t *testing.T) {
	s := store.NewStore()
	r := bitfieldCaseRouter()
	for _, encoding := range []string{"I8", "U8", "u64", "i65", "u08"} {
		if out := bitfieldCaseRun(t, s, r, "BITFIELD", "k", "SET", encoding, "0", "1"); !strings.HasPrefix(out, "-ERR Invalid bitfield type.") {
			t.Fatalf("encoding %s accepted: %q", encoding, out)
		}
	}
	if s.Exists("k") {
		t.Fatal("invalid encoding created key")
	}
}

func TestBitfieldSignedSaturationAndWrap(t *testing.T) {
	s := store.NewStore()
	r := bitfieldCaseRouter()
	bitfieldCaseRun(t, s, r, "BITFIELD", "k", "SET", "i8", "0", "100")
	if out := bitfieldCaseRun(t, s, r, "BITFIELD", "k", "OVERFLOW", "SAT", "INCRBY", "i8", "0", "-1000"); out != "*1\r\n:-128\r\n" {
		t.Fatalf("signed saturation: %q", out)
	}
	bitfieldCaseRun(t, s, r, "BITFIELD", "k", "SET", "i8", "0", "127")
	bitfieldCaseRun(t, s, r, "BITFIELD", "k", "INCRBY", "i8", "0", "1")
	if got := bitfieldCaseGet(t, s, r, "k", "i8", 0); got != -128 {
		t.Fatalf("wrap=%d", got)
	}
}

func TestBitfieldUnsignedWrapAndReadControls(t *testing.T) {
	s := store.NewStore()
	r := bitfieldCaseRouter()
	bitfieldCaseRun(t, s, r, "BITFIELD", "k", "SET", "u8", "0", "-1")
	if got := bitfieldCaseGet(t, s, r, "k", "u8", 0); got != 255 {
		t.Fatalf("unsigned wrap=%d", got)
	}
	if got := bitfieldCaseGet(t, s, r, "missing", "i64", 0); got != 0 {
		t.Fatalf("missing=%d", got)
	}
	if s.Exists("missing") {
		t.Fatal("read created key")
	}
}
