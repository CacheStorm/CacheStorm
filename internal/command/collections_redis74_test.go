package command

import (
	"bytes"
	"fmt"
	"math"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression tests for list, hash, set and sorted-set replies measured
// against Redis 7.4 by the differential harness in tests/compat.

func collRouter() *Router {
	r := NewRouter()
	RegisterListCommands(r)
	RegisterHashCommands(r)
	RegisterSetCommands(r)
	RegisterSortedSetCommands(r)
	return r
}

func collRun(t *testing.T, s *store.Store, r *Router, args ...string) string {
	t.Helper()
	argv := make([][]byte, len(args)-1)
	for i, a := range args[1:] {
		argv[i] = []byte(a)
	}
	buf := &bytes.Buffer{}
	_ = r.Execute(NewContext(args[0], argv, s, resp.NewWriter(buf)))
	return buf.String()
}

func collExpect(t *testing.T, s *store.Store, r *Router, want string, args ...string) {
	t.Helper()
	if got := collRun(t, s, r, args...); got != want {
		t.Fatalf("%v:\n got  %q\n want %q", args, got, want)
	}
}

func collParse(t *testing.T, raw string) *resp.Value {
	t.Helper()
	v, err := resp.NewReader(strings.NewReader(raw)).ReadValue()
	if err != nil {
		t.Fatalf("parse %q: %v", raw, err)
	}
	return v
}

// scanAllMembers runs a full cursor loop and returns how often each element
// came back. pairs says whether the reply alternates element and value.
func scanAllMembers(t *testing.T, s *store.Store, r *Router, cmd, key string, pairs bool, between func()) map[string]int {
	t.Helper()
	seen := map[string]int{}
	cursor := "0"
	for rounds := 0; ; rounds++ {
		if rounds > 100000 {
			t.Fatalf("%s loop did not terminate", cmd)
		}
		v := collParse(t, collRun(t, s, r, cmd, key, cursor, "COUNT", "13"))
		if v.Type != resp.TypeArray || len(v.Array) != 2 {
			t.Fatalf("%s reply %v", cmd, v)
		}
		step := 1
		if pairs {
			step = 2
		}
		for i := 0; i < len(v.Array[1].Array); i += step {
			seen[string(v.Array[1].Array[i].Bulk)]++
		}
		cursor = string(v.Array[0].Bulk)
		if cursor == "0" {
			return seen
		}
		if between != nil {
			between()
		}
	}
}

// The SCAN guarantee: an element present for the whole scan is returned,
// whatever is added or removed between calls.
func TestCollectionScanGuaranteeUnderChurn(t *testing.T) {
	for _, tc := range []struct {
		cmd, add, rem string
		pairs         bool
		addArgs       func(m string) []string
	}{
		{"SSCAN", "SADD", "SREM", false, func(m string) []string { return []string{m} }},
		{"HSCAN", "HSET", "HDEL", true, func(m string) []string { return []string{m, "v"} }},
		{"ZSCAN", "ZADD", "ZREM", true, func(m string) []string { return []string{"1", m} }},
	} {
		t.Run(tc.cmd, func(t *testing.T) {
			s := store.NewStore()
			r := collRouter()
			const stable = 1000
			for i := 0; i < stable; i++ {
				collRun(t, s, r, append([]string{tc.add, "k"}, tc.addArgs(fmt.Sprintf("stable-%d", i))...)...)
			}
			n := 0
			seen := scanAllMembers(t, s, r, tc.cmd, "k", tc.pairs, func() {
				n++
				collRun(t, s, r, append([]string{tc.add, "k"}, tc.addArgs(fmt.Sprintf("churn-%d", n))...)...)
				if n%2 == 0 {
					collRun(t, s, r, tc.rem, "k", fmt.Sprintf("churn-%d", n-1))
				}
			})
			for i := 0; i < stable; i++ {
				m := fmt.Sprintf("stable-%d", i)
				if seen[m] != 1 {
					t.Fatalf("%s returned %s %d times, want once", tc.cmd, m, seen[m])
				}
			}
		})
	}
}

func TestCollectionScanSmallAndOptions(t *testing.T) {
	s := store.NewStore()
	r := collRouter()
	collRun(t, s, r, "HSET", "h", "a", "1", "bb", "2")
	collExpect(t, s, r, "*2\r\n$1\r\n0\r\n*1\r\n$1\r\na\r\n", "HSCAN", "h", "0", "MATCH", "?", "NOVALUES", "COUNT", "1")
	collExpect(t, s, r, "-ERR invalid cursor\r\n", "HSCAN", "h", "-1")
	collExpect(t, s, r, "-ERR syntax error\r\n", "HSCAN", "h", "0", "COUNT", "0")
	collExpect(t, s, r, "-ERR value is not an integer or out of range\r\n", "HSCAN", "h", "0", "COUNT", "x")
	collRun(t, s, r, "SADD", "s", "a")
	collExpect(t, s, r, "-ERR NOVALUES option can only be used in HSCAN\r\n", "SSCAN", "s", "0", "NOVALUES")
	collExpect(t, s, r, "*2\r\n$1\r\n0\r\n*0\r\n", "SSCAN", "missing", "0", "COUNT", "0")
}

// A scan of a large collection runs under the collection's read lock while
// writers keep mutating it.
func TestCollectionScanConcurrentWriters(t *testing.T) {
	s := store.NewStore()
	r := collRouter()
	for i := 0; i < 300; i++ {
		collRun(t, s, r, "SADD", "s", fmt.Sprintf("m%d", i))
		collRun(t, s, r, "ZADD", "z", "1", fmt.Sprintf("m%d", i))
		collRun(t, s, r, "HSET", "h", fmt.Sprintf("m%d", i), "v")
	}
	var wg sync.WaitGroup
	stop := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			m := fmt.Sprintf("x%d", i%50)
			collRun(t, s, r, "SADD", "s", m)
			collRun(t, s, r, "SREM", "s", m)
			collRun(t, s, r, "ZADD", "z", "2", m)
			collRun(t, s, r, "ZREM", "z", m)
			collRun(t, s, r, "HSET", "h", m, "v")
			collRun(t, s, r, "HDEL", "h", m)
		}
	}()
	for i := 0; i < 5; i++ {
		scanAllMembers(t, s, r, "SSCAN", "s", false, nil)
		scanAllMembers(t, s, r, "ZSCAN", "z", true, nil)
		scanAllMembers(t, s, r, "HSCAN", "h", true, nil)
	}
	close(stop)
	wg.Wait()
}

