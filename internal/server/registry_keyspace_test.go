package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The registry namespaces of advanced_commands2 are keys: their objects
// survive a restart through an AOF holding only Redis commands.
func TestRegistriesAreKeys(t *testing.T) {
	cfg := aofTxConfig(t)
	s := startToggleServer(t, cfg)
	c := dialRepl(t, s)
	ok := func(args ...string) string {
		t.Helper()
		v := c.do(args...)
		if v.Type == '-' {
			t.Fatalf("%v: %v", args, v.Err)
		}
		return string(v.Bulk)
	}
	ok("VALIDATE.CREATE", "zip", "^[0-9]{5}$")
	ok("VIEW.CREATE", "v", "SELECT 1")
	ok("STAGE.CREATE", "rollout", "3")
	ok("STAGE.NEXT", "rollout")
	ok("STAGE.NEXT", "rollout")
	ok("CONTEXT.CREATE", "req")
	ok("CONTEXT.SET", "req", "trace", "abc")
	ok("CACHEX.CREATE", "c")
	ok("CACHEX.SET", "c", "k", "v")
	ok("STOREX.CREATE", "st")
	ok("STOREX.PUT", "st", "k", "v2")
	rec := ok("RECORD.CREATE", "order")
	ok("RECORD.ADD", rec, "total", "42")
	ok("ENTITY.CREATE", "e1", "user")
	ok("ENTITY.SET", "e1", "name", "ada")
	prof := ok("PROFILE.CREATE", "ada")
	ok("PROFILE.SET", prof, "theme", "dark")
	ok("COUNTERX2.CREATE", "hits", "10")
	ok("COUNTERX2.INCR", "hits", "5")
	ok("COUNTERX2.DECR", "hits")
	ok("LEVEL.CREATE", "vol", "10")
	ok("LEVEL.SET", "vol", "7")
	ok("PERMIT.GRANT", "ada", "docs", "read")
	grant := ok("GRANT.CREATE", "bob", "docs", "read", "write")
	rel := ok("RELATION.CREATE", "e1", "e2", "follows")
	timer := ok("TIMER.CREATE", "t", "3600000")
	tok := ok("TOKEN.CREATE", "ada", "3600000")
	ok("POOLX.CREATE", "p", "3")
	if got := ok("POOLX.ACQUIRE", "p"); got != "resource-0" {
		t.Fatalf("POOLX.ACQUIRE = %q", got)
	}
	ok("POOLX.ACQUIRE", "p")
	ok("POOLX.RELEASE", "p", "resource-0")
	ok("BUFFERX.CREATE", "buf", "16")
	ok("BUFFERX.WRITE", "buf", "hello ")
	ok("BUFFERX.WRITE", "buf", "world")
	ok("STREAMX.CREATE", "sx")
	ok("STREAMX.WRITE", "sx", "e1")
	ok("STREAMX.WRITE", "sx", "e2")
	ok("AUDITX.LOG", "audit", "login", "ada", "web")
	ok("AUDITX.LOG", "audit", "logout", "bob")
	ok("INDEX.CREATE", "ix")
	ok("INDEX.ADD", "ix", "red", "1")
	ok("INDEX.ADD", "ix", "red", "2")
	ok("SET", "plain", "v")
	ok("HSET", "h", "f", "v")
	for _, args := range [][]string{
		{"VALIDATE.CHECK", "plain", "x"}, {"CONTEXT.SET", "h", "a", "b"}, {"COUNTERX2.INCR", "c"}, {"PERMIT.GRANT", "plain", "r", "a"},
		{"POOLX.ACQUIRE", "st"}, {"BUFFERX.WRITE", "h", "x"}, {"STREAMX.WRITE", "audit", "x"}, {"INDEX.ADD", "plain", "k", "1"},
	} {
		if got := c.do(args...); got.Type != '-' || !strings.HasPrefix(got.Err, "WRONGTYPE") {
			t.Fatalf("%v: %v", args, got)
		}
	}
	for _, args := range [][]string{
		{"VALIDATE.CREATE", "bad", "("}, {"LEVEL.SET", "vol", "11"}, {"STAGE.CREATE", "s2", "x"}, {"TOKEN.CREATE", "u", "0"},
		{"CONTEXT.SET", "req", "__context", "x"}, {"COUNTERX2.INCR", "hits", "x"},
	} {
		if got := c.do(args...); got.Type != '-' {
			t.Fatalf("%v accepted: %v", args, got)
		}
	}
	stopReplServer(s)

	data, err := os.ReadFile(filepath.Join(cfg.Persistence.DataDir, "appendonly.aof"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"VALIDATE.", "VIEW.", "STAGE.", "CONTEXT.", "CACHEX.", "STOREX.", "RECORD.", "ENTITY.", "PROFILE.",
		"COUNTERX2.", "LEVEL.", "PERMIT.", "GRANT.", "RELATION.", "TIMER.", "TOKEN.", "POOLX.", "BUFFERX.", "STREAMX.", "AUDITX.", "INDEX."} {
		if strings.Contains(string(data), name) {
			t.Fatalf("the AOF records %s commands", name)
		}
	}

	s = startToggleServer(t, cfg)
	defer stopReplServer(s)
	c = dialRepl(t, s)
	expect := func(want string, args ...string) {
		t.Helper()
		if got := renderReply(c.do(args...), ","); got != want {
			t.Fatalf("%v after restart = %q, want %q", args, got, want)
		}
	}
	expect("1", "VALIDATE.CHECK", "zip", "12345")
	expect("0", "VALIDATE.CHECK", "zip", "1234")
	expect("SELECT 1", "VIEW.GET", "v")
	expect("[name rollout current 2 total 3]", "STAGE.LIST")
	expect("abc", "CONTEXT.GET", "req", "trace")
	expect("v", "CACHEX.GET", "c", "k")
	expect("v2", "STOREX.GET", "st", "k")
	expect("id,"+rec+",name,order,total,42", "RECORD.GET", rec)
	expect("id,e1,type,user,name,ada", "ENTITY.GET", "e1")
	expect("id,"+prof+",user,ada,theme,dark", "PROFILE.GET", prof)
	expect("14", "COUNTERX2.GET", "hits")
	expect("current,7,max,10", "LEVEL.GET", "vol")
	expect("1", "PERMIT.CHECK", "ada", "docs", "read")
	expect("ada:docs:read", "PERMIT.LIST", "ada")
	expect("1", "GRANT.CHECK", "bob", "docs", "write")
	expect("0", "GRANT.CHECK", "bob", "docs", "delete")
	expect("[id "+grant+" user bob resource docs]", "GRANT.LIST")
	expect("id,"+rel+",from,e1,to,e2,type,follows", "RELATION.GET", rel)
	if got := c.do("TIMER.STATUS", timer); len(got.Array) != 6 || string(got.Array[3].Bulk) != "true" {
		t.Fatalf("TIMER.STATUS after restart: %+v", got)
	}
	expect("1", "TOKEN.VALIDATE", tok)
	if got := c.do("PTTL", tok); got.Int <= 0 {
		t.Fatalf("the token lost its TTL: %v", got)
	}
	expect("name,p,size,3,available,2,in_use,1", "POOLX.STATUS", "p")
	expect("resource-0", "POOLX.ACQUIRE", "p")
	expect("hello world", "BUFFERX.READ", "buf")
	expect("e1,e2", "STREAMX.READ", "sx")
	if got := c.do("AUDITX.SEARCH", "audit", "bob"); len(got.Array) != 1 || string(got.Array[0].Array[3].Bulk) != "logout" {
		t.Fatalf("AUDITX.SEARCH after restart: %+v", got)
	}
	expect("1,2", "INDEX.SEARCH", "ix", "red")
	ok("TOKEN.DELETE", tok)
	expect("0", "TOKEN.VALIDATE", tok)
}
