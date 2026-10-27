package server

import (
	"net"
	"strings"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/config"
	"github.com/cachestorm/cachestorm/internal/resp"
)

func configPairs(t *testing.T, v *resp.Value) map[string]string {
	t.Helper()
	if v.Type != resp.TypeArray || len(v.Array)%2 != 0 {
		t.Fatalf("CONFIG GET reply = %+v", v)
	}
	out := make(map[string]string)
	for i := 0; i < len(v.Array); i += 2 {
		out[string(v.Array[i].Bulk)] = string(v.Array[i+1].Bulk)
	}
	return out
}

func TestConfigGetReportsTheRunningConfiguration(t *testing.T) {
	s := startReplServerCfg(t, replTestConfig(t, func(c *config.Config) {
		c.Memory.MaxMemory = "64mb"
		c.Memory.EvictionPolicy = "volatile-ttl"
		c.Server.MaxConnections = 321
	}))
	c := dialRepl(t, s)
	got := configPairs(t, c.do("CONFIG", "GET", "maxmemory", "maxmemory-policy", "maxclients", "port"))
	if got["maxmemory"] != "67108864" || got["maxmemory-policy"] != "volatile-ttl" || got["maxclients"] != "321" {
		t.Fatalf("CONFIG GET = %v", got)
	}
	if got["port"] == "0" || got["port"] == "" {
		t.Fatalf("CONFIG GET port = %q, want the listening port", got["port"])
	}
	if exact := configPairs(t, c.do("CONFIG", "GET", "maxmemory")); len(exact) != 1 {
		t.Fatalf("CONFIG GET maxmemory matched %v, want only maxmemory", exact)
	}
	if glob := configPairs(t, c.do("CONFIG", "GET", "maxmemory*")); len(glob) != 3 {
		t.Fatalf("CONFIG GET maxmemory* matched %v, want the three maxmemory parameters", glob)
	}
}

func TestConfigSetMaxmemoryIsEnforced(t *testing.T) {
	s := startReplServerCfg(t, replTestConfig(t, func(c *config.Config) {
		c.Memory.EvictionPolicy = "noeviction"
	}))
	c := dialRepl(t, s)
	value := strings.Repeat("v", 1024)
	for i := 0; i < 200; i++ {
		if got := c.do("SET", "k"+string(rune('a'+i%26))+strings.Repeat("x", i/26), value); got.Type == resp.TypeError {
			t.Fatalf("SET without a limit = %+v", got)
		}
	}
	if got := c.do("CONFIG", "SET", "maxmemory", "100kb"); got.Type != resp.TypeSimpleString {
		t.Fatalf("CONFIG SET maxmemory = %+v", got)
	}
	if got := c.do("SET", "one-more", value); got.Type != resp.TypeError || !strings.HasPrefix(got.Err, "OOM") {
		t.Fatalf("SET over the runtime limit with noeviction = %+v, want OOM", got)
	}
	if got := c.do("CONFIG", "SET", "maxmemory-policy", "allkeys-lru"); got.Type != resp.TypeSimpleString {
		t.Fatalf("CONFIG SET maxmemory-policy = %+v", got)
	}
	if got := c.do("SET", "one-more", value); got.Type == resp.TypeError {
		t.Fatalf("SET with allkeys-lru after a runtime policy change = %+v", got)
	}
	if n := c.do("DBSIZE").Int; n >= 200 {
		t.Fatalf("DBSIZE = %d, eviction did not run", n)
	}
	if got := c.do("CONFIG", "SET", "maxmemory-policy", "bogus"); got.Type != resp.TypeError {
		t.Fatalf("CONFIG SET maxmemory-policy bogus = %+v", got)
	}
	if got := c.do("CONFIG", "SET", "maxmemory", "0"); got.Type != resp.TypeSimpleString {
		t.Fatalf("CONFIG SET maxmemory 0 = %+v", got)
	}
	if got := c.do("SET", "unlimited", strings.Repeat("v", 1<<20)); got.Type == resp.TypeError {
		t.Fatalf("SET after removing the limit = %+v", got)
	}
}

func TestConfigSetMaxclientsAndTimeout(t *testing.T) {
	s := startReplServerCfg(t, replTestConfig(t, nil))
	c := dialRepl(t, s)
	if got := c.do("CONFIG", "SET", "maxclients", "1"); got.Type != resp.TypeSimpleString {
		t.Fatalf("CONFIG SET maxclients = %+v", got)
	}
	conn, err := net.Dial("tcp", s.listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 128)
	n, _ := conn.Read(buf)
	conn.Close()
	if !strings.Contains(string(buf[:n]), "max number of clients") {
		t.Fatalf("second client with maxclients 1 got %q", buf[:n])
	}
	if got := c.do("CONFIG", "SET", "maxclients", "100"); got.Type != resp.TypeSimpleString {
		t.Fatalf("CONFIG SET maxclients = %+v", got)
	}
	if got := c.do("CONFIG", "SET", "timeout", "1"); got.Type != resp.TypeSimpleString {
		t.Fatalf("CONFIG SET timeout = %+v", got)
	}
	if got := configPairs(t, c.do("CONFIG", "GET", "timeout")); got["timeout"] != "1" {
		t.Fatalf("CONFIG GET timeout = %v", got)
	}
	idle, err := net.Dial("tcp", s.listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer idle.Close()
	idle.SetReadDeadline(time.Now().Add(5 * time.Second))
	if _, err := idle.Read(buf); err == nil {
		t.Fatal("an idle client was not disconnected after the runtime timeout")
	}
}

func TestIdleClientsStayConnectedByDefault(t *testing.T) {
	s := startReplServerCfg(t, replTestConfig(t, nil))
	c := dialRepl(t, s)
	if got := configPairs(t, c.do("CONFIG", "GET", "timeout")); got["timeout"] != "0" {
		t.Fatalf("default CONFIG GET timeout = %v, want 0 as in Redis", got)
	}
}

func TestConfigSetRequirepass(t *testing.T) {
	s := startReplServerCfg(t, replTestConfig(t, nil))
	c := dialRepl(t, s)
	if got := c.do("CONFIG", "SET", "requirepass", "s3cret"); got.Type != resp.TypeSimpleString {
		t.Fatalf("CONFIG SET requirepass = %+v", got)
	}
	other := dialRepl(t, s)
	if got := other.do("GET", "x"); got.Type != resp.TypeError || !strings.HasPrefix(got.Err, "NOAUTH") {
		t.Fatalf("GET without AUTH after CONFIG SET requirepass = %+v", got)
	}
	if got := other.do("AUTH", "s3cret"); got.Type != resp.TypeSimpleString {
		t.Fatalf("AUTH = %+v", got)
	}
	if got := c.do("CONFIG", "SET", "port", "1234"); got.Type != resp.TypeError {
		t.Fatalf("CONFIG SET port = %+v, want an immutable-config error", got)
	}
}