func TestListPopCount(t *testing.T) {
	s := store.NewStore()
	r := collRouter()
	collRun(t, s, r, "RPUSH", "l", "a", "b", "c", "d")
	collExpect(t, s, r, "*2\r\n$1\r\na\r\n$1\r\nb\r\n", "LPOP", "l", "2")
	collExpect(t, s, r, "*2\r\n$1\r\nd\r\n$1\r\nc\r\n", "RPOP", "l", "5")
	if _, ok := s.Get("l"); ok {
		t.Fatal("emptied list kept")
	}
	collExpect(t, s, r, "*-1\r\n", "LPOP", "l", "2")
	collExpect(t, s, r, "$-1\r\n", "LPOP", "l")
	collRun(t, s, r, "RPUSH", "m", "x")
	collExpect(t, s, r, "*0\r\n", "LPOP", "m", "0")
	collExpect(t, s, r, "-ERR value is out of range, must be positive\r\n", "LPOP", "m", "-1")
	collExpect(t, s, r, "-ERR value is out of range, must be positive\r\n", "RPOP", "m", "x")
	collExpect(t, s, r, "-ERR wrong number of arguments for 'lpop' command\r\n", "LPOP", "m", "1", "2")
}

func TestListErrorsAndLPos(t *testing.T) {
	s := store.NewStore()
	r := collRouter()
	collExpect(t, s, r, "-ERR no such key\r\n", "LSET", "missing", "0", "x")
	collRun(t, s, r, "RPUSH", "l", "a", "b", "c", "1", "2", "3", "c", "c")
	collExpect(t, s, r, "*0\r\n", "LPOS", "l", "zz", "COUNT", "0")
	collExpect(t, s, r, "*0\r\n", "LPOS", "missing", "a", "COUNT", "1")
	collExpect(t, s, r, "*2\r\n:7\r\n:6\r\n", "LPOS", "l", "c", "RANK", "-1", "COUNT", "2")
	collExpect(t, s, r, "*2\r\n:2\r\n:6\r\n", "LPOS", "l", "c", "COUNT", "0", "MAXLEN", "7")
	collExpect(t, s, r, "-ERR COUNT can't be negative\r\n", "LPOS", "l", "c", "COUNT", "-1")
	collExpect(t, s, r, "-ERR MAXLEN can't be negative\r\n", "LPOS", "l", "c", "MAXLEN", "-1")
	collExpect(t, s, r, "-"+errLPosRankZero.Error()+"\r\n", "LPOS", "l", "c", "RANK", "0")
	collExpect(t, s, r, "-"+errLPosRank.Error()+"\r\n", "LPOS", "l", "c", "RANK", "-9223372036854775808")
	collExpect(t, s, r, "-ERR syntax error\r\n", "LPOS", "l", "c", "BOGUS", "1")

	collExpect(t, s, r, "-ERR numkeys should be greater than 0\r\n", "LMPOP", "0", "a", "LEFT")
	collExpect(t, s, r, "-ERR numkeys should be greater than 0\r\n", "LMPOP", "abc", "a", "LEFT")
	collExpect(t, s, r, "-ERR syntax error\r\n", "LMPOP", "2", "a", "LEFT")
	collExpect(t, s, r, "-ERR count should be greater than 0\r\n", "LMPOP", "1", "a", "LEFT", "COUNT", "0")
	collExpect(t, s, r, "-ERR syntax error\r\n", "LMPOP", "1", "a", "LEFT", "COUNT")
}

