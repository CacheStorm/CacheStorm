package server

import (
	"io"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/config"
	"github.com/cachestorm/cachestorm/internal/resp"
)

// drainEvents returns the keyspace events a PSUBSCRIBEd client has received
// so far: it sends PING and collects the messages ahead of the reply.
func drainEvents(t *testing.T, sub *replClient) []string {
	t.Helper()
	if _, err := sub.conn.Write(encodeArgs("PING")); err != nil {
		t.Fatal(err)
	}
	var out []string
	for {
		v, ok := sub.next(3 * time.Second)
		if !ok {
			t.Fatalf("no PING reply after events %v", out)
		}
		if len(v.Array) == 2 && string(v.Array[0].Bulk) == "pong" {
			return out
		}
		if len(v.Array) != 4 || string(v.Array[0].Bulk) != "pmessage" {
			t.Fatalf("unexpected frame %+v", v)
		}
		ch := string(v.Array[2].Bulk)
		out = append(out, ch[strings.Index(ch, "__:")+3:]+" "+string(v.Array[3].Bulk))
	}
}

// The digests are Redis 8.2.11's for the same data (debug.c's
// xorObjectDigest and computeDatasetDigest, vset.c's VectorSetDigest), so a
// CacheStorm master and a Redis replica, or the other way round, can be
// compared with DEBUG DIGEST.
func TestDebugDigestMatchesRedis(t *testing.T) {
	c := dialRepl(t, startReplServer(t, nil))
	for _, cmd := range [][]string{
		{"SET", "s", "v"}, {"SET", "n", "12345"}, {"RPUSH", "l", "a", "b", "1"},
		{"HSET", "h", "f", "v", "g", "2"}, {"ZADD", "z", "1", "a", "2.5", "b", "-inf", "c", "1e300", "d"},
		{"SADD", "st", "x", "y", "3"}, {"SADD", "si", "1", "2", "3"}, {"SET", "e", "v", "PX", "100000"},
		{"XADD", "x", "1-1", "f", "v", "g", "w"}, {"XADD", "x", "2-5", "f", "v2"},
		{"PFADD", "hll", "a", "b", "c"}, {"SETBIT", "bm", "7", "1"},
		{"HSET", "hx", "a", "1", "b", "2"}, {"HEXPIRE", "hx", "1000", "FIELDS", "1", "a"},
	} {
		if got := c.str(cmd...); strings.HasPrefix(got, "ERR") {
			t.Fatalf("%v: %s", cmd, got)
		}
	}
	zbig, hbig := []string{"ZADD", "zbig"}, []string{"HSET", "hbig"}
	for i := 0; i < 200; i++ {
		zbig = append(zbig, strconv.Itoa(i), "m"+strconv.Itoa(i))
		hbig = append(hbig, "f"+strconv.Itoa(i), "v"+strconv.Itoa(i))
	}
	c.str(zbig...)
	c.str(hbig...)
	c.str("SELECT", "3")
	c.str("SET", "d3", "v")
	c.str("SELECT", "0")

	want := []struct{ key, digest string }{
		{"s", "d79ee9fe5d5ad902d7f6620efc3b07b75ddd742f"},
		{"n", "e1cf90774e995ae03de66e8109dc5cb9d9c2dc83"},
		{"l", "981fd62c1655c074205cff40b800bd5a5bfdcd41"},
		{"h", "9dc7e1ed596e8ccab4b91d04b8317fc85b68294a"},
		{"z", "a077f48ec0d4711f3a5cd403ab8d5c7239df9c34"},
		{"st", "415a07507e2348d139f9fd645a024f6be569dd65"},
		{"si", "2a462a3f51a1db89738ebbeef0c47ce1bbc85246"},
		{"e", "311b17018163335616e089f6b951c82c477335ac"},
		{"x", "6333662e9a6bf68f222c7ae64848e53e46a07ccd"},
		{"hll", "f579608d33b88f5e9f7172d6fb480ecf5d13ddf5"},
		{"bm", "979efc88cba7b776b9495fca95d4beb535e96f0e"},
		{"hx", "c07db73128914a41e685572f9d83e5ec946f598d"},
		{"zbig", "981002a26dc5c65b35baac65a30f21b91b0a7b44"},
		{"hbig", "0887b9799375ff93095b92928148c2e3d49c5e84"},
		{"nokey", "0000000000000000000000000000000000000000"},
	}
	args := []string{"DEBUG", "DIGEST-VALUE"}
	var digests []string
	for _, w := range want {
		args = append(args, w.key)
		digests = append(digests, w.digest)
	}
	if got, w := c.str(args...), "["+strings.Join(digests, " ")+"]"; got != w {
		t.Fatalf("DIGEST-VALUE\n got %s\nwant %s", got, w)
	}
	if v := c.do("DEBUG", "DIGEST"); v.Type != resp.TypeSimpleString || v.Str != "91fefccca81b72e629e7c62dbfb0581f2cf657f5" {
		t.Fatalf("DEBUG DIGEST = %+v", v)
	}

	// Module types: Redis' JSON and probabilistic modules have no digest
	// callback, so only the type counts; vector sets digest their nodes.
	c.str("FLUSHALL")
	for _, cmd := range [][]string{
		{"JSON.SET", "j", "$", `{"a":1}`}, {"BF.ADD", "b", "x"},
		{"VADD", "v", "VALUES", "2", "1", "2", "e"},
		{"VADD", "v2", "VALUES", "2", "1", "2", "e"}, {"VADD", "v2", "VALUES", "2", "0.5", "3", "f", "SETATTR", `{"a":1}`},
		{"VADD", "v3", "VALUES", "3", "1", "2", "3", "e", "NOQUANT"},
		{"VADD", "v4", "VALUES", "3", "1", "-2", "3", "e", "BIN"}, {"VADD", "v4", "VALUES", "3", "-1", "-2", "3", "f", "BIN"},
		{"VADD", "v5", "REDUCE", "2", "VALUES", "4", "1", "2", "3", "4", "e"},
	} {
		if got := c.str(cmd...); strings.HasPrefix(got, "ERR") {
			t.Fatalf("%v: %s", cmd, got)
		}
	}
	const typeOnly = "865b1024b47cf7b38672487172782cb7502865bb"
	if got, w := c.str("DEBUG", "DIGEST-VALUE", "j", "b", "v", "v2", "v3", "v4", "v5"), "["+strings.Join([]string{
		typeOnly, typeOnly,
		"fc87a8e22f4a0ed99b82793467848b76623b5d8d",
		"478d034b6cf7a28226208090245bcfb6e6c1903f",
		"b0797f71f6c5fc80393b7265ad65cbdbe5378512",
		"6292b269d7e0cc2e52e17104534fe75fdacedd26",
		"eb44a1f52f2152c4ea1990d2ed44a7f31da2af75",
	}, " ")+"]"; got != w {
		t.Fatalf("module DIGEST-VALUE\n got %s\nwant %s", got, w)
	}
	c.str("FLUSHALL")
	if got := c.str("DEBUG", "DIGEST"); got != strings.Repeat("0", 40) {
		t.Fatalf("empty dataset digest = %s", got)
	}
}

