package server

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/resp"
)

func TestAOFFailureDoesNotAcknowledgeWrites(t *testing.T) {
	if _, err := os.Stat("/dev/full"); err != nil {
		t.Skip("requires /dev/full")
	}
	for _, mode := range []string{"command", "transaction", "script"} {
		t.Run(mode, func(t *testing.T) {
			s := fullAOFServer(t)
			tx := command.NewTransaction()
			run := func(name string, args ...string) string {
				var out bytes.Buffer
				argv := make([][]byte, len(args))
				for i := range args {
					argv[i] = []byte(args[i])
				}
				ctx := command.NewContext(name, argv, s.store, resp.NewWriter(&out))
				ctx.Transaction = tx
				if err := s.router.Execute(ctx); err != nil {
					_ = ctx.Writer.WriteError(err.Error())
				}
				return out.String()
			}
			var reply string
			switch mode {
			case "command":
				reply = run("SET", "first", "v")
			case "transaction":
				run("MULTI")
				run("SET", "first", "v")
				reply = run("EXEC")
			case "script":
				reply = run("EVAL", "redis.call('SET', 'first', 'v'); return string.rep('x', 10000)", "0")
			}
			if !strings.HasPrefix(reply, "-MISCONF ") || strings.Count(reply, "\r\n") != 1 {
				t.Fatalf("failed persistence acknowledged or malformed reply: %.120q", reply)
			}
			if reply := run("SET", "later", "v"); !strings.HasPrefix(reply, "-MISCONF ") {
				t.Fatalf("subsequent write accepted: %q", reply)
			}
			if s.store.Exists("later") {
				t.Fatal("write mutated data after persistence failure")
			}
			if reply := run("EVAL", "redis.call('SET', 'later-script', 'v')", "0"); !strings.HasPrefix(reply, "-MISCONF ") || s.store.Exists("later-script") {
				t.Fatalf("script ran after persistence failure: %q", reply)
			}
			run("MULTI")
			if reply := run("SET", "later-transaction", "v"); !strings.HasPrefix(reply, "-MISCONF ") || !tx.IsAborted() {
				t.Fatalf("write queued after persistence failure: %q", reply)
			}
			if reply := run("DISCARD"); reply != "+OK\r\n" {
				t.Fatalf("failed to discard transaction: %q", reply)
			}
			if reply := run("PING"); reply != "+PONG\r\n" {
				t.Fatalf("read/control command refused: %q", reply)
			}
			if reply := run("INFO", "persistence"); !strings.Contains(reply, "aof_last_write_status:err") {
				t.Fatalf("missing failed AOF status: %q", reply)
			}
		})
	}
}

func fullAOFServer(t *testing.T) *Server {
	t.Helper()
	if _, err := os.Stat("/dev/full"); err != nil {
		t.Skip("requires /dev/full")
	}
	cfg := aofTxConfig(t)
	cfg.Server.Bind, cfg.Server.Port = "127.0.0.1", 0
	s, err := newOwnedTestServer(t, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/dev/full", filepath.Join(cfg.Persistence.DataDir, "appendonly.aof")); err != nil {
		t.Fatal(err)
	}
	if err := s.aof.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { stopReplServer(s) })
	return s
}

func TestAOFFailureKeepsTCPPipelineAligned(t *testing.T) {
	s := fullAOFServer(t)
	if err := s.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { stopReplServer(s) })
	c := dialRepl(t, s)
	_ = c.conn.SetDeadline(time.Now().Add(5 * time.Second))
	data := append(encodeArgs("PING"), encodeArgs("SET", "first", "v")...)
	data = append(data, encodeArgs("PING")...)
	if _, err := c.conn.Write(data); err != nil {
		t.Fatal(err)
	}
	for i, typ := range []resp.Type{resp.TypeSimpleString, resp.TypeError, resp.TypeSimpleString} {
		v, err := c.rd.ReadValue()
		if err != nil {
			t.Fatal(err)
		}
		if v.Type != typ {
			t.Fatalf("reply %d: %#v", i, v)
		}
		if typ == resp.TypeError && !strings.HasPrefix(v.Err, "MISCONF ") {
			t.Fatalf("unexpected error: %s", v.Err)
		}
		if typ == resp.TypeSimpleString && v.Str != "PONG" {
			t.Fatalf("unexpected reply: %#v", v)
		}
	}
	if v := c.do("SET", "later", "v"); v.Type != resp.TypeError {
		t.Fatalf("accepted later write: %#v", v)
	}
	if s.store.Exists("later") {
		t.Fatal("later write mutated state")
	}
}

func TestAOFFailureReturnsHTTPUnavailable(t *testing.T) {
	for _, mode := range []string{"execute", "rest"} {
		t.Run(mode, func(t *testing.T) {
			s := fullAOFServer(t)
			h := NewHTTPServer(s.store, s.router, &HTTPConfig{})
			h.srv, h.dbs = s, s.dbs
			t.Cleanup(h.cancel)
			for _, key := range []string{"first", "later"} {
				body, path := fmt.Sprintf(`{"command":"SET","args":[%q,"v"]}`, key), "/api/execute"
				if mode == "rest" {
					body, path = fmt.Sprintf(`{"key":%q,"value":"v"}`, key), "/api/keys"
				}
				req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
				out := httptest.NewRecorder()
				if mode == "rest" {
					h.handleKeys(out, req)
				} else {
					h.handleExecute(out, req)
				}
				if out.Code != http.StatusServiceUnavailable || !strings.Contains(out.Body.String(), "MISCONF") {
					t.Fatalf("failure acknowledged: %d %s", out.Code, out.Body.String())
				}
			}
			if s.store.Exists("later") {
				t.Fatal("later HTTP write mutated state")
			}
			h.SetReady(true)
			ready := httptest.NewRecorder()
			h.handleReady(ready, httptest.NewRequest(http.MethodGet, "/api/ready", nil))
			if ready.Code != http.StatusServiceUnavailable {
				t.Fatalf("unhealthy writer reported ready: %d", ready.Code)
			}
		})
	}
}
