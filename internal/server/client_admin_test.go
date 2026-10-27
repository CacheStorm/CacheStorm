package server

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
)

func clientLines(t *testing.T, v *resp.Value) []map[string]string {
	t.Helper()
	if v.Type != resp.TypeBulkString {
		t.Fatalf("CLIENT LIST = %+v", v)
	}
	var out []map[string]string
	for _, line := range strings.Split(strings.TrimSpace(string(v.Bulk)), "\n") {
		if line == "" {
			continue
		}
		fields := map[string]string{}
		for _, kv := range strings.Fields(line) {
			k, val, _ := strings.Cut(kv, "=")
			fields[k] = val
		}
		out = append(out, fields)
	}
	return out
}

func TestClientListAndInfo(t *testing.T) {
	s := startReplServer(t, nil)
	a := dialRepl(t, s)
	b := dialRepl(t, s)
	a.do("CLIENT", "SETNAME", "worker-a")
	a.do("CLIENT", "SETINFO", "LIB-NAME", "go-redis")
	a.do("SELECT", "3")
	b.do("SUBSCRIBE", "ch")
	lines := clientLines(t, a.do("CLIENT", "LIST"))
	if len(lines) != 2 {
		t.Fatalf("CLIENT LIST has %d lines, want 2: %v", len(lines), lines)
	}
	var mine, other map[string]string
	aid := strconv.FormatInt(a.do("CLIENT", "ID").Int, 10)
	for _, l := range lines {
		if l["id"] == aid {
			mine = l
		} else {
			other = l
		}
	}
	if mine == nil || mine["name"] != "worker-a" || mine["db"] != "3" || mine["lib-name"] != "go-redis" || mine["cmd"] != "client|list" {
		t.Fatalf("own CLIENT LIST line = %v", mine)
	}
	if other == nil || other["flags"] != "P" || other["sub"] != "1" {
		t.Fatalf("subscriber CLIENT LIST line = %v", other)
	}
	if got := clientLines(t, a.do("CLIENT", "LIST", "TYPE", "pubsub")); len(got) != 1 || got[0]["id"] == aid {
		t.Fatalf("CLIENT LIST TYPE pubsub = %v", got)
	}
	if got := clientLines(t, a.do("CLIENT", "LIST", "ID", aid)); len(got) != 1 || got[0]["id"] != aid {
		t.Fatalf("CLIENT LIST ID = %v", got)
	}
	if got := clientLines(t, a.do("CLIENT", "INFO")); len(got) != 1 || got[0]["id"] != aid {
		t.Fatalf("CLIENT INFO = %v", got)
	}
}

func TestClientKill(t *testing.T) {
	s := startReplServer(t, nil)
	admin := dialRepl(t, s)
	victim := dialRepl(t, s)
	vid := victim.do("CLIENT", "ID").Int
	if got := admin.do("CLIENT", "KILL", "ID", strconv.FormatInt(vid, 10)); got.Int != 1 {
		t.Fatalf("CLIENT KILL ID = %+v", got)
	}
	if _, ok := victim.next(2 * time.Second); ok {
		t.Fatal("killed client still received a frame")
	}
	other := dialRepl(t, s)
	addr := clientLines(t, other.do("CLIENT", "INFO"))[0]["addr"]
	if got := admin.do("CLIENT", "KILL", addr); got.Type != resp.TypeSimpleString {
		t.Fatalf("CLIENT KILL addr = %+v", got)
	}
	if got := admin.do("CLIENT", "KILL", "127.0.0.1:1"); got.Type != resp.TypeError {
		t.Fatalf("CLIENT KILL of a missing address = %+v", got)
	}
	// SKIPME defaults to yes: TYPE normal does not close the caller.
	dialRepl(t, s).do("PING")
	if got := admin.do("CLIENT", "KILL", "TYPE", "normal"); got.Int < 1 {
		t.Fatalf("CLIENT KILL TYPE normal = %+v", got)
	}
	if got := admin.do("PING"); got.Str != "PONG" {
		t.Fatalf("caller was closed by its own KILL TYPE normal: %+v", got)
	}
}

