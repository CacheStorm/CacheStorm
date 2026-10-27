//go:build compat

package compat

import (
	"bytes"
	"fmt"
	"strconv"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestHLLReferenceInteroperability(t *testing.T) {
	redis := &Target{Name: "redis", Addr: startRedis(t), conns: map[int]*Conn{}}
	cs := &Target{Name: "cachestorm", Addr: startCacheStorm(t), conns: map[int]*Conn{}}
	defer redis.resetConns()
	defer cs.resetConns()
	must := func(t *testing.T, target *Target, args ...string) *Val {
		t.Helper()
		r := target.adminDo(args...)
		if r.Err != "" || r.Val == nil || r.Val.Kind == '-' {
			t.Fatalf("%s %s: %s", target.Name, args[0], r.Display())
		}
		return r.Val
	}
	for _, n := range []int{0, 1, 100, 2500, 20000} {
		t.Run(strconv.Itoa(n), func(t *testing.T) {
			args := []string{"PFADD", "h"}
			for i := 0; i < n; i++ {
				args = append(args, fmt.Sprintf("item:%d\x00", i))
			}
			for _, target := range []*Target{redis, cs} {
				must(t, target, "FLUSHALL")
				must(t, target, args...)
			}
			expected, actual := must(t, redis, "PFCOUNT", "h"), must(t, cs, "PFCOUNT", "h")
			if !bytes.Equal(expected.Encode(), actual.Encode()) {
				t.Fatalf("n=%d counts: %s / %s", n, expected.Str, actual.Str)
			}
			referenceBytes, ownBytes := must(t, redis, "GET", "h").Str, must(t, cs, "GET", "h").Str
			reference, err := store.DecodeHLL([]byte(referenceBytes))
			if err != nil {
				t.Fatal(err)
			}
			own, err := store.DecodeHLL([]byte(ownBytes))
			if err != nil {
				t.Fatal(err)
			}
			if reference.Registers != own.Registers {
				t.Fatalf("n=%d registers differ from Redis", n)
			}
			must(t, cs, "SET", "imported", referenceBytes)
			must(t, redis, "SET", "imported", ownBytes)
			if got := must(t, cs, "PFCOUNT", "imported"); got.Str != expected.Str {
				t.Fatal("reference import count differs")
			}
			if got := must(t, redis, "PFCOUNT", "imported"); got.Str != expected.Str {
				t.Fatal("Redis rejected own payload count")
			}
			for _, target := range []*Target{redis, cs} {
				must(t, target, "PFADD", "imported", "new-element")
				must(t, target, "PFMERGE", "merged", "h", "imported")
			}
			expected, actual = must(t, redis, "PFCOUNT", "merged"), must(t, cs, "PFCOUNT", "merged")
			if expected.Str != actual.Str {
				t.Fatalf("merged=%s / %s", expected.Str, actual.Str)
			}
			for _, direction := range []struct{ src, dst *Target }{{redis, cs}, {cs, redis}} {
				payload := must(t, direction.src, "DUMP", "merged").Str
				must(t, direction.dst, "RESTORE", "restored", "0", payload, "REPLACE")
				if got := must(t, direction.dst, "PFCOUNT", "restored"); got.Str != expected.Str {
					t.Fatal("DUMP/RESTORE lost HLL registers")
				}
			}
		})
	}
}
