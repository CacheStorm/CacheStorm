package command

import (
	"math"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestHIncrByFloatRejectsInputBeforeCreation(t *testing.T) {
	for _, increment := range []string{"NaN", "Inf", "-Inf", "+Inf"} {
		t.Run(increment, func(t *testing.T) {
			s := store.NewStore()
			for repeat := 0; repeat < 2; repeat++ {
				got := runHashAuditX(t, s, "HINCRBYFLOAT", "missing", "f", increment)
				t.Logf("EXPECTED: error and no key ACTUAL: %q", got)
				if !strings.HasPrefix(got, "-ERR increment would produce NaN or Infinity") {
					t.Fatal("non-finite increment not rejected")
				}
				if _, ok := s.Get("missing"); ok {
					t.Fatal("rejection created a hash")
				}
			}
			runHashAuditX(t, s, "HSET", "existing", "f", "2", "keep", "v")
			before, _ := s.Get("existing")
			before.ExpiresAt = math.MaxInt64
			for _, field := range []string{"f", "absent"} {
				if got := runHashAuditX(t, s, "HINCRBYFLOAT", "existing", field, increment); !strings.HasPrefix(got, "-ERR") {
					t.Fatal("existing-key rejected input")
				}
				after, _ := s.Get("existing")
				if before != after || after.ExpiresAt != math.MaxInt64 {
					t.Fatal("rejected increment changed metadata")
				}
				if got := runHashAuditX(t, s, "HMGET", "existing", "f", "keep", "absent"); got != "*3\r\n$1\r\n2\r\n$1\r\nv\r\n$-1\r\n" {
					t.Fatal("rejected increment changed fields")
				}
			}
		})
	}
	s := store.NewStore()
	if got := runHashAuditX(t, s, "HINCRBYFLOAT", "invalid", "f", "not-a-number"); !strings.HasPrefix(got, "-ERR") {
		t.Fatal("parse error control")
	}
	if _, ok := s.Get("invalid"); ok {
		t.Fatal("parse failure created key")
	}
	if got := runHashAuditX(t, s, "HINCRBYFLOAT", "valid", "f", "1.5"); got != "$3\r\n1.5\r\n" {
		t.Fatalf("valid new key: %q", got)
	}
	if got := runHashAuditX(t, s, "HINCRBYFLOAT", "valid", "f", "-0.5"); got != "$1\r\n1\r\n" {
		t.Fatalf("valid existing key: %q", got)
	}
	if got := runHashAuditX(t, s, "HGET", "valid", "f"); got != "$1\r\n1\r\n" {
		t.Fatal("valid result not stored")
	}
	runHashAuditX(t, s, "HSET", "overflow", "f", "1e308")
	if got := runHashAuditX(t, s, "HINCRBYFLOAT", "overflow", "f", "1e308"); !strings.Contains(got, "NaN or Infinity") {
		t.Fatal("finite result overflow no longer rejected")
	}
	if got := runHashAuditX(t, s, "HGET", "overflow", "f"); got != "$5\r\n1e308\r\n" {
		t.Fatal("overflow changed existing value")
	}
}