func TestListMoveSameKeyAndTypes(t *testing.T) {
	s := store.NewStore()
	r := collRouter()
	collRun(t, s, r, "RPUSH", "l", "a", "b", "c")
	collExpect(t, s, r, "$1\r\nc\r\n", "RPOPLPUSH", "l", "l")
	collExpect(t, s, r, "*3\r\n$1\r\nc\r\n$1\r\na\r\n$1\r\nb\r\n", "LRANGE", "l", "0", "-1")
	collExpect(t, s, r, "$1\r\nc\r\n", "LMOVE", "l", "l", "LEFT", "RIGHT")
	collExpect(t, s, r, "*3\r\n$1\r\na\r\n$1\r\nb\r\n$1\r\nc\r\n", "LRANGE", "l", "0", "-1")
	if err := s.Set("str", &store.StringValue{Data: []byte("v")}, store.SetOptions{}); err != nil {
		t.Fatal(err)
	}
	collExpect(t, s, r, "-WRONGTYPE Operation against a key holding the wrong kind of value\r\n", "LMOVE", "l", "str", "LEFT", "LEFT")
	collExpect(t, s, r, "$-1\r\n", "RPOPLPUSH", "missing", "str")
	collExpect(t, s, r, "$1\r\na\r\n", "LMOVE", "l", "dst", "LEFT", "LEFT")
	collExpect(t, s, r, "*1\r\n$1\r\na\r\n", "LRANGE", "dst", "0", "-1")
}

