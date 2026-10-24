package command

import (
	"bytes"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression tests for the string and generic key commands, pinned to the
// replies of Redis 7.4 as measured by tests/compat.

type compatSession struct {
	t      *testing.T
	s      *store.Store
	router *Router
}

func newCompatSession(t *testing.T) *compatSession {
	t.Helper()
	router := NewRouter()
	RegisterStringCommands(router)
	RegisterKeyCommands(router)
	RegisterServerCommands(router)
	RegisterListCommands(router)
	RegisterHashCommands(router)
	RegisterBitmapCommands(router)
	return &compatSession{t: t, s: store.NewStore(), router: router}
}

func (c *compatSession) do(cmd string, args ...string) string {
	c.t.Helper()
	parsed := make([][]byte, len(args))
	for i, a := range args {
		parsed[i] = []byte(a)
	}
	var buf bytes.Buffer
	if err := c.router.Execute(NewContext(cmd, parsed, c.s, resp.NewWriter(&buf))); err != nil {
		c.t.Fatalf("%s %q: %v", cmd, args, err)
	}
	return buf.String()
}

func (c *compatSession) expect(want, cmd string, args ...string) {
	c.t.Helper()
	if got := c.do(cmd, args...); got != want {
		c.t.Fatalf("%s %q = %q, want %q", cmd, args, got, want)
	}
}

func TestParseRedisIntMatchesString2ll(t *testing.T) {
	for in, want := range map[string]bool{
		"0": true, "1": true, "-1": true, "9223372036854775807": true, "-9223372036854775808": true,
		"": false, "-": false, "+1": false, "01": false, "-0": false, " 1": false, "1 ": false,
		"1.0": false, "9223372036854775808": false, "abc": false,
	} {
		if _, ok := parseRedisInt([]byte(in)); ok != want {
			t.Errorf("parseRedisInt(%q) ok = %v, want %v", in, ok, want)
		}
	}
}

func TestIncrByFloatLongDoubleFormatting(t *testing.T) {
	c := newCompatSession(t)
	c.expect("$3\r\n0.1\r\n", "INCRBYFLOAT", "f", "0.1")
	c.expect("$3\r\n0.3\r\n", "INCRBYFLOAT", "f", "0.2")
	c.expect("$1\r\n0\r\n", "INCRBYFLOAT", "c", "-0")
	c.expect("$1\r\n0\r\n", "INCRBYFLOAT", "e", "1e-20")
	c.expect("$19\r\n1.23456789012345678\r\n", "INCRBYFLOAT", "g", "1.23456789012345678")
	c.expect("$21\r\n100000000000000000000\r\n", "INCRBYFLOAT", "a", "1e20")
	c.do("SET", "h", "5.0e3")
	c.expect("$4\r\n5200\r\n", "INCRBYFLOAT", "h", "2.0e2")
	c.do("SET", "big", "1.7e308")
	got := c.do("INCRBYFLOAT", "big", "1.7e308")
	if !strings.HasPrefix(got, "$309\r\n34000000000000000000627") {
		t.Fatalf("1.7e308+1.7e308 = %q, want the 309-digit long double sum", got)
	}
}

func TestIncrByFloatErrors(t *testing.T) {
	c := newCompatSession(t)
	c.expect("-ERR value is not a valid float\r\n", "INCRBYFLOAT", "f", "abc")
	c.expect("-ERR value is not a valid float\r\n", "INCRBYFLOAT", "f", "nan")
	c.expect("-ERR value is not a valid float\r\n", "INCRBYFLOAT", "f", " 1")
	c.expect("-ERR increment would produce NaN or Infinity\r\n", "INCRBYFLOAT", "f", "inf")
	c.do("SET", "s", "abc")
	c.expect("-ERR value is not a valid float\r\n", "INCRBYFLOAT", "s", "1")
	c.expect(":0\r\n", "EXISTS", "f")
}

func TestIncrStrictIntegersAndOverflow(t *testing.T) {
	c := newCompatSession(t)
	c.do("SET", "z", "01")
	c.expect("-ERR value is not an integer or out of range\r\n", "INCR", "z")
	c.expect("-ERR decrement would overflow\r\n", "DECRBY", "y", "-9223372036854775808")
	c.expect(":0\r\n", "EXISTS", "y")
	c.expect("-ERR value is not an integer or out of range\r\n", "INCRBY", "n", "+1")
}

func TestSetOptionsFollowRedis(t *testing.T) {
	c := newCompatSession(t)
	c.expect("$-1\r\n", "SET", "k", "v", "NX", "GET")
	c.expect("$1\r\nv\r\n", "SET", "k", "v2", "NX", "GET")
	c.expect("$1\r\nv\r\n", "GET", "k")
	c.expect("-ERR syntax error\r\n", "SET", "x", "v", "EX", "10", "PX", "100")
	c.expect("-ERR syntax error\r\n", "SET", "x", "v", "KEEPTTL", "EX", "10")
	c.expect("-ERR syntax error\r\n", "SET", "x", "v", "NX", "XX")
	c.expect(":0\r\n", "EXISTS", "x")
	c.expect("-ERR invalid expire time in 'set' command\r\n", "SET", "x", "v", "EX", "0")
	c.expect("-ERR invalid expire time in 'set' command\r\n", "SET", "x", "v", "EX", "9223372036854775807")
	c.expect("-ERR value is not an integer or out of range\r\n", "SET", "x", "v", "EX", "abc")
	c.expect("+OK\r\n", "SET", "x", "v", "ex", "100")
	if ttl := c.do("TTL", "x"); ttl != ":100\r\n" && ttl != ":99\r\n" {
		t.Fatalf("lowercase ex: TTL = %q", ttl)
	}
	c.expect("+OK\r\n", "SET", "x", "v", "PXAT", "1")
	c.expect(":0\r\n", "EXISTS", "x")
	c.do("RPUSH", "l", "a")
	c.expect("-WRONGTYPE Operation against a key holding the wrong kind of value\r\n", "SET", "l", "v", "GET")
	c.expect("-ERR invalid expire time in 'setex' command\r\n", "SETEX", "b", "0", "v")
	c.expect("-ERR invalid expire time in 'psetex' command\r\n", "PSETEX", "b", "0", "v")
}

func TestGetexOptionsFollowRedis(t *testing.T) {
	c := newCompatSession(t)
	c.do("SET", "k", "v")
	c.expect("-ERR invalid expire time in 'getex' command\r\n", "GETEX", "k", "EX", "0")
	c.expect("-ERR syntax error\r\n", "GETEX", "k", "EX", "10", "PX", "100")
	c.expect("-ERR syntax error\r\n", "GETEX", "k", "EX", "10", "PERSIST")
	c.expect(":-1\r\n", "TTL", "k")
	c.expect("$-1\r\n", "GETEX", "missing", "EX", "0")
	c.expect("$1\r\nv\r\n", "GETEX", "k", "EXAT", "1")
	c.expect(":0\r\n", "EXISTS", "k")
}

func TestSetrangeEdgeCases(t *testing.T) {
	c := newCompatSession(t)
	c.expect(":0\r\n", "SETRANGE", "e", "0", "")
	c.expect(":0\r\n", "SETRANGE", "e", "10", "")
	c.expect(":0\r\n", "EXISTS", "e")
	c.do("SET", "s", "Hello World")
	c.expect(":11\r\n", "SETRANGE", "s", "20", "")
	c.expect("-ERR offset is out of range\r\n", "SETRANGE", "s", "-1", "x")
	c.expect("-ERR string exceeds maximum allowed size (proto-max-bulk-len)\r\n", "SETRANGE", "s", "536870912", "x")
	c.expect(":11\r\n", "SETRANGE", "s", "6", "Redis")
	c.expect("$11\r\nHello Redis\r\n", "GET", "s")
}

func TestSetrangeDoesNotMutateReadValue(t *testing.T) {
	c := newCompatSession(t)
	c.do("SET", "s", "abc")
	entry, _ := c.s.Get("s")
	before := entry.Value.(*store.StringValue).Data
	c.expect(":3\r\n", "SETRANGE", "s", "0", "X")
	if string(before) != "abc" {
		t.Fatalf("SETRANGE wrote into the previous value's bytes: %q", before)
	}
}

func TestMsetOddArgsArityError(t *testing.T) {
	c := newCompatSession(t)
	c.expect("-ERR wrong number of arguments for 'mset' command\r\n", "MSET", "a", "1", "b")
	c.expect("-ERR wrong number of arguments for 'msetnx' command\r\n", "MSETNX", "a", "1", "b")
}

func TestBinaryKeysAccepted(t *testing.T) {
	c := newCompatSession(t)
	key := "k\x00\r\nkey"
	c.expect("+OK\r\n", "SET", key, "v")
	c.expect("$1\r\nv\r\n", "GET", key)
	c.expect("+OK\r\n", "MSET", key, "w")
	c.expect("$1\r\nw\r\n", "GET", key)
}

func TestPersistOnlyCountsRemovedExpiry(t *testing.T) {
	c := newCompatSession(t)
	c.do("SET", "k", "v")
	c.expect(":0\r\n", "PERSIST", "k")
	c.do("EXPIRE", "k", "100")
	c.expect(":1\r\n", "PERSIST", "k")
	c.expect(":0\r\n", "PERSIST", "k")
	c.expect(":0\r\n", "PERSIST", "missing")
}

func TestExpireOptionsFollowRedis(t *testing.T) {
	c := newCompatSession(t)
	c.do("SET", "k", "v")
	nxErr := "-ERR NX and XX, GT or LT options at the same time are not compatible\r\n"
	c.expect(nxErr, "EXPIRE", "k", "100", "NX", "XX")
	c.expect(nxErr, "EXPIRE", "k", "100", "NX", "GT")
	c.expect("-ERR GT and LT options at the same time are not compatible\r\n", "EXPIRE", "k", "100", "GT", "LT")
	c.expect("-ERR Unsupported option BOGUS\r\n", "EXPIRE", "k", "100", "BOGUS")
	c.expect(nxErr, "EXPIRE", "k", "abc", "NX", "XX")
	c.expect("-ERR invalid expire time in 'expire' command\r\n", "EXPIRE", "k", "9223372036854775807")
	c.expect(":0\r\n", "EXPIRE", "k", "100", "GT")
	c.expect(":1\r\n", "EXPIRE", "k", "100", "LT")
	c.expect(":0\r\n", "EXPIRE", "k", "200", "LT")
	c.expect(":1\r\n", "EXPIRE", "k", "200", "XX", "GT")
	c.expect(":0\r\n", "EXPIRE", "k", "300", "NX")
	c.expect(":1\r\n", "PEXPIRE", "k", "0")
	c.expect(":0\r\n", "EXISTS", "k")
}

func TestRenameMissingKeyError(t *testing.T) {
	c := newCompatSession(t)
	c.expect("-ERR no such key\r\n", "RENAME", "missing", "x")
	c.expect("-ERR no such key\r\n", "RENAMENX", "missing", "x")
	c.do("SET", "a", "1")
	c.do("EXPIRE", "a", "100")
	c.expect("+OK\r\n", "RENAME", "a", "a")
	c.expect("+OK\r\n", "RENAME", "a", "b")
	if ttl := c.do("TTL", "b"); ttl != ":100\r\n" && ttl != ":99\r\n" {
		t.Fatalf("RENAME dropped the TTL: %q", ttl)
	}
}

func TestKeysGlobClassesAndEscapes(t *testing.T) {
	c := newCompatSession(t)
	for _, k := range []string{"user:1", "user:2", "user:3", "user:4", "user:10", "user:*", "a?b"} {
		c.do("SET", k, "v")
	}
	keysOf := func(pattern string) []string {
		v, err := resp.NewReader(strings.NewReader(c.do("KEYS", pattern))).ReadValue()
		if err != nil {
			t.Fatal(err)
		}
		out := make([]string, 0, len(v.Array))
		for _, e := range v.Array {
			out = append(out, string(e.Bulk))
		}
		sort.Strings(out)
		return out
	}
	for pattern, want := range map[string]string{
		"user:[1-3]*": "user:1 user:10 user:2 user:3",
		"user:[^1-3]": "user:* user:4",
		`user:\*`:     "user:*",
		`a\?b`:        "a?b",
		"user:[13]":   "user:1 user:3",
	} {
		if got := strings.Join(keysOf(pattern), " "); got != want {
			t.Errorf("KEYS %q = %q, want %q", pattern, got, want)
		}
	}
}

func scanAll(t *testing.T, c *compatSession, args ...string) []string {
	t.Helper()
	cursor := "0"
	seen := map[string]bool{}
	for i := 0; ; i++ {
		if i > 10000 {
			t.Fatal("SCAN did not terminate")
		}
		v, err := resp.NewReader(strings.NewReader(c.do("SCAN", append([]string{cursor}, args...)...))).ReadValue()
		if err != nil || len(v.Array) != 2 {
			t.Fatalf("SCAN reply: %v %v", v, err)
		}
		for _, e := range v.Array[1].Array {
			seen[string(e.Bulk)] = true
		}
		cursor = string(v.Array[0].Bulk)
		if cursor == "0" {
			break
		}
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func TestScanTypeFilterAndErrors(t *testing.T) {
	c := newCompatSession(t)
	for i := 0; i < 12; i++ {
		c.do("SET", "user:"+strconv.Itoa(i), "v")
		c.do("RPUSH", "list:"+strconv.Itoa(i), "a")
		c.do("HSET", "hash:"+strconv.Itoa(i), "f", "v")
	}
	if got := scanAll(t, c, "TYPE", "list"); len(got) != 12 || got[0] != "list:0" {
		t.Fatalf("SCAN TYPE list = %v", got)
	}
	if got := scanAll(t, c, "TYPE", "string", "MATCH", "user:1*"); strings.Join(got, " ") != "user:1 user:10 user:11" {
		t.Fatalf("SCAN TYPE string MATCH user:1* = %v", got)
	}
	if got := scanAll(t, c, "TYPE", "bogus"); len(got) != 0 {
		t.Fatalf("SCAN TYPE bogus = %v", got)
	}
	c.expect("-ERR invalid cursor\r\n", "SCAN", "abc")
	c.expect("-ERR syntax error\r\n", "SCAN", "0", "COUNT", "0")
	c.expect("-ERR value is not an integer or out of range\r\n", "SCAN", "0", "COUNT", "abc")
	c.expect("-ERR syntax error\r\n", "SCAN", "0", "MATCH")
}

// A key that exists for the whole iteration must be returned even when other
// keys are deleted between SCAN calls; an index into an unordered key list
// could skip it.
func TestScanReturnsStableKeysDespiteDeletes(t *testing.T) {
	c := newCompatSession(t)
	for i := 0; i < 300; i++ {
		c.do("SET", "k"+strconv.Itoa(i), "v")
	}
	cursor := "0"
	seen := map[string]bool{}
	deleted := map[string]bool{}
	next := 0
	for {
		v, _ := resp.NewReader(strings.NewReader(c.do("SCAN", cursor, "COUNT", "7"))).ReadValue()
		for _, e := range v.Array[1].Array {
			seen[string(e.Bulk)] = true
		}
		for j := 0; j < 5 && next < 300; j++ {
			k := "k" + strconv.Itoa(next)
			next += 2
			if !seen[k] {
				c.do("DEL", k)
				deleted[k] = true
			}
		}
		cursor = string(v.Array[0].Bulk)
		if cursor == "0" {
			break
		}
	}
	for i := 0; i < 300; i++ {
		k := "k" + strconv.Itoa(i)
		if !deleted[k] && !seen[k] {
			t.Fatalf("SCAN skipped %s, which existed for the whole iteration", k)
		}
	}
}

func TestBitmapReadersRejectWrongType(t *testing.T) {
	c := newCompatSession(t)
	c.do("RPUSH", "L", "a")
	wrong := "-WRONGTYPE Operation against a key holding the wrong kind of value\r\n"
	c.expect(wrong, "BITCOUNT", "L")
	c.expect(wrong, "GETBIT", "L", "0")
	c.expect(wrong, "BITPOS", "L", "1")
	c.expect(wrong, "BITOP", "OR", "dst", "L")
	c.expect(":0\r\n", "EXISTS", "dst")
}