// DEBUG's test helpers reply as Redis 8.2's do.
func TestDebugSubcommandsMatchRedis(t *testing.T) {
	s := startReplServer(t, nil)
	c := dialRepl(t, s)
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"DEBUG", "FOO"}, "ERR:ERR unknown subcommand or wrong number of arguments for 'FOO'. Try DEBUG HELP."},
		{[]string{"DEBUG", "sleep"}, "ERR:ERR unknown subcommand or wrong number of arguments for 'sleep'. Try DEBUG HELP."},
		{[]string{"DEBUG", "OBJECT"}, "ERR:ERR unknown subcommand or wrong number of arguments for 'OBJECT'. Try DEBUG HELP."},
		{[]string{"DEBUG", "DIGEST", "x"}, "ERR:ERR unknown subcommand or wrong number of arguments for 'DIGEST'. Try DEBUG HELP."},
		{[]string{"DEBUG", "SLEEP", "abc"}, "OK"},
		{[]string{"DEBUG", "ERROR", "MYERR line\r\nbreak"}, "ERR:MYERR line  break"},
		{[]string{"DEBUG", "LOG", "hello"}, "OK"},
		{[]string{"DEBUG", "STRUCTSIZE"}, "bits:" + strconv.Itoa(strconv.IntSize) + " robj:16 dictentry:24 sdshdr5:1 sdshdr8:3 sdshdr16:5 sdshdr32:9 sdshdr64:17 "},
		{[]string{"DEBUG", "DIGEST-VALUE"}, "[]"},
		{[]string{"DEBUG", "PROTOCOL", "string"}, "Hello World"},
		{[]string{"DEBUG", "PROTOCOL", "integer"}, "12345"},
		{[]string{"DEBUG", "PROTOCOL", "double"}, "3.141"},
		{[]string{"DEBUG", "PROTOCOL", "bignum"}, "1234567999999999999999999999999999999"},
		{[]string{"DEBUG", "PROTOCOL", "null"}, "<null>"},
		{[]string{"DEBUG", "PROTOCOL", "array"}, "[0 1 2]"},
		{[]string{"DEBUG", "PROTOCOL", "set"}, "[0 1 2]"},
		{[]string{"DEBUG", "PROTOCOL", "map"}, "[0 0 1 1 2 0]"},
		{[]string{"DEBUG", "PROTOCOL", "attrib"}, "Some real reply following the attribute"},
		{[]string{"DEBUG", "PROTOCOL", "push"}, "ERR:ERR RESP2 is not supported by this command"},
		{[]string{"DEBUG", "PROTOCOL", "verbatim"}, "This is a verbatim\nstring"},
		{[]string{"DEBUG", "PROTOCOL", "true"}, "1"},
		{[]string{"DEBUG", "PROTOCOL", "false"}, "0"},
		{[]string{"DEBUG", "PROTOCOL", "nope"}, "ERR:ERR Wrong protocol type name. Please use one of the following: string|integer|double|bignum|null|array|set|map|attrib|push|verbatim|true|false"},
	} {
		if got := c.str(tc.args...); got != tc.want {
			t.Fatalf("%v = %q, want %q", tc.args, got, tc.want)
		}
	}
	if v := c.do("DEBUG", "HELP"); len(v.Array) == 0 || v.Array[0].Str != "DEBUG <subcommand> [<arg> [value] [opt] ...]. Subcommands are:" ||
		v.Array[len(v.Array)-1].Str != "    Print this help." {
		t.Fatalf("DEBUG HELP = %s", str(v))
	}

	// RESP3: the push goes out after the reply, as Redis 7.2 and later send
	// the pushes a command makes, and an attribute precedes its reply.
	c.str("HELLO", "3")
	if got := c.str("DEBUG", "PROTOCOL", "push"); got != "Some real reply following the push reply" {
		t.Fatalf("push reply = %q", got)
	}
	if v, ok := c.next(time.Second); !ok || v.Type != resp.TypePush || len(v.Array) != 2 || string(v.Array[0].Bulk) != "server-cpu-usage" || v.Array[1].Int != 42 {
		t.Fatalf("push = %+v", v)
	}
	// The test client has no attribute type: read those bytes raw.
	raw := dialRepl(t, s)
	raw.str("HELLO", "3")
	if _, err := raw.conn.Write(encodeArgs("DEBUG", "PROTOCOL", "attrib")); err != nil {
		t.Fatal(err)
	}
	const attrib = "|1\r\n$14\r\nkey-popularity\r\n*2\r\n$7\r\nkey:123\r\n:90\r\n$39\r\nSome real reply following the attribute\r\n"
	buf := make([]byte, len(attrib))
	_ = raw.conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, err := io.ReadFull(raw.conn, buf); err != nil || string(buf) != attrib {
		t.Fatalf("attrib = %q (%v), want %q", buf, err, attrib)
	}
	if v := c.do("DEBUG", "PROTOCOL", "double"); v.Type != resp.TypeDouble || v.Str != "3.141" {
		t.Fatalf("double = %+v", v)
	}
	if v := c.do("DEBUG", "PROTOCOL", "true"); v.Type != resp.TypeBoolean || v.Int != 1 {
		t.Fatalf("true = %+v", v)
	}
	if got := c.str("PING"); got != "PONG" {
		t.Fatalf("connection out of step: PING = %q", got)
	}
}