// RPOPLPUSH and LMOVE used to mutate both lists without their locks.
func TestListMoveConcurrentWithPush(t *testing.T) {
	s := store.NewStore()
	r := collRouter()
	var wg sync.WaitGroup
	const n = 300
	wg.Add(3)
	go func() {
		defer wg.Done()
		for i := 0; i < n; i++ {
			collRun(t, s, r, "RPUSH", "src", "x")
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < n; i++ {
			collRun(t, s, r, "RPOPLPUSH", "src", "dst")
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < n; i++ {
			collRun(t, s, r, "LMOVE", "dst", "src", "LEFT", "RIGHT")
			collRun(t, s, r, "LPUSH", "dst", "y")
		}
	}()
	wg.Wait()
}

func TestBlockingTimeoutsAndTypes(t *testing.T) {
	s := store.NewStore()
	r := collRouter()
	collExpect(t, s, r, "-ERR timeout is out of range\r\n", "BLPOP", "q", "inf")
	collExpect(t, s, r, "-ERR timeout is negative\r\n", "BLPOP", "q", "-1")
	collExpect(t, s, r, "-ERR timeout is not a float or out of range\r\n", "BLPOP", "q", "abc")
	collExpect(t, s, r, "-ERR timeout is out of range\r\n", "BLPOP", "q", "1e17")
	collExpect(t, s, r, "*-1\r\n", "BLMOVE", "a", "b", "LEFT", "RIGHT", "0.01")
	collExpect(t, s, r, "*-1\r\n", "BRPOPLPUSH", "a", "b", "0.01")

	if err := s.Set("str", &store.StringValue{Data: []byte("v")}, store.SetOptions{}); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	collExpect(t, s, r, "-WRONGTYPE Operation against a key holding the wrong kind of value\r\n", "BLPOP", "missing", "str", "5")
	collExpect(t, s, r, "-WRONGTYPE Operation against a key holding the wrong kind of value\r\n", "BZPOPMIN", "str", "5")
	if time.Since(start) > time.Second {
		t.Fatal("WRONGTYPE was not immediate")
	}
	collRun(t, s, r, "RPUSH", "l", "v")
	collExpect(t, s, r, "*2\r\n$1\r\nl\r\n$1\r\nv\r\n", "BLPOP", "l", "str", "5")

	for arg, want := range map[string]time.Duration{"0.0001": time.Millisecond, "1.5": 1500 * time.Millisecond, "0": blockForever, "-0.0001": blockForever} {
		got, err := parseBlockTimeout(arg)
		if err != nil || got != want {
			t.Fatalf("parseBlockTimeout(%q) = %v, %v; want %v", arg, got, err, want)
		}
	}
}

// Blocking pops read a list or zset only under its lock, so they never race
// the locked writes of concurrent pushes (run under -race).
func TestBlockingPopsRaceFreeWithPushes(t *testing.T) {
	s := store.NewStore()
	r := collRouter()
	const n = 200
	var wg sync.WaitGroup
	for _, block := range [][]string{
		{"BLPOP", "l", "1"},
		{"BRPOP", "l", "1"},
		{"BLMOVE", "l", "dst", "LEFT", "RIGHT", "1"},
		{"BLMPOP", "1", "1", "l", "LEFT", "COUNT", "2"},
		{"BZPOPMIN", "z", "1"},
		{"BZMPOP", "1", "1", "z", "MAX", "COUNT", "2"},
	} {
		wg.Add(1)
		go func(args []string) {
			defer wg.Done()
			for i := 0; i < n/4; i++ {
				collRun(t, s, r, args...)
			}
		}(block)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < n; i++ {
			collRun(t, s, r, "RPUSH", "l", "a", "b")
			collRun(t, s, r, "ZADD", "z", fmt.Sprint(i), fmt.Sprintf("m%d", i))
		}
	}()
	wg.Wait()
}

func TestZScoreFormatting(t *testing.T) {
	for f, want := range map[float64]string{
		1.5:                  "1.5",
		3:                    "3",
		1e20:                 "1e+20",
		0.1:                  "0.1",
		math.Copysign(0, -1): "0",
		123456789012345678:   "123456789012345680",
		0.30000000000000004:  "0.30000000000000004",
		math.Inf(1):          "inf",
		math.Inf(-1):         "-inf",
		-2.5:                 "-2.5",
		1e-7:                 "1e-7",
		0.0001:               "0.0001",
		5e18:                 "5e+18",
		1.5e300:              "1.5e+300",
		-1.25e-10:            "-1.25e-10",
		1e15 + 0.5:           "1000000000000000.5",
	} {
		if got := formatZScore(f); got != want {
			t.Errorf("formatZScore(%v) = %q, want %q", f, got, want)
		}
	}
}

func TestZRangeUnifiedSyntax(t *testing.T) {
	s := store.NewStore()
	r := collRouter()
	collRun(t, s, r, "ZADD", "z", "1", "a", "2", "b", "3", "c", "4", "d", "5", "e")
	collExpect(t, s, r, "*3\r\n$1\r\nd\r\n$1\r\nc\r\n$1\r\nb\r\n", "ZRANGE", "z", "4", "2", "BYSCORE", "REV")
	collExpect(t, s, r, "*4\r\n$1\r\ne\r\n$1\r\n5\r\n$1\r\nd\r\n$1\r\n4\r\n", "ZRANGE", "z", "+inf", "-inf", "BYSCORE", "REV", "LIMIT", "0", "2", "WITHSCORES")
	collExpect(t, s, r, "-ERR min or max is not a float\r\n", "ZRANGE", "z", "abc", "2", "BYSCORE")
	collExpect(t, s, r, "-"+errZLimitRank.Error()+"\r\n", "ZRANGE", "z", "0", "-1", "LIMIT", "0", "1")
	collExpect(t, s, r, "-"+errZWithScoresLex.Error()+"\r\n", "ZRANGE", "z", "-", "+", "BYLEX", "WITHSCORES")
	collExpect(t, s, r, "*2\r\n$1\r\ne\r\n$1\r\nd\r\n", "ZREVRANGEBYSCORE", "z", "+inf", "(3")
	collExpect(t, s, r, ":2\r\n", "ZCOUNT", "z", "(1", "3")
	collExpect(t, s, r, ":0\r\n", "ZCOUNT", "z", "(3", "(3")
	collExpect(t, s, r, "-ERR syntax error\r\n", "ZRANGEBYSCORE", "z", "0", "1", "BYSCORE")
	collExpect(t, s, r, ":0\r\n", "ZRANGESTORE", "dst", "z", "2", "4", "BYSCORE", "REV")
	collExpect(t, s, r, ":3\r\n", "ZRANGESTORE", "dst", "z", "4", "2", "BYSCORE", "REV")
	collExpect(t, s, r, "*3\r\n$1\r\nb\r\n$1\r\nc\r\n$1\r\nd\r\n", "ZRANGE", "dst", "0", "-1")
	collExpect(t, s, r, "-ERR syntax error\r\n", "ZRANGESTORE", "dst", "z", "0", "1", "WITHSCORES")
	collExpect(t, s, r, ":1\r\n", "ZREMRANGEBYSCORE", "z", "(2", "3")

	collRun(t, s, r, "ZADD", "lex", "0", "a", "0", "b", "0", "c", "0", "d", "0", "e")
	collExpect(t, s, r, "*2\r\n$1\r\nc\r\n$1\r\nb\r\n", "ZRANGE", "lex", "(d", "[b", "BYLEX", "REV")
	collExpect(t, s, r, "*0\r\n", "ZRANGE", "lex", "+", "-", "BYLEX")
	collExpect(t, s, r, ":0\r\n", "ZLEXCOUNT", "lex", "+", "+")
	collExpect(t, s, r, "-ERR min or max not valid string range item\r\n", "ZLEXCOUNT", "lex", "a", "+")
}

func TestZAddFlags(t *testing.T) {
	s := store.NewStore()
	r := collRouter()
	collExpect(t, s, r, "-"+errZAddXXNX.Error()+"\r\n", "ZADD", "z", "NX", "XX", "1", "a")
	collExpect(t, s, r, "-"+errZAddGTLTNX.Error()+"\r\n", "ZADD", "z", "GT", "LT", "1", "a")
	collExpect(t, s, r, "-"+errZAddGTLTNX.Error()+"\r\n", "ZADD", "z", "NX", "GT", "1", "a")
	collExpect(t, s, r, "-"+errZAddIncrPair.Error()+"\r\n", "ZADD", "z", "INCR", "1", "a", "2", "b")
	collExpect(t, s, r, "-ERR syntax error\r\n", "ZADD", "z", "1", "a", "2")
	if _, ok := s.Get("z"); ok {
		t.Fatal("rejected ZADD created the key")
	}
	collExpect(t, s, r, ":0\r\n", "ZADD", "z", "XX", "1", "a")
	collExpect(t, s, r, "$-1\r\n", "ZADD", "z", "XX", "INCR", "1", "a")
	collExpect(t, s, r, "$1\r\n5\r\n", "ZADD", "z", "INCR", "5", "a")
	collExpect(t, s, r, "$-1\r\n", "ZADD", "z", "NX", "INCR", "1", "a")
	collExpect(t, s, r, "$-1\r\n", "ZADD", "z", "GT", "INCR", "-1", "a")
	collExpect(t, s, r, "$1\r\n6\r\n", "ZADD", "z", "GT", "INCR", "1", "a")
	collExpect(t, s, r, ":2\r\n", "ZADD", "z", "GT", "CH", "3", "a", "7", "b", "1", "c")
	collExpect(t, s, r, "*6\r\n$1\r\nc\r\n$1\r\n1\r\n$1\r\na\r\n$1\r\n6\r\n$1\r\nb\r\n$1\r\n7\r\n", "ZRANGE", "z", "0", "-1", "WITHSCORES")
	collExpect(t, s, r, ":3\r\n", "ZADD", "z", "-inf", "lo", "+inf", "hi", "0", "mid")
	collExpect(t, s, r, "$4\r\n-inf\r\n", "ZSCORE", "z", "lo")
}

func TestZPopAndMPopShapes(t *testing.T) {
	s := store.NewStore()
	r := collRouter()
	collRun(t, s, r, "ZADD", "z", "1", "a", "2", "b", "3", "c", "4", "d", "5", "e")
	collExpect(t, s, r, "*4\r\n$1\r\ne\r\n$1\r\n5\r\n$1\r\nd\r\n$1\r\n4\r\n", "ZPOPMAX", "z", "2")
	collExpect(t, s, r, "-ERR value is out of range, must be positive\r\n", "ZPOPMIN", "z", "-1")
	collExpect(t, s, r, "*2\r\n$1\r\nz\r\n*1\r\n*2\r\n$1\r\na\r\n$1\r\n1\r\n", "ZMPOP", "1", "z", "MIN")
	collExpect(t, s, r, "*2\r\n$1\r\nz\r\n*2\r\n*2\r\n$1\r\nb\r\n$1\r\n2\r\n*2\r\n$1\r\nc\r\n$1\r\n3\r\n", "ZMPOP", "2", "missing", "z", "MIN", "COUNT", "10")
	if _, ok := s.Get("z"); ok {
		t.Fatal("emptied zset kept")
	}
	collExpect(t, s, r, "*-1\r\n", "ZMPOP", "1", "z", "MIN")
	collExpect(t, s, r, "-ERR numkeys should be greater than 0\r\n", "ZMPOP", "0", "z", "MIN")
	collRun(t, s, r, "ZADD", "one", "1", "x")
	collExpect(t, s, r, "*2\r\n$1\r\nx\r\n$1\r\n1\r\n", "ZPOPMIN", "one", "5")
	if _, ok := s.Get("one"); ok {
		t.Fatal("ZPOPMIN left an empty zset behind")
	}
}

func TestZRankWithScoreAndSetOps(t *testing.T) {
	s := store.NewStore()
	r := collRouter()
	collRun(t, s, r, "ZADD", "z", "1", "a", "2", "b", "3", "c", "4", "d", "5", "e")
	collExpect(t, s, r, "*2\r\n:2\r\n$1\r\n3\r\n", "ZRANK", "z", "c", "WITHSCORE")
	collExpect(t, s, r, "*2\r\n:4\r\n$1\r\n1\r\n", "ZREVRANK", "z", "a", "WITHSCORE")
	collExpect(t, s, r, "*-1\r\n", "ZRANK", "z", "zz", "WITHSCORE")
	collExpect(t, s, r, "-ERR syntax error\r\n", "ZRANK", "z", "a", "BOGUS")

	collRun(t, s, r, "ZADD", "a", "1", "x")
	collRun(t, s, r, "SADD", "s", "x", "y")
	collExpect(t, s, r, "*4\r\n$1\r\ny\r\n$1\r\n1\r\n$1\r\nx\r\n$1\r\n2\r\n", "ZUNION", "2", "a", "s", "WITHSCORES")
	collExpect(t, s, r, ":1\r\n", "ZINTERSTORE", "d", "2", "a", "s")
	collExpect(t, s, r, ":1\r\n", "ZINTERCARD", "2", "a", "s")
	collExpect(t, s, r, "-ERR at least 1 input key is needed for 'zunionstore' command\r\n", "ZUNIONSTORE", "d", "0", "a")
	collExpect(t, s, r, "-ERR syntax error\r\n", "ZUNION", "3", "a", "s")
	collExpect(t, s, r, "-ERR weight value is not a float\r\n", "ZUNION", "2", "a", "s", "WEIGHTS", "1", "x")
	collExpect(t, s, r, "-ERR syntax error\r\n", "ZDIFF", "2", "a", "s", "WEIGHTS", "1", "1")
	if err := s.Set("str", &store.StringValue{Data: []byte("v")}, store.SetOptions{}); err != nil {
		t.Fatal(err)
	}
	collExpect(t, s, r, ":1\r\n", "ZUNIONSTORE", "str", "1", "a")
	collExpect(t, s, r, "*1\r\n$1\r\nx\r\n", "ZRANGE", "str", "0", "-1")
	collExpect(t, s, r, "-ERR value is not a valid float\r\n", "ZINCRBY", "z", "abc", "a")
}

func TestHashIncrAndRandField(t *testing.T) {
	s := store.NewStore()
	r := collRouter()
	collExpect(t, s, r, "$3\r\n0.1\r\n", "HINCRBYFLOAT", "h", "f", "0.1")
	collExpect(t, s, r, "$3\r\n0.3\r\n", "HINCRBYFLOAT", "h", "f", "0.2")
	collExpect(t, s, r, "-ERR value is not a valid float\r\n", "HINCRBYFLOAT", "h", "f", "abc")
	collExpect(t, s, r, "$2\r\n16\r\n", "HINCRBYFLOAT", "hex", "f", "0x10")
	collExpect(t, s, r, ":1\r\n", "ZADD", "zhex", "0x10", "m")
	collExpect(t, s, r, "$2\r\n16\r\n", "ZSCORE", "zhex", "m")
	collExpect(t, s, r, "-ERR value is NaN or Infinity\r\n", "HINCRBYFLOAT", "h", "f", "inf")
	collRun(t, s, r, "HSET", "h", "s", "abc")
	collExpect(t, s, r, "-ERR hash value is not a float\r\n", "HINCRBYFLOAT", "h", "s", "1")
	collExpect(t, s, r, "-ERR hash value is not an integer\r\n", "HINCRBY", "h", "s", "1")
	collExpect(t, s, r, "-ERR wrong number of arguments for 'hset' command\r\n", "HSET", "h", "a", "1", "b")

	collRun(t, s, r, "HSET", "r", "a", "1", "b", "2", "c", "3")
	if v := collParse(t, collRun(t, s, r, "HRANDFIELD", "r", "-5", "WITHVALUES")); len(v.Array) != 10 {
		t.Fatalf("HRANDFIELD -5 WITHVALUES returned %d elements", len(v.Array))
	}
	if v := collParse(t, collRun(t, s, r, "HRANDFIELD", "r", "2")); len(v.Array) != 2 || string(v.Array[0].Bulk) == string(v.Array[1].Bulk) {
		t.Fatalf("HRANDFIELD 2 returned %v", v.Array)
	}
	collExpect(t, s, r, "*0\r\n", "HRANDFIELD", "r", "0")
	collExpect(t, s, r, "*0\r\n", "HRANDFIELD", "missing", "3")
	collExpect(t, s, r, "-ERR syntax error\r\n", "HRANDFIELD", "r", "1", "BOGUS")
}

func TestSetCommandErrors(t *testing.T) {
	s := store.NewStore()
	r := collRouter()
	collRun(t, s, r, "SADD", "a", "1", "2", "3", "4")
	collRun(t, s, r, "SADD", "b", "3", "4", "5")
	collExpect(t, s, r, ":2\r\n", "SINTERCARD", "2", "a", "b")
	collExpect(t, s, r, ":1\r\n", "SINTERCARD", "2", "a", "b", "LIMIT", "1")
	collExpect(t, s, r, ":0\r\n", "SINTERCARD", "2", "a", "missing")
	collExpect(t, s, r, "-ERR numkeys should be greater than 0\r\n", "SINTERCARD", "0", "a")
	collExpect(t, s, r, "-ERR Number of keys can't be greater than number of args\r\n", "SINTERCARD", "3", "a", "b")
	collExpect(t, s, r, "-ERR LIMIT can't be negative\r\n", "SINTERCARD", "1", "a", "LIMIT", "-1")
	collExpect(t, s, r, "-ERR syntax error\r\n", "SINTERCARD", "1", "a", "BOGUS")
	collExpect(t, s, r, "-ERR value is out of range, must be positive\r\n", "SPOP", "a", "-1")
	collExpect(t, s, r, "-ERR syntax error\r\n", "SPOP", "a", "1", "2")
	collExpect(t, s, r, "-ERR syntax error\r\n", "SRANDMEMBER", "a", "1", "2")
	if v := collParse(t, collRun(t, s, r, "SRANDMEMBER", "a", "-6")); len(v.Array) != 6 {
		t.Fatalf("SRANDMEMBER -6 returned %d elements", len(v.Array))
	}
}
