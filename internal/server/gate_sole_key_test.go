package server

import (
	"fmt"
	"math/rand"
	"slices"
	"testing"

	"github.com/cachestorm/cachestorm/internal/command"
)

// The write gate takes a command's stripe from command.SoleFirstKey without
// building its keys; for every registered command and argument shape that
// must be exactly the key CommandKeys names, or two writes to one key could
// hold different stripes.
func TestSoleFirstKeyAgreesWithCommandKeys(t *testing.T) {
	s := startReplServerCfg(t, replTestConfig(t, nil))
	rng := rand.New(rand.NewSource(1))
	words := []string{"k", "0", "1", "2", "-1", "STORE", "BY", "GET", "#", "WITHSCORES", "STREAMS", "KEYS", "LEFT", "RIGHT", "nosort", "x"}
	checked, checkedAll := 0, 0
	for name := range s.router.Commands() {
		for n := 0; n <= 6; n++ {
			for range 8 {
				args := make([][]byte, n)
				for i := range args {
					args[i] = []byte(words[rng.Intn(len(words))])
				}
				if command.AllArgsAreKeys(name, args) {
					checkedAll++
					if want := command.CommandKeys(name, args); !slices.Equal(want, toStrs(args)) {
						t.Fatalf("%s %q: AllArgsAreKeys, CommandKeys %q", name, args, want)
					}
				}
				key, ok := command.SoleFirstKey(name, args)
				if !ok {
					continue
				}
				checked++
				if want := command.CommandKeys(name, args); !slices.Equal(want, []string{string(key)}) {
					t.Fatalf("%s %q: SoleFirstKey %q, CommandKeys %q", name, args, key, want)
				}
			}
		}
	}
	for _, c := range []string{"SET", "get", "INCR", "LPUSH", "HSET", "ZADD", "SADD", "XADD"} {
		if _, ok := command.SoleFirstKey(c, [][]byte{[]byte("k"), []byte("v"), []byte("w")}); !ok {
			t.Errorf("%s: no sole first key", c)
		}
	}
	for _, c := range []string{"MSET", "DEL", "RENAME", "SUNIONSTORE", "EVAL", "PING", "LMOVE"} {
		if k, ok := command.SoleFirstKey(c, [][]byte{[]byte("a"), []byte("b"), []byte("0")}); ok {
			t.Errorf("%s: sole first key %q", c, k)
		}
	}
	if checked < 1000 || checkedAll < 50 {
		t.Fatalf("only %d shapes had a sole first key, %d all keys", checked, checkedAll)
	}
	for _, c := range []string{"MGET", "del", "EXISTS", "UNLINK", "WATCH"} {
		if !command.AllArgsAreKeys(c, [][]byte{[]byte("a"), []byte("b")}) {
			t.Errorf("%s: not all arguments are keys", c)
		}
	}
	for _, c := range []string{"SET", "MSET", "LMOVE", "EVAL", "PING", "SINTERCARD"} {
		if command.AllArgsAreKeys(c, [][]byte{[]byte("1"), []byte("b"), []byte("c")}) {
			t.Errorf("%s: all arguments taken as keys", c)
		}
	}
}

func toStrs(args [][]byte) []string {
	out := make([]string, len(args))
	for i, a := range args {
		out[i] = string(a)
	}
	return out
}

// A command over more keys than the hold's inline array plans its stripes
// into a buffer the hold keeps, sorted and without duplicates, so the next
// such command does not allocate.
func TestGateHoldReusesItsSpillBuffer(t *testing.T) {
	var g writeGate
	ctx := &command.Context{Command: "MGET"}
	for i := range 40 {
		ctx.Args = append(ctx.Args, []byte(fmt.Sprint("key", i%30)))
	}
	var h gateHold
	g.planRead(&h, ctx)
	want := map[int]bool{}
	for _, a := range ctx.Args {
		want[stripeOf(a)] = true
	}
	if len(h.stripes) != len(want) || !slices.IsSorted(h.stripes) {
		t.Fatalf("stripes %v, want the %d distinct ones sorted", h.stripes, len(want))
	}
	for _, s := range h.stripes {
		if !want[s] {
			t.Fatalf("stripe %d is no key's", s)
		}
	}
	if allocs := testing.AllocsPerRun(50, func() { g.planRead(&h, ctx) }); allocs != 0 {
		t.Fatalf("planning again allocated %v times", allocs)
	}
	ctx.Args = ctx.Args[:2]
	g.planRead(&h, ctx)
	if len(h.stripes) > 2 || &h.stripes[:1][0] != &h.inline[0] {
		t.Fatalf("two keys: stripes %v not inline", h.stripes)
	}
}