// DEBUG SET-ACTIVE-EXPIRE 0 stops the active expiry of keys and hash
// fields in every database; reads still expire what they meet, and 1
// starts it again.
func TestDebugSetActiveExpire(t *testing.T) {
	c := dialRepl(t, startReplServer(t, nil))
	if got := c.str("DEBUG", "SET-ACTIVE-EXPIRE", "0"); got != "OK" {
		t.Fatalf("SET-ACTIVE-EXPIRE 0: %s", got)
	}
	c.str("SET", "a", "v", "PX", "20")
	c.str("SET", "b", "v", "PX", "20")
	c.str("SELECT", "5")
	c.str("SET", "c", "v", "PX", "20")
	c.str("SELECT", "0")
	time.Sleep(400 * time.Millisecond) // four active expiry cycles
	if got := c.str("DBSIZE"); got != "2" {
		t.Fatalf("DBSIZE with active expiry off = %s, want 2", got)
	}
	if got := c.str("GET", "a"); got != "<null>" {
		t.Fatalf("GET of an expired key = %s", got)
	}
	if got := c.str("DBSIZE"); got != "1" {
		t.Fatalf("DBSIZE after the read expired a = %s, want 1", got)
	}
	c.str("SELECT", "5")
	if got := c.str("DBSIZE"); got != "1" {
		t.Fatalf("db 5 DBSIZE with active expiry off = %s, want 1", got)
	}
	c.str("SELECT", "0")
	if got := c.str("DEBUG", "SET-ACTIVE-EXPIRE", "1"); got != "OK" {
		t.Fatalf("SET-ACTIVE-EXPIRE 1: %s", got)
	}
	eventually(t, 5*time.Second, "active expiry to remove b", func() bool { return c.str("DBSIZE") == "0" })
	c.str("SELECT", "5")
	eventually(t, 5*time.Second, "active expiry to remove c", func() bool { return c.str("DBSIZE") == "0" })
}

