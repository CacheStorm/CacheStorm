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
	go func() {
		time.Sleep(200 * time.Millisecond)
		dialRepl(t, s).do("CLIENT", "UNPAUSE")
	}()
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
