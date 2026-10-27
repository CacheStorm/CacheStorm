package server

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/config"
	"github.com/cachestorm/cachestorm/internal/resp"
)

// replTestConfig is a configuration for a server of a replication test:
// loopback, an ephemeral port, no HTTP or metrics listener.
func replTestConfig(t *testing.T, mutate func(*config.Config)) *config.Config {
	t.Helper()
	cfg := config.Default()
	cfg.Server.Bind = "127.0.0.1"
	cfg.Server.Port = 0
	cfg.HTTP.Enabled = false
	cfg.Plugins.Metrics.Enabled = false
	cfg.Persistence.Enabled = false
	cfg.Logging.Level = "error"
	if mutate != nil {
		mutate(cfg)
	}
	return cfg
}

// startReplServer starts a server and stops it when the test ends.
func startReplServer(t *testing.T, mutate func(*config.Config)) *Server {
	t.Helper()
	return startReplServerCfg(t, replTestConfig(t, mutate))
}

func startReplServerCfg(t *testing.T, cfg *config.Config) *Server {
	t.Helper()
	s, err := newOwnedTestServer(t, cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { stopReplServer(s) })
	return s
}

// stopReplServer stops s; stopping twice is harmless.
func stopReplServer(s *Server) {
	if s.stopping.Load() {
		return
	}
	_ = s.Stop(context.Background())
}

func (s *Server) testPort() int {
	return s.listener.Addr().(*net.TCPAddr).Port
}

// replClient is a minimal RESP client.
type replClient struct {
	t    *testing.T
	conn net.Conn
	rd   *resp.Reader
}

func dialRepl(t *testing.T, s *Server) *replClient {
	t.Helper()
	return dialReplPort(t, s.testPort())
}

func dialReplPort(t *testing.T, port int) *replClient {
	t.Helper()
	conn, err := net.DialTimeout("tcp", "127.0.0.1:"+strconv.Itoa(port), 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return &replClient{t: t, conn: conn, rd: resp.NewReader(bufio.NewReader(conn))}
}

func encodeArgs(args ...string) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "*%d\r\n", len(args))
	for _, a := range args {
		fmt.Fprintf(&b, "$%d\r\n%s\r\n", len(a), a)
	}
	return []byte(b.String())
}

// do sends a command and returns its reply.
func (c *replClient) do(args ...string) *resp.Value {
	c.t.Helper()
	_ = c.conn.SetDeadline(time.Now().Add(20 * time.Second))
	if _, err := c.conn.Write(encodeArgs(args...)); err != nil {
		c.t.Fatalf("%v: write: %v", args, err)
	}
	v, err := c.rd.ReadValue()
	if err != nil {
		c.t.Fatalf("%v: read: %v", args, err)
	}
	return v
}

// str renders a reply compactly for comparisons.
func str(v *resp.Value) string {
	if v == nil {
		return "<nil>"
	}
	switch v.Type {
	case resp.TypeSimpleString:
		return v.Str
	case resp.TypeError:
		return "ERR:" + v.Err
	case resp.TypeInteger:
		return strconv.FormatInt(v.Int, 10)
	case resp.TypeBulkString:
		if v.IsNull {
			return "<null>"
		}
		return string(v.Bulk)
	case resp.TypeArray:
		if v.IsNull {
			return "<nullarray>"
		}
		parts := make([]string, len(v.Array))
		for i, e := range v.Array {
			parts[i] = str(e)
		}
		return "[" + strings.Join(parts, " ") + "]"
	case resp.TypeNull:
		return "<null>"
	}
	return fmt.Sprintf("%#v", v)
}

func (c *replClient) str(args ...string) string {
	c.t.Helper()
	return str(c.do(args...))
}

// info returns the fields of an INFO section.
func (c *replClient) info(section string) map[string]string {
	c.t.Helper()
	out := map[string]string{}
	for _, line := range strings.Split(str(c.do("INFO", section)), "\r\n") {
		if k, v, ok := strings.Cut(line, ":"); ok {
			out[k] = v
		}
	}
	return out
}

// eventually polls cond until it holds or timeout passes.
func eventually(t *testing.T, timeout time.Duration, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		if cond() {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// replicate makes replica follow master and waits for the link.
func replicate(t *testing.T, replica, master *Server) *replClient {
	t.Helper()
	c := dialRepl(t, replica)
	if got := c.str("REPLICAOF", "127.0.0.1", strconv.Itoa(master.testPort())); got != "OK" {
		t.Fatalf("REPLICAOF: %s", got)
	}
	waitLinkUp(t, c)
	return c
}

func waitLinkUp(t *testing.T, c *replClient) {
	t.Helper()
	eventually(t, 10*time.Second, "master link up", func() bool {
		return c.info("replication")["master_link_status"] == "up"
	})
}

// waitSynced waits until the replica's offset reaches the master's.
func waitSynced(t *testing.T, replica, master *Server) {
	t.Helper()
	eventually(t, 10*time.Second, "replica caught up", func() bool {
		_, mo := master.repl.ReplID()
		_, ro := replica.repl.ReplID()
		return ro == mo
	})
}

// dumpAll returns every key of every database with its type, value and
// whether it has a TTL, as seen through the protocol.
func dumpAll(t *testing.T, c *replClient) map[string]string {
	t.Helper()
	out := map[string]string{}
	for db := 0; db < 4; db++ {
		c.str("SELECT", strconv.Itoa(db))
		keys := c.do("KEYS", "*")
		for _, k := range keys.Array {
			key := string(k.Bulk)
			typ := c.str("TYPE", key)
			var val string
			switch typ {
			case "string":
				val = c.str("GET", key)
			case "list":
				val = c.str("LRANGE", key, "0", "-1")
			case "set":
				val = sortedMembers(c.do("SMEMBERS", key))
			case "hash":
				val = sortedPairs(c.do("HGETALL", key))
			case "zset":
				val = c.str("ZRANGE", key, "0", "-1", "WITHSCORES")
			case "stream":
				val = c.str("XRANGE", key, "-", "+")
			default:
				val = typ
			}
			ttl := "persist"
			if c.do("PTTL", key).Int > 0 {
				ttl = "ttl"
			}
			out[strconv.Itoa(db)+"/"+key] = typ + "|" + val + "|" + ttl
		}
	}
	c.str("SELECT", "0")
	return out
}

func sortedMembers(v *resp.Value) string {
	m := make([]string, len(v.Array))
	for i, e := range v.Array {
		m[i] = string(e.Bulk)
	}
	sortStrings(m)
	return strings.Join(m, ",")
}

func sortedPairs(v *resp.Value) string {
	var m []string
	for i := 0; i+1 < len(v.Array); i += 2 {
		m = append(m, string(v.Array[i].Bulk)+"="+string(v.Array[i+1].Bulk))
	}
	sortStrings(m)
	return strings.Join(m, ",")
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// sameData fails the test unless both servers hold the same keys and values.
func sameData(t *testing.T, a, b *replClient) {
	t.Helper()
	da, db := dumpAll(t, a), dumpAll(t, b)
	if len(da) != len(db) {
		t.Fatalf("key count differs: %d vs %d\n%v\n%v", len(da), len(db), da, db)
	}
	for k, v := range da {
		if db[k] != v {
			t.Fatalf("key %s differs: %q vs %q", k, v, db[k])
		}
	}
}
