package command

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

type hfeRun struct {
	t      *testing.T
	s      *store.Store
	router *Router
}

func newHFERun(t *testing.T) *hfeRun {
	router := NewRouter()
	RegisterHashCommands(router)
	RegisterKeyCommands(router)
	RegisterStringCommands(router)
	return &hfeRun{t: t, s: store.NewStore(), router: router}
}

// do runs a command and returns its RESP reply and what it propagates, each
// propagated command rendered as "NAME arg arg".
func (r *hfeRun) do(args ...string) (string, []string) {
	r.t.Helper()
	parsed := make([][]byte, len(args)-1)
	for i, a := range args[1:] {
		parsed[i] = []byte(a)
	}
	ctx := newTestContext(strings.ToUpper(args[0]), parsed, r.s)
	buf := &bytes.Buffer{}
	ctx.Writer = resp.NewWriter(buf)
	_ = r.router.Execute(ctx)
	cmds, set := ctx.Propagation()
	var prop []string
	if !set {
		prop = []string{strings.Join(args, " ")}
	}
	for _, c := range cmds {
		parts := []string{c.Name}
		for _, a := range c.Args {
			parts = append(parts, string(a))
		}
		prop = append(prop, strings.Join(parts, " "))
	}
	return buf.String(), prop
}

func (r *hfeRun) want(want string, args ...string) {
	r.t.Helper()
	if got, _ := r.do(args...); got != want {
		r.t.Fatalf("%v: got %q, want %q", args, got, want)
	}
}

func ints(n ...int64) string {
	var b strings.Builder
	fmt.Fprintf(&b, "*%d\r\n", len(n))
	for _, v := range n {
		fmt.Fprintf(&b, ":%d\r\n", v)
	}
	return b.String()
}

func bulks(vals ...string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "*%d\r\n", len(vals))
	for _, v := range vals {
		if v == "<nil>" {
			b.WriteString("$-1\r\n")
			continue
		}
		fmt.Fprintf(&b, "$%d\r\n%s\r\n", len(v), v)
	}
	return b.String()
}

func errReply(msg string) string { return "-" + msg + "\r\n" }

func TestHExpirePropagatesAbsoluteMilliseconds(t *testing.T) {
	r := newHFERun(t)
	r.want(":3\r\n", "HSET", "h", "a", "1", "b", "2", "c", "3")
	before := time.Now().UnixMilli()
	reply, prop := r.do("HEXPIRE", "h", "100", "FIELDS", "3", "a", "zz", "b")
	if reply != ints(1, -2, 1) || len(prop) != 1 {
		t.Fatalf("reply %q, propagated %v", reply, prop)
	}
	parts := strings.Fields(prop[0])
	at, _ := strconv.ParseInt(parts[2], 10, 64)
	if parts[0] != "HPEXPIREAT" || parts[1] != "h" || at < before+100000 || at > time.Now().UnixMilli()+100000 ||
		strings.Join(parts[3:], " ") != "FIELDS 2 a b" {
		t.Fatalf("propagated %q", prop[0])
	}

	reply, prop = r.do("HEXPIRE", "h", "0", "FIELDS", "2", "a", "c")
	if reply != ints(2, 2) || fmt.Sprint(prop) != "[HDEL h a HDEL h c]" {
		t.Fatalf("past expiry: reply %q, propagated %v", reply, prop)
	}
	reply, prop = r.do("HPEXPIREAT", "h", "4102444800000", "NX", "FIELDS", "2", "b", "zz")
	if reply != ints(0, -2) || len(prop) != 0 {
		t.Fatalf("no-op: reply %q, propagated %v", reply, prop)
	}
	reply, prop = r.do("HPERSIST", "h", "FIELDS", "2", "b", "zz")
	if reply != ints(1, -2) || fmt.Sprint(prop) != "[HPERSIST h FIELDS 1 b]" {
		t.Fatalf("HPERSIST: reply %q, propagated %v", reply, prop)
	}
	if _, prop = r.do("HPERSIST", "h", "FIELDS", "1", "b"); len(prop) != 0 {
		t.Fatalf("HPERSIST without an expiry propagated %v", prop)
	}
	if _, prop = r.do("HEXPIRE", "missing", "10", "FIELDS", "1", "b"); len(prop) != 0 {
		t.Fatalf("HEXPIRE on a missing key propagated %v", prop)
	}
}

