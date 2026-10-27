package command

import (
	"strconv"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func TestExpiryMillisecondsFullRange(t *testing.T) {
	for _, at := range []int64{4102444800000, 4102444800499, 4102444800500, 4102444800999, 9223372036855, 16725225600000, 9223372036854775807} {
		for _, cmd := range []string{"PEXPIREAT", "SET", "GETEX"} {
			t.Run(cmd+"/"+strconv.FormatInt(at, 10), func(t *testing.T) {
				s := store.NewStore()
				expireAtRun(t, s, "SET", "k", "v")
				var result *resp.Value
				switch cmd {
				case "SET":
					result = expireAtRun(t, s, cmd, "k", "v", "PXAT", strconv.FormatInt(at, 10))
				case "GETEX":
					result = expireAtRun(t, s, cmd, "k", "PXAT", strconv.FormatInt(at, 10))
				default:
					result = expireAtRun(t, s, cmd, "k", strconv.FormatInt(at, 10))
				}
				if result.Type == resp.TypeError {
					t.Fatal(result.Err)
				}
				if got := expireAtRun(t, s, "PEXPIRETIME", "k").Int; got != at {
					t.Fatalf("PEXPIRETIME: got %d, want %d", got, at)
				}
				wantSeconds := at / 1000
				if at%1000 >= 500 {
					wantSeconds++
				}
				if got := expireAtRun(t, s, "EXPIRETIME", "k").Int; got != wantSeconds {
					t.Fatalf("EXPIRETIME: got %d, want %d", got, wantSeconds)
				}
				before := time.Now().UnixMilli()
				got := expireAtRun(t, s, "PTTL", "k").Int
				after := time.Now().UnixMilli()
				if got < at-after-1 || got > at-before {
					t.Fatalf("PTTL: got %d, want [%d,%d]", got, at-after, at-before)
				}
				wantTTL := (got + 500) / 1000
				if ttl := expireAtRun(t, s, "TTL", "k").Int; ttl < wantTTL-1 || ttl > wantTTL {
					t.Fatalf("TTL: got %d, want approximately %d", ttl, wantTTL)
				}
			})
		}
	}
}

func TestExpiryMillisecondsComparisonAndPersist(t *testing.T) {
	s := store.NewStore()
	expireAtRun(t, s, "SET", "k", "v", "PXAT", "16725225600000")
	for _, step := range []struct {
		args []string
		want int64
	}{
		{[]string{"PEXPIREAT", "k", "16725225600001", "GT"}, 1},
		{[]string{"PEXPIREAT", "k", "16725225600000", "GT"}, 0},
		{[]string{"PEXPIREAT", "k", "16725225600000", "LT"}, 1},
	} {
		if got := expireAtRun(t, s, step.args...).Int; got != step.want {
			t.Fatalf("%v: got %d, want %d", step.args, got, step.want)
		}
	}
	expireAtRun(t, s, "SET", "k", "new", "KEEPTTL")
	if got := expireAtRun(t, s, "PEXPIRETIME", "k").Int; got != 16725225600000 {
		t.Fatalf("KEEPTTL: got %d", got)
	}
	if got := expireAtRun(t, s, "PERSIST", "k").Int; got != 1 {
		t.Fatalf("PERSIST: got %d", got)
	}
	if got := expireAtRun(t, s, "PTTL", "k").Int; got != -1 {
		t.Fatalf("PTTL after PERSIST: got %d", got)
	}
}
