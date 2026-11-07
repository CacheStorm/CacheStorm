package server

import (
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
	checked := 0
	for name := range s.router.Commands() {
		for n := 0; n <= 6; n++ {
			for range 8 {
				args := make([][]byte, n)
				for i := range args {
					args[i] = []byte(words[rng.Intn(len(words))])
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
	if checked < 1000 {
		t.Fatalf("only %d shapes had a sole first key", checked)
	}
}