func TestHExpireReplayKeepsPastExpiry(t *testing.T) {
	r := newHFERun(t)
	r.want(":2\r\n", "HSET", "h", "a", "1", "b", "2")
	r.s.BeginLoading()
	r.want(ints(1), "HPEXPIREAT", "h", "1", "FIELDS", "1", "a")
	r.want(ints(-1), "HTTL", "h", "FIELDS", "1", "b")
	r.s.EndLoading()
	r.want(":1\r\n", "HLEN", "h")
	r.want(ints(-2, -1), "HTTL", "h", "FIELDS", "2", "a", "b")
}

func TestHTTLFamily(t *testing.T) {
	r := newHFERun(t)
	r.want(":2\r\n", "HSET", "h", "a", "1", "b", "2")
	r.want(ints(1), "HPEXPIREAT", "h", "4102444800500", "FIELDS", "1", "a")
	r.want(ints(4102444801, -1, -2), "HEXPIRETIME", "h", "FIELDS", "3", "a", "b", "c")
	r.want(ints(4102444800500), "HPEXPIRETIME", "h", "FIELDS", "1", "a")
	reply, _ := r.do("HPTTL", "h", "FIELDS", "1", "a")
	ms, _ := strconv.ParseInt(strings.TrimSpace(strings.Split(reply, ":")[1]), 10, 64)
	if want := int64(4102444800500) - time.Now().UnixMilli(); ms < want-1000 || ms > want {
		t.Fatalf("HPTTL %d, want about %d", ms, want)
	}
	r.want(errReply("ERR Number of fields must be a positive integer"), "HTTL", "h", "FIELDS", "0", "a")
	r.want(errReply("ERR The `numfields` parameter must match the number of arguments"), "HTTL", "h", "FIELDS", "2", "a")
	r.want(errReply("ERR Mandatory argument FIELDS is missing or not at the right position"), "HTTL", "h", "FOO", "1", "a")
	r.want(ints(-2, -2), "HTTL", "nokey", "FIELDS", "2", "a", "b")
	r.want("+OK\r\n", "SET", "s", "v")
}

func TestHGetExRedis8Semantics(t *testing.T) {
	r := newHFERun(t)
	r.want(":3\r\n", "HSET", "h", "a", "1", "b", "2", "c", "3")

	reply, prop := r.do("HGETEX", "h", "FIELDS", "2", "a", "z")
	if reply != bulks("1", "<nil>") || len(prop) != 0 {
		t.Fatalf("plain HGETEX: %q, propagated %v", reply, prop)
	}
	reply, prop = r.do("HGETEX", "h", "EX", "100", "FIELDS", "2", "a", "z")
	if reply != bulks("1", "<nil>") || len(prop) != 1 || !strings.HasPrefix(prop[0], "HPEXPIREAT h ") || !strings.HasSuffix(prop[0], " FIELDS 1 a") {
		t.Fatalf("HGETEX EX: %q, propagated %v", reply, prop)
	}
	r.want(ints(100), "HTTL", "h", "FIELDS", "1", "a")
	reply, prop = r.do("HGETEX", "h", "PERSIST", "FIELDS", "2", "a", "b")
	if reply != bulks("1", "2") || fmt.Sprint(prop) != "[HPERSIST h FIELDS 1 a]" {
		t.Fatalf("HGETEX PERSIST: %q, propagated %v", reply, prop)
	}
	r.want(ints(-1), "HTTL", "h", "FIELDS", "1", "a")
	r.want(bulks("1"), "HGETEX", "h", "EXAT", "4102444800", "FIELDS", "1", "a")
	r.want(ints(4102444800), "HEXPIRETIME", "h", "FIELDS", "1", "a")

	reply, prop = r.do("HGETEX", "h", "PX", "0", "FIELDS", "1", "a")
	if reply != bulks("1") || fmt.Sprint(prop) != "[HDEL h a]" {
		t.Fatalf("HGETEX PX 0: %q, propagated %v", reply, prop)
	}
	r.want(bulks("<nil>"), "HGETEX", "h", "FIELDS", "1", "a")
	r.want(bulks("2", "3"), "HGETEX", "h", "EXAT", "1", "FIELDS", "2", "b", "c")
	r.want(":0\r\n", "EXISTS", "h")
	r.want(bulks("<nil>", "<nil>"), "HGETEX", "nokey", "EX", "10", "FIELDS", "2", "a", "b")

	r.want(":1\r\n", "HSET", "h", "a", "1")
	for _, c := range []struct {
		want string
		args []string
	}{
		{"ERR Mandatory argument FIELDS is missing or not at the right position", []string{"PX", "100", "EX", "5", "FIELDS", "1", "a"}},
		{"ERR Mandatory argument FIELDS is missing or not at the right position", []string{"FOO", "1", "a"}},
		{"ERR Mandatory argument FIELDS is missing or not at the right position", []string{"KEEPTTL", "FIELDS", "1", "a"}},
		{"ERR invalid expire time, must be >= 0", []string{"EX", "-1", "FIELDS", "1", "a"}},
		{"ERR value is not an integer or out of range", []string{"EX", "abc", "FIELDS", "1", "a"}},
		{"ERR value is not an integer or out of range", []string{"EX", "FIELDS", "1", "a"}},
		{"ERR invalid expire time in 'hgetex' command", []string{"EX", "9999999999999", "FIELDS", "1", "a"}},
		{"ERR invalid expire time in 'hgetex' command", []string{"PXAT", "70368744177664", "FIELDS", "1", "a"}},
		{"ERR Number of fields must be a positive integer", []string{"FIELDS", "0", "a"}},
		{"ERR Number of fields must be a positive integer", []string{"FIELDS", "x", "a"}},
		{"ERR The `numfields` parameter must match the number of arguments", []string{"FIELDS", "2", "a"}},
		{"ERR The `numfields` parameter must match the number of arguments", []string{"FIELDS", "1", "a", "EX", "10"}},
		{"ERR wrong number of arguments for 'hgetex' command", []string{"EX", "10", "FIELDS"}},
	} {
		r.want(errReply(c.want), append([]string{"HGETEX", "h"}, c.args...)...)
	}
	r.want("+OK\r\n", "SET", "s", "v")
	r.want(errReply("WRONGTYPE Operation against a key holding the wrong kind of value"), "HGETEX", "s", "EX", "abc", "FIELDS", "1", "a")
}