func TestClientPause(t *testing.T) {
	s := startReplServer(t, nil)
	admin := dialRepl(t, s)
	w := dialRepl(t, s)
	if got := admin.do("CLIENT", "PAUSE", "300", "WRITE"); got.Type != resp.TypeSimpleString {
		t.Fatalf("CLIENT PAUSE = %+v", got)
	}
	start := time.Now()
	if got := w.do("GET", "k"); got.Type != resp.TypeBulkString {
		t.Fatalf("GET during a WRITE pause = %+v", got)
	}
	if time.Since(start) > 150*time.Millisecond {
		t.Fatal("a read waited for a WRITE pause")
	}
	w.do("SET", "k", "v")
	if waited := time.Since(start); waited < 250*time.Millisecond {
		t.Fatalf("SET ran %v into a 300ms WRITE pause", waited)
	}
	admin.do("CLIENT", "PAUSE", "10000")
	unpaused := make(chan struct{})
	go func() {
		defer close(unpaused)
		time.Sleep(200 * time.Millisecond)
		dialRepl(t, s).do("CLIENT", "UNPAUSE")
	}()
	defer func() { <-unpaused }()
	start = time.Now()
	if got := w.do("GET", "k"); string(got.Bulk) != "v" {
		t.Fatalf("GET after UNPAUSE = %+v", got)
	}
	if waited := time.Since(start); waited < 150*time.Millisecond || waited > 5*time.Second {
		t.Fatalf("GET during an ALL pause returned after %v", waited)
	}
}