// SET with an expiry already past stores the key as expired, as Redis'
// setGenericCommand does: it announces "set" and "expire", counts in
// DBSIZE until a read or the active cycle removes it, which announces
// "expired" and propagates the DEL; a replica and an AOF restart agree.
func TestSetWithPastExpiryStoresAnExpiredKey(t *testing.T) {
	dir := t.TempDir()
	cfg := replTestConfig(t, func(c *config.Config) {
		c.Persistence.Enabled = true
		c.Persistence.DataDir = dir
		c.Persistence.AOF = true
		c.Persistence.AOFSync = "always"
	})
	master := startReplServerCfg(t, cfg)
	replica := startReplServer(t, nil)
	rc := replicate(t, replica, master)
	c := dialRepl(t, master)
	c.str("CONFIG", "SET", "notify-keyspace-events", "KEA")
	c.str("DEBUG", "SET-ACTIVE-EXPIRE", "0")
	sub := dialRepl(t, master)
	sub.do("PSUBSCRIBE", "__keyevent@*")

	for _, opts := range [][]string{{"PXAT", "1"}, {"EXAT", "1"}, {"PXAT", "1", "GET"}} {
		c.str("SET", "old", "v")
		drainEvents(t, sub)
		if got := c.str(append([]string{"SET", "old", "new"}, opts...)...); got != "OK" && got != "v" {
			t.Fatalf("SET %v = %s", opts, got)
		}
		sameEvents(t, drainEvents(t, sub), "set old", "expire old")
		if got := c.str("DBSIZE"); got != "1" {
			t.Fatalf("DBSIZE after SET %v = %s, want the expired key counted", opts, got)
		}
		if got := c.str("EXISTS", "old"); got != "0" {
			t.Fatalf("EXISTS = %s", got)
		}
		sameEvents(t, drainEvents(t, sub), "expired old")
	}
	// NX and XX see the expired key as missing.
	c.str("SET", "k", "a", "PXAT", "1")
	if got := c.str("SET", "k", "b", "NX"); got != "OK" {
		t.Fatalf("SET NX over an expired key = %s", got)
	}
	c.str("SET", "x", "a", "PXAT", "1")
	if got := c.str("SET", "x", "b", "XX"); got != "<null>" {
		t.Fatalf("SET XX over an expired key = %s", got)
	}
	c.str("SET", "gone", "v", "PXAT", "1")
	c.str("SET", "kept", "v")
	waitSynced(t, replica, master)
	if got := rc.str("EXISTS", "gone"); got != "0" {
		t.Fatalf("replica EXISTS gone = %s", got)
	}
	if got := rc.str("GET", "k"); got != "b" {
		t.Fatalf("replica k = %s", got)
	}

	stopReplServer(master)
	restarted := startReplServerCfg(t, cfg)
	rc2 := dialRepl(t, restarted)
	for key, want := range map[string]string{"gone": "<null>", "old": "<null>", "x": "<null>", "k": "b", "kept": "v"} {
		if got := rc2.str("GET", key); got != want {
			t.Fatalf("after AOF restart %s = %s, want %s", key, got, want)
		}
	}
}