func TestHSetExRedis8Semantics(t *testing.T) {
	r := newHFERun(t)
	r.want(":1\r\n", "HSETEX", "h", "FIELDS", "1", "a", "10")
	reply, prop := r.do("HSETEX", "h", "EX", "100", "FIELDS", "2", "a", "11", "d", "4")
	if reply != ":1\r\n" || len(prop) != 1 || !strings.HasPrefix(prop[0], "HSETEX h PXAT ") || !strings.HasSuffix(prop[0], " FIELDS 2 a 11 d 4") {
		t.Fatalf("HSETEX EX: %q, propagated %v", reply, prop)
	}
	r.want(ints(100, 100, -2), "HTTL", "h", "FIELDS", "3", "a", "d", "b")
	reply, prop = r.do("HSETEX", "h", "FNX", "FIELDS", "2", "a", "1", "x", "1")
	if reply != ":0\r\n" || len(prop) != 0 {
		t.Fatalf("failed FNX: %q, propagated %v", reply, prop)
	}
	r.want(":1\r\n", "HSETEX", "h", "FNX", "FIELDS", "2", "x", "1", "y", "2")
	r.want(":0\r\n", "HSETEX", "h", "FXX", "FIELDS", "2", "x", "1", "zz", "2")
	reply, prop = r.do("HSETEX", "h", "FXX", "KEEPTTL", "FIELDS", "1", "a", "12")
	if reply != ":1\r\n" || fmt.Sprint(prop) != "[HSETEX h KEEPTTL FIELDS 1 a 12]" {
		t.Fatalf("KEEPTTL: %q, propagated %v", reply, prop)
	}
	r.want(ints(100), "HTTL", "h", "FIELDS", "1", "a")
	r.want(":1\r\n", "HSETEX", "h", "FIELDS", "1", "a", "13")
	r.want(ints(-1), "HTTL", "h", "FIELDS", "1", "a")
	r.want(":1\r\n", "HSETEX", "h", "FIELDS", "2", "q", "1", "q", "2")
	r.want("$1\r\n2\r\n", "HGET", "h", "q")

	reply, prop = r.do("HSETEX", "h", "EXAT", "1", "FIELDS", "1", "q", "1")
	if reply != ":1\r\n" || fmt.Sprint(prop) != "[HDEL h q]" {
		t.Fatalf("past EXAT: %q, propagated %v", reply, prop)
	}
	r.want(":0\r\n", "HEXISTS", "h", "q")
	r.want(":0\r\n", "HSETEX", "nokey", "FXX", "FIELDS", "1", "q", "1")
	r.want(":0\r\n", "EXISTS", "nokey")
	r.want(":1\r\n", "HSETEX", "nokey2", "EXAT", "1", "FIELDS", "1", "q", "1")
	r.want(":0\r\n", "EXISTS", "nokey2")
	r.want(":1\r\n", "HSETEX", "h3", "fnx", "ex", "10", "fields", "1", "lq", "1")
	r.want(ints(10), "HTTL", "h3", "FIELDS", "1", "lq")

	for _, c := range []struct {
		want string
		args []string
	}{
		{"ERR Only one of FXX or FNX arguments can be specified", []string{"FNX", "FXX", "FIELDS", "1", "q", "1"}},
		{"ERR Only one of EX, PX, EXAT, PXAT or KEEPTTL arguments can be specified", []string{"EX", "10", "PX", "10", "FIELDS", "1", "q", "1"}},
		{"ERR Only one of EX, PX, EXAT, PXAT or KEEPTTL arguments can be specified", []string{"KEEPTTL", "KEEPTTL", "FIELDS", "1", "q", "1"}},
		{"ERR invalid expire time, must be >= 0", []string{"EX", "-1", "FIELDS", "1", "q", "1"}},
		{"ERR invalid expire time in 'hsetex' command", []string{"EX", "9999999999999", "FIELDS", "1", "a", "1"}},
		{"ERR unknown argument: PERSIST", []string{"PERSIST", "FIELDS", "1", "q", "1"}},
		{"ERR unknown argument: FOO", []string{"EX", "10", "FOO", "FIELDS", "1", "q", "1"}},
		{"ERR unknown argument: a", []string{"EX", "10", "a", "b", "c"}},
		{"ERR invalid number of fields", []string{"FIELDS", "0", "q", "1"}},
		{"ERR invalid number of fields", []string{"FIELDS", "abc", "q", "1"}},
		{"ERR value is not an integer or out of range", []string{"FNX", "EX", "FIELDS", "1", "a", "1"}},
		{"ERR wrong number of arguments for 'hsetex' command", []string{"FIELDS", "2", "q", "1"}},
		{"ERR wrong number of arguments for 'hsetex' command", []string{"FIELDS", "1", "q", "1", "r"}},
		{"ERR wrong number of arguments for 'hsetex' command", []string{"FIELDS", "1", "q", "1", "EX", "10"}},
		{"ERR wrong number of arguments for 'hsetex' command", []string{"EX", "10", "FIELDS", "1"}},
	} {
		r.want(errReply(c.want), append([]string{"HSETEX", "h"}, c.args...)...)
	}
	r.want("+OK\r\n", "SET", "s", "v")
	r.want(errReply("WRONGTYPE Operation against a key holding the wrong kind of value"), "HSETEX", "s", "FIELDS", "1", "q", "1")
	r.want(errReply("ERR value is not an integer or out of range"), "HSETEX", "s", "EX", "abc", "FIELDS", "1", "q", "1")
}

