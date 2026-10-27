package server

import (
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/config"
	"github.com/cachestorm/cachestorm/internal/resp"
)

func TestResetRestoresConnectionDefaults(t *testing.T) {
	s := startReplServer(t, nil)
	c := dialRepl(t, s)
	c.do("SET", "k", "v")
	c.do("SELECT", "3")
	c.do("CLIENT", "SETNAME", "worker")
	c.do("WATCH", "k")
	c.do("MULTI")
	if got := c.do("RESET"); got.Type != resp.TypeSimpleString || got.Str != "RESET" {
		t.Fatalf("RESET inside MULTI = %+v", got)
	}
	if got := c.do("GET", "k"); string(got.Bulk) != "v" {
		t.Fatalf("GET after RESET = %+v, want db 0 and no open MULTI", got)
	}
	if got := c.do("CLIENT", "GETNAME"); !got.IsNull {
		t.Fatalf("CLIENT GETNAME after RESET = %+v", got)
	}
	if got := c.do("EXEC"); got.Type != resp.TypeError {
		t.Fatalf("EXEC after RESET = %+v, want an error", got)
	}

	c.do("HELLO", "3")
	c.do("CLIENT", "TRACKING", "ON")
	c.do("RESET")
	if got := str(c.do("CLIENT", "TRACKINGINFO")); !strings.HasPrefix(got, "[flags [off]") {
		t.Fatalf("CLIENT TRACKINGINFO after RESET = %s", got)
	}

	sub := dialRepl(t, s)
	sub.do("SUBSCRIBE", "ch")
	if got := sub.do("RESET"); got.Str != "RESET" {
		t.Fatalf("RESET while subscribed = %+v", got)
	}
	if got := sub.do("PING"); got.Str != "PONG" {
		t.Fatalf("PING after RESET = %+v, want a plain PONG", got)
	}
	if got := c.do("PUBLISH", "ch", "x"); got.Int != 0 {
		t.Fatalf("PUBLISH after the subscriber's RESET reached %d clients", got.Int)
	}
}

func TestResetDeauthenticates(t *testing.T) {
	s := startReplServer(t, func(cfg *config.Config) { cfg.Server.RequirePass = "pw" })
	c := dialRepl(t, s)
	c.do("AUTH", "pw")
	if got := c.do("PING"); got.Str != "PONG" {
		t.Fatalf("PING after AUTH = %+v", got)
	}
	c.do("RESET")
	if got := c.do("PING"); got.Type != resp.TypeError {
		t.Fatalf("PING after RESET = %+v, want NOAUTH", got)
	}
}