func TestClientUnblock(t *testing.T) {
	s := startReplServer(t, nil)
	admin := dialRepl(t, s)
	b := dialRepl(t, s)
	bid := strconv.FormatInt(b.do("CLIENT", "ID").Int, 10)
	if got := admin.do("CLIENT", "UNBLOCK", bid); got.Int != 0 {
		t.Fatalf("CLIENT UNBLOCK of an idle client = %+v", got)
	}
	done := make(chan *resp.Value, 1)
	go func() { done <- b.do("BLPOP", "nolist", "0") }()
	eventually(t, 3*time.Second, "client blocked", func() bool {
		return strings.Contains(string(admin.do("CLIENT", "LIST", "ID", bid).Bulk), "flags=b")
	})
	if got := admin.do("CLIENT", "UNBLOCK", bid); got.Int != 1 {
		t.Fatalf("CLIENT UNBLOCK = %+v", got)
	}
	select {
	case got := <-done:
		if !got.IsNull {
			t.Fatalf("BLPOP unblocked with TIMEOUT = %+v, want null", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("BLPOP was not unblocked")
	}
	go func() { done <- b.do("BLPOP", "nolist", "0") }()
	eventually(t, 3*time.Second, "client blocked again", func() bool {
		return strings.Contains(string(admin.do("CLIENT", "LIST", "ID", bid).Bulk), "flags=b")
	})
	admin.do("CLIENT", "UNBLOCK", bid, "ERROR")
	select {
	case got := <-done:
		if got.Type != resp.TypeError || !strings.HasPrefix(got.Err, "UNBLOCKED") {
			t.Fatalf("BLPOP unblocked with ERROR = %+v", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("BLPOP was not unblocked with ERROR")
	}
	b.do("RPUSH", "l", "x")
	if got := b.do("BLPOP", "l", "1"); len(got.Array) != 2 {
		t.Fatalf("BLPOP after unblocking = %+v", got)
	}
}

func TestClientReply(t *testing.T) {
	s := startReplServer(t, nil)
	c := dialRepl(t, s)
	if _, err := c.conn.Write(encodeArgs("CLIENT", "REPLY", "OFF")); err != nil {
		t.Fatal(err)
	}
	if _, err := c.conn.Write(encodeArgs("SET", "a", "1")); err != nil {
		t.Fatal(err)
	}
	if got := c.do("CLIENT", "REPLY", "ON"); got.Str != "OK" {
		t.Fatalf("first reply after CLIENT REPLY OFF/ON = %+v, want the ON reply", got)
	}
	if _, err := c.conn.Write(encodeArgs("CLIENT", "REPLY", "SKIP")); err != nil {
		t.Fatal(err)
	}
	if _, err := c.conn.Write(encodeArgs("GET", "a")); err != nil {
		t.Fatal(err)
	}
	if got := c.do("INCR", "n"); got.Int != 1 {
		t.Fatalf("reply after SKIP = %+v, want INCR's 1", got)
	}
	if got := c.do("GET", "a"); string(got.Bulk) != "1" {
		t.Fatalf("SET under CLIENT REPLY OFF was not applied: %+v", got)
	}
}

func TestLatencyMonitor(t *testing.T) {
	s := startReplServer(t, nil)
	c := dialRepl(t, s)
	c.do("LATENCY", "RESET")
	if got := c.do("LATENCY", "LATEST"); len(got.Array) != 0 {
		t.Fatalf("LATENCY LATEST with monitoring off = %+v, want empty", got)
	}
	if got := string(c.do("LATENCY", "DOCTOR").Bulk); !strings.Contains(got, "disabled") {
		t.Fatalf("LATENCY DOCTOR = %q", got)
	}
	c.do("CONFIG", "SET", "latency-monitor-threshold", "50")
	defer c.do("CONFIG", "SET", "latency-monitor-threshold", "0")
	c.do("EVAL", "local t = redis.call('TIME') local s = tonumber(t[1]) * 1000000 + tonumber(t[2]) repeat t = redis.call('TIME') until tonumber(t[1]) * 1000000 + tonumber(t[2]) - s > 80000 return 1", "0")
	latest := c.do("LATENCY", "LATEST")
	if len(latest.Array) != 1 || string(latest.Array[0].Array[0].Bulk) != "command" || latest.Array[0].Array[2].Int < 50 {
		t.Fatalf("LATENCY LATEST after a slow command = %+v", latest)
	}
	if got := c.do("LATENCY", "HISTORY", "command"); len(got.Array) != 1 || got.Array[0].Array[1].Int < 50 {
		t.Fatalf("LATENCY HISTORY = %+v", got)
	}
	if got := c.do("LATENCY", "RESET"); got.Int != 1 {
		t.Fatalf("LATENCY RESET = %+v", got)
	}
}

func TestMonitorStreamsCommands(t *testing.T) {
	s := startReplServer(t, nil)
	m := dialRepl(t, s)
	if got := m.do("MONITOR"); got.Str != "OK" {
		t.Fatalf("MONITOR = %+v", got)
	}
	c := dialRepl(t, s)
	c.do("SELECT", "2")
	c.do("SET", "k", "a b\n")
	c.do("AUTH", "secret")
	c.do("GET", "k")
	var lines []string
	for len(lines) < 3 {
		v, ok := m.next(3 * time.Second)
		if !ok {
			t.Fatalf("MONITOR stopped after %v", lines)
		}
		lines = append(lines, v.Str)
	}
	if !strings.HasSuffix(lines[0], `"SELECT" "2"`) {
		t.Fatalf("first MONITOR line = %q", lines[0])
	}
	if !strings.Contains(lines[1], "[2 ") || !strings.HasSuffix(lines[1], `"SET" "k" "a b\n"`) {
		t.Fatalf("SET MONITOR line = %q", lines[1])
	}
	if !strings.HasSuffix(lines[2], `"GET" "k"`) {
		t.Fatalf("AUTH must not be shown; third line = %q", lines[2])
	}
}

func TestACLLogRecordsDenials(t *testing.T) {
	s := startReplServer(t, nil)
	admin := dialRepl(t, s)
	admin.do("ACL", "LOG", "RESET")
	admin.do("ACL", "SETUSER", "limited", "on", "on", ">pw", "-@all", "+get", "~user:*")
	u := dialRepl(t, s)
	u.do("AUTH", "limited", "pw")
	if got := u.do("SET", "user:1", "x"); got.Type != resp.TypeError || got.Err != "NOPERM User limited has no permissions to run the 'set' command" {
		t.Fatalf("denied command = %+v", got)
	}
	u.do("GET", "other:1")
	u.do("GET", "other:1")
	u.do("AUTH", "limited", "wrong")
	log := admin.do("ACL", "LOG")
	if len(log.Array) != 3 {
		t.Fatalf("ACL LOG has %d entries, want 3: %+v", len(log.Array), log)
	}
	entry := func(i int) map[string]*resp.Value {
		out := map[string]*resp.Value{}
		a := log.Array[i].Array
		for j := 0; j+1 < len(a); j += 2 {
			out[string(a[j].Bulk)] = a[j+1]
		}
		return out
	}
	auth, key, cmd := entry(0), entry(1), entry(2)
	if string(auth["reason"].Bulk) != "auth" || string(auth["username"].Bulk) != "limited" {
		t.Fatalf("auth entry = %v", auth)
	}
	if string(key["reason"].Bulk) != "key" || string(key["object"].Bulk) != "other:1" || key["count"].Int != 2 {
		t.Fatalf("key entry = %v", key)
	}
	if string(cmd["reason"].Bulk) != "command" || string(cmd["object"].Bulk) != "set" || string(cmd["context"].Bulk) != "toplevel" {
		t.Fatalf("command entry = %v", cmd)
	}
	if got := admin.do("ACL", "LOG", "1"); len(got.Array) != 1 {
		t.Fatalf("ACL LOG 1 = %+v", got)
	}
	admin.do("ACL", "LOG", "RESET")
	if got := admin.do("ACL", "LOG"); len(got.Array) != 0 {
		t.Fatalf("ACL LOG after RESET = %+v", got)
	}
}

// A command or key a script is refused is logged with the "lua" context, as
// in Redis.
func TestACLLogRecordsScriptDenials(t *testing.T) {
	s := startReplServer(t, nil)
	admin := dialRepl(t, s)
	admin.do("ACL", "LOG", "RESET")
	admin.do("ACL", "SETUSER", "scripter", "on", "on", ">pw", "-@all", "+eval", "+get", "~user:*")
	u := dialRepl(t, s)
	u.do("AUTH", "scripter", "pw")
	if got := u.do("EVAL", "return redis.call('set', 'user:1', 'x')", "0"); got.Type != resp.TypeError {
		t.Fatalf("script SET = %+v, want an error", got)
	}
	if got := u.do("EVAL", "return redis.call('get', 'other:1')", "0"); got.Type != resp.TypeError {
		t.Fatalf("script GET of a denied key = %+v, want an error", got)
	}
	log := admin.do("ACL", "LOG")
	if len(log.Array) != 2 {
		t.Fatalf("ACL LOG has %d entries, want 2: %+v", len(log.Array), log)
	}
	fields := func(v *resp.Value) map[string]string {
		out := map[string]string{}
		for j := 0; j+1 < len(v.Array); j += 2 {
			out[string(v.Array[j].Bulk)] = string(v.Array[j+1].Bulk)
		}
		return out
	}
	key, cmd := fields(log.Array[0]), fields(log.Array[1])
	if key["reason"] != "key" || key["object"] != "other:1" || key["context"] != "lua" {
		t.Fatalf("script key entry = %v", key)
	}
	if cmd["reason"] != "command" || cmd["object"] != "set" || cmd["context"] != "lua" || cmd["username"] != "scripter" {
		t.Fatalf("script command entry = %v", cmd)
	}
}

func TestSearchAliasesAndTagValues(t *testing.T) {
	s := startReplServer(t, nil)
	c := dialRepl(t, s)
	c.do("FT.CREATE", "aliasidx", "SCHEMA", "tags", "TAG")
	c.do("FT.ADD", "aliasidx", "d1", "1.0", "FIELDS", "tags", "Red, blue")
	c.do("FT.ADD", "aliasidx", "d2", "1.0", "FIELDS", "tags", "green,red")
	if got := c.do("FT.ALIASADD", "aliasname", "aliasidx"); got.Str != "OK" {
		t.Fatalf("FT.ALIASADD = %+v", got)
	}
	if got := c.do("FT.ALIASADD", "aliasname", "aliasidx"); got.Type != resp.TypeError {
		t.Fatalf("duplicate FT.ALIASADD = %+v", got)
	}
	got := c.do("FT.TAGVALS", "aliasname", "tags")
	if len(got.Array) != 3 || string(got.Array[0].Bulk) != "blue" || string(got.Array[2].Bulk) != "red" {
		t.Fatalf("FT.TAGVALS through the alias = %+v", got)
	}
	if got := c.do("FT.ALIASDEL", "aliasname"); got.Str != "OK" {
		t.Fatalf("FT.ALIASDEL = %+v", got)
	}
	if got := c.do("FT.TAGVALS", "aliasname", "tags"); got.Type != resp.TypeError {
		t.Fatalf("FT.TAGVALS after FT.ALIASDEL = %+v", got)
	}
	c.do("FT.DROPINDEX", "aliasidx")
}

func TestRuntimeStatsAreReal(t *testing.T) {
	c := dialRepl(t, startReplServer(t, nil))
	stats := c.do("HEAP.STATS")
	if len(stats.Array) < 2 || stats.Array[1].Int == 0 {
		t.Fatalf("HEAP.STATS = %+v, want real allocation counts", stats)
	}
	if got := c.do("MEMORY", "PURGE"); got.Str != "OK" {
		t.Fatalf("MEMORY PURGE = %+v", got)
	}
}

func TestACLCategoriesAndCommandIntrospection(t *testing.T) {
	s := startReplServer(t, nil)
	admin := dialRepl(t, s)
	if got := admin.do("ACL", "SETUSER", "reader", "on", "on", ">pw", "~*", "-@all", "+@read", "+@connection"); got.Str != "OK" {
		t.Fatalf("ACL SETUSER with categories = %+v", got)
	}
	if got := admin.do("ACL", "SETUSER", "x", "on", "+@bogus"); got.Type != resp.TypeError {
		t.Fatalf("unknown category = %+v", got)
	}
	admin.do("SET", "k", "v")
	r := dialRepl(t, s)
	r.do("AUTH", "reader", "pw")
	if got := r.str("GET", "k"); got != "v" {
		t.Fatalf("GET under +@read = %q", got)
	}
	if got := r.do("SET", "k", "w"); got.Type != resp.TypeError || !strings.HasPrefix(got.Err, "NOPERM") {
		t.Fatalf("SET under -@all +@read = %+v", got)
	}
	if got := r.do("CONFIG", "GET", "maxmemory"); got.Type != resp.TypeError || !strings.Contains(got.Err, "config|get") {
		t.Fatalf("CONFIG GET under +@read = %+v", got)
	}

	info := admin.do("COMMAND", "INFO", "get", "nosuchcommand")
	if len(info.Array) != 2 || !info.Array[1].IsNull {
		t.Fatalf("COMMAND INFO = %+v", info)
	}
	get := info.Array[0].Array
	if string(get[0].Bulk) != "get" || get[1].Int != 2 || get[3].Int != 1 || len(get[6].Array) == 0 {
		t.Fatalf("COMMAND INFO get = %+v", get)
	}
	count := admin.do("COMMAND", "COUNT").Int
	if all := admin.do("COMMAND"); int64(len(all.Array)) != count || count < 1150 {
		t.Fatalf("COMMAND has %d entries, COMMAND COUNT says %d", len(all.Array), count)
	}
	if got := admin.do("COMMAND", "LIST", "FILTERBY", "ACLCAT", "hyperloglog"); len(got.Array) < 3 {
		t.Fatalf("COMMAND LIST FILTERBY ACLCAT hyperloglog = %+v", got)
	}
	if got := admin.do("ACL", "CAT", "dangerous"); len(got.Array) < 10 {
		t.Fatalf("ACL CAT dangerous = %+v", got)
	}
}