func TestHashFieldTTLEncoding(t *testing.T) {
	r := newHFERun(t)
	r.want(":1\r\n", "HSET", "h", "a", "1")
	if got := getEncoding(mustHash(t, r.s, "h")); got != "listpack" {
		t.Fatalf("encoding %q", got)
	}
	r.want(ints(1), "HEXPIRE", "h", "100", "FIELDS", "1", "a")
	if got := getEncoding(mustHash(t, r.s, "h")); got != "listpackex" {
		t.Fatalf("encoding with a field TTL %q, want listpackex", got)
	}
	r.want(ints(1), "HPERSIST", "h", "FIELDS", "1", "a")
	if got := getEncoding(mustHash(t, r.s, "h")); got != "listpackex" {
		t.Fatalf("encoding after HPERSIST %q, want listpackex as in Redis", got)
	}
}

func mustHash(t *testing.T, s *store.Store, key string) *store.HashValue {
	t.Helper()
	e, ok := s.Get(key)
	if !ok {
		t.Fatalf("%s missing", key)
	}
	return e.Value.(*store.HashValue)
}

// On a writable replica a read hides expired fields behind a copy of the
// hash; a write must still change the stored hash, and treat the expired
// fields, which only the master deletes, as absent.
func TestReplicaHashWritesChangeTheStoredHash(t *testing.T) {
	setup := func(t *testing.T) (*hfeRun, *store.Databases) {
		r := newHFERun(t)
		dbs := store.NewDatabases(r.s, 1)
		r.s = dbs.DB(0)
		r.want(":3\r\n", "HSET", "h", "gone", "x", "live", "y", "kept", "z")
		r.want(ints(1), "HPEXPIREAT", "h", strconv.FormatInt(time.Now().UnixMilli()+20, 10), "FIELDS", "1", "gone")
		r.want(ints(1), "HEXPIRE", "h", "1000", "FIELDS", "1", "kept")
		time.Sleep(40 * time.Millisecond)
		dbs.SetPassiveExpiry(true)
		return r, dbs
	}
	cases := []struct {
		args  []string
		reply string
		check [][2]string
	}{
		{[]string{"HDEL", "h", "live", "gone"}, ":1\r\n", [][2]string{{"HEXISTS h live", ":0\r\n"}, {"HLEN h", ":1\r\n"}}},
		{[]string{"HGETDEL", "h", "FIELDS", "2", "live", "gone"}, bulks("y", "<nil>"), [][2]string{{"HEXISTS h live", ":0\r\n"}}},
		{[]string{"HPERSIST", "h", "FIELDS", "2", "kept", "gone"}, ints(1, -2), [][2]string{{"HTTL h FIELDS 1 kept", ints(-1)}}},
		{[]string{"HEXPIRE", "h", "5000", "FIELDS", "2", "live", "gone"}, ints(1, -2), [][2]string{{"HTTL h FIELDS 1 live", ints(5000)}}},
		{[]string{"HGETEX", "h", "PERSIST", "FIELDS", "2", "kept", "gone"}, bulks("z", "<nil>"), [][2]string{{"HTTL h FIELDS 1 kept", ints(-1)}}},
		{[]string{"HSET", "h", "gone", "v"}, ":1\r\n", [][2]string{{"HTTL h FIELDS 1 gone", ints(-1)}}},
		{[]string{"HSETNX", "h", "gone", "v"}, ":1\r\n", [][2]string{{"HGET h gone", "$1\r\nv\r\n"}}},
		{[]string{"HINCRBY", "h", "gone", "5"}, ":5\r\n", [][2]string{{"HTTL h FIELDS 1 gone", ints(-1)}, {"HLEN h", ":3\r\n"}}},
		{[]string{"HSETEX", "h", "FNX", "EX", "100", "FIELDS", "1", "gone", "v"}, ":1\r\n", [][2]string{{"HGET h gone", "$1\r\nv\r\n"}}},
	}
	for _, c := range cases {
		t.Run(c.args[0], func(t *testing.T) {
			r, _ := setup(t)
			r.want(c.reply, c.args...)
			for _, ch := range c.check {
				r.want(ch[1], strings.Fields(ch[0])...)
			}
		})
	}
}

// The master's own HDEL of a field the replica holds past its expiry still
// deletes it.
func TestReplicaAppliesTheMastersHDELOfAnExpiredField(t *testing.T) {
	r := newHFERun(t)
	dbs := store.NewDatabases(r.s, 1)
	r.s = dbs.DB(0)
	r.want(":2\r\n", "HSET", "h", "gone", "x", "live", "y")
	r.want(ints(1), "HPEXPIREAT", "h", strconv.FormatInt(time.Now().UnixMilli()+20, 10), "FIELDS", "1", "gone")
	time.Sleep(40 * time.Millisecond)
	dbs.SetPassiveExpiry(true)
	dbs.BeginAuthoritativeApply()
	r.want(":1\r\n", "HDEL", "h", "gone")
	dbs.EndAuthoritativeApply()
	entry, ok := r.s.GetForWrite("h")
	if !ok {
		t.Fatal("hash missing")
	}
	h := entry.Value.(*store.HashValue)
	h.RLock()
	n := len(h.Fields)
	h.RUnlock()
	if n != 1 {
		t.Fatalf("stored fields %d, want 1", n)
	}
}