// A write to a watched key looks at what it stored without expiring it: an
// "expired" event belongs to the read that expires the key, as in Redis,
// not to the bookkeeping WATCH does around the write.
func TestWatchedWritesDoNotExpireKeys(t *testing.T) {
	s := startReplServer(t, nil)
	c, w, sub := dialRepl(t, s), dialRepl(t, s), dialRepl(t, s)
	c.str("CONFIG", "SET", "notify-keyspace-events", "KEA")
	c.str("DEBUG", "SET-ACTIVE-EXPIRE", "0")
	sub.do("PSUBSCRIBE", "__keyevent@*")
	w.str("WATCH", "k")
	c.str("SET", "k", "v", "PXAT", "1")
	sameEvents(t, drainEvents(t, sub), "set k", "expire k")
	w.str("MULTI")
	w.str("PING")
	if got := w.str("EXEC"); got != "<nullarray>" {
		t.Fatalf("EXEC after a write to the watched key = %s", got)
	}
}

// The commands of a transaction and of a script run at one command time,
// as Redis' command time snapshot: a relative expiry set late in the
// transaction counts from where an earlier one did.
func TestTransactionCommandTimeStandsStill(t *testing.T) {
	c := dialRepl(t, startReplServer(t, nil))
	c.str("MULTI")
	for _, cmd := range [][]string{
		{"SET", "a", "x", "PX", "100000"},
		{"DEBUG", "SLEEP", "0.005"},
		{"SET", "b", "x", "PX", "100000"},
		{"PSETEX", "c", "100000", "x"},
		{"SET", "d", "x"},
		{"PEXPIRE", "d", "100000"},
		{"GETEX", "a", "PX", "100000"},
		{"PEXPIRE", "a", "100000", "GT"},
		{"PEXPIRETIME", "a"},
		{"PEXPIRETIME", "b"},
		{"PEXPIRETIME", "c"},
		{"PEXPIRETIME", "d"},
	} {
		c.str(cmd...)
	}
	v := c.do("EXEC")
	if len(v.Array) != 12 {
		t.Fatalf("EXEC = %s", str(v))
	}
	if got := str(v.Array[7]); got != "0" {
		t.Fatalf("PEXPIRE GT at the same command time = %s, want 0", got)
	}
	at := str(v.Array[8])
	for i := 9; i < 12; i++ {
		if got := str(v.Array[i]); got != at {
			t.Fatalf("expiry times %s", str(v))
		}
	}

	got := c.str("EVAL", `
		redis.call('set', 'e', 'x', 'px', 100000)
		local t = redis.call('time')
		local start = t[1] * 1000000 + t[2]
		repeat
			local u = redis.call('time')
		until (u[1] * 1000000 + u[2]) - start > 5000
		local gt = redis.call('pexpire', 'e', 100000, 'GT')
		redis.call('set', 'f', 'x', 'px', 100000)
		return {gt, redis.call('pexpiretime', 'e') - redis.call('pexpiretime', 'f')}`, "0")
	if got != "[0 0]" {
		t.Fatalf("script: %s, want the same command time throughout", got)
	}
}
