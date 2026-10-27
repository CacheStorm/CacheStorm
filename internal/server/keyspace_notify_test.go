package server

import (
	"strings"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/config"
	"github.com/cachestorm/cachestorm/internal/resp"
)

// keyEvents subscribes c to every keyevent channel and returns a reader of
// the "event key" pairs it receives.
func keyEvents(t *testing.T, c *replClient) func(n int) []string {
	t.Helper()
	if got := str(c.do("PSUBSCRIBE", "__keyevent@*")); !strings.HasPrefix(got, "[psubscribe ") {
		t.Fatalf("PSUBSCRIBE: %s", got)
	}
	return func(n int) []string {
		t.Helper()
		var out []string
		for len(out) < n {
			v, ok := c.next(3 * time.Second)
			if !ok {
				t.Fatalf("got %d events %v, want %d", len(out), out, n)
			}
			if len(v.Array) != 4 || string(v.Array[0].Bulk) != "pmessage" {
				t.Fatalf("unexpected frame %+v", v)
			}
			ch := string(v.Array[2].Bulk)
			out = append(out, ch[strings.Index(ch, "__:")+3:]+" "+string(v.Array[3].Bulk))
		}
		return out
	}
}

func sameEvents(t *testing.T, got []string, want ...string) {
	t.Helper()
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("events %v, want %v", got, want)
	}
}

func TestKeyspaceNotificationsOffByDefault(t *testing.T) {
	s := startReplServer(t, nil)
	c := dialRepl(t, s)
	if got := c.str("CONFIG", "GET", "notify-keyspace-events"); got != "[notify-keyspace-events ]" {
		t.Fatalf("CONFIG GET: %s", got)
	}
	sub := dialRepl(t, s)
	sub.do("PSUBSCRIBE", "*")
	c.do("SET", "a", "1")
	c.do("DEL", "a")
	noFrame(t, sub)
}

func TestKeyspaceNotificationsBothChannels(t *testing.T) {
	s := startReplServer(t, nil)
	c := dialRepl(t, s)
	if got := c.str("CONFIG", "SET", "notify-keyspace-events", "KEA"); got != "OK" {
		t.Fatalf("CONFIG SET: %s", got)
	}
	if got := c.str("CONFIG", "SET", "notify-keyspace-events", "KEZ"); !strings.Contains(got, "Invalid event class character") {
		t.Fatalf("invalid flags: %s", got)
	}
	if got := c.str("CONFIG", "GET", "notify-keyspace-events"); got != "[notify-keyspace-events AKE]" {
		t.Fatalf("CONFIG GET: %s", got)
	}
	sub := dialRepl(t, s)
	sub.do("PSUBSCRIBE", "__key*__:*")
	c.do("SET", "k", "v")
	want := []string{
		"__keyspace@0__:k set",
		"__keyevent@0__:set k",
	}
	for _, w := range want {
		v, ok := sub.next(3 * time.Second)
		if !ok || len(v.Array) != 4 {
			t.Fatalf("no frame for %q: %+v", w, v)
		}
		if got := string(v.Array[2].Bulk) + " " + string(v.Array[3].Bulk); got != w {
			t.Fatalf("frame %q, want %q", got, w)
		}
	}
}

func TestKeyspaceNotificationsRESP3Push(t *testing.T) {
	s := startReplServer(t, func(cfg *config.Config) { cfg.Server.NotifyKeyspaceEvents = "Eg" })
	c := dialRepl(t, s)
	sub := dialRepl(t, s)
	hello3(t, sub)
	sub.do("SUBSCRIBE", "__keyevent@0__:del")
	c.do("SET", "a", "1")
	c.do("DEL", "a")
	v, ok := sub.next(3 * time.Second)
	if !ok || v.Type != resp.TypePush || len(v.Array) != 3 || string(v.Array[2].Bulk) != "a" {
		t.Fatalf("expected a push message, got %+v", v)
	}
	// The subscribed RESP3 client still runs commands.
	if got := sub.str("PING"); got != "PONG" {
		t.Fatalf("PING: %s", got)
	}
}

func TestKeyspaceNotificationsConfigFile(t *testing.T) {
	s := startReplServer(t, func(cfg *config.Config) { cfg.Server.NotifyKeyspaceEvents = "xKEg" })
	c := dialRepl(t, s)
	if got := c.str("CONFIG", "GET", "notify-keyspace-events"); got != "[notify-keyspace-events gxKE]" {
		t.Fatalf("CONFIG GET: %s", got)
	}
	cfg := replTestConfig(t, func(cfg *config.Config) { cfg.Server.NotifyKeyspaceEvents = "KEq" })
	if err := config.Validate(cfg); err == nil {
		t.Fatal("Validate accepted an invalid notify_keyspace_events")
	}
	if srv, err := New(cfg); err == nil {
		_ = srv.Stop(t.Context())
		t.Fatal("New accepted an invalid notify_keyspace_events")
	}
}

func TestKeyspaceNotificationsExpiryAndEviction(t *testing.T) {
	s := startReplServer(t, func(cfg *config.Config) { cfg.Server.NotifyKeyspaceEvents = "EA" })
	c := dialRepl(t, s)
	events := keyEvents(t, dialRepl(t, s))

	// Active expiry, without any read of the key.
	c.do("SET", "gone", "1", "PX", "30")
	sameEvents(t, events(3), "set gone", "expire gone", "expired gone")

	// Lazy expiry on a read.
	c.do("SET", "lazy", "1", "PX", "1")
	sameEvents(t, events(2), "set lazy", "expire lazy")
	time.Sleep(5 * time.Millisecond)
	c.do("GET", "lazy")
	sameEvents(t, events(1), "expired lazy")

	// Field expiry: hexpired for each reap, del once the hash is empty.
	c.do("HSET", "h", "a", "1", "b", "2")
	c.do("HPEXPIRE", "h", "30", "FIELDS", "2", "a", "b")
	sameEvents(t, events(4), "hset h", "hexpire h", "hexpired h", "del h")

	// Eviction.
	c.do("CONFIG", "SET", "maxmemory-policy", "allkeys-lru")
	c.do("SET", "victim", strings.Repeat("x", 4096))
	sameEvents(t, events(1), "set victim")
	c.do("CONFIG", "SET", "maxmemory", "1")
	c.do("SET", "next", "y")
	c.do("CONFIG", "SET", "maxmemory", "0")
	sameEvents(t, events(1), "evicted victim")
}

func TestKeyspaceNotificationsDatabaseIndex(t *testing.T) {
	s := startReplServer(t, func(cfg *config.Config) { cfg.Server.NotifyKeyspaceEvents = "Kg" })
	c := dialRepl(t, s)
	sub := dialRepl(t, s)
	sub.do("PSUBSCRIBE", "__keyspace@*")
	read := func() string {
		t.Helper()
		v, ok := sub.next(3 * time.Second)
		if !ok {
			t.Fatal("no event")
		}
		return string(v.Array[2].Bulk) + " " + string(v.Array[3].Bulk)
	}
	c.do("SELECT", "2")
	c.do("SET", "a", "1")
	c.do("DEL", "a")
	if got := read(); got != "__keyspace@2__:a del" {
		t.Fatalf("event %q", got)
	}
	c.do("SET", "b", "1")
	c.do("SWAPDB", "2", "5")
	c.do("SELECT", "5")
	c.do("DEL", "b")
	if got := read(); got != "__keyspace@5__:b del" {
		t.Fatalf("event after SWAPDB %q", got)
	}
	c.do("SELECT", "0")
	c.do("SET", "m", "1")
	c.do("MOVE", "m", "7")
	if got := read(); got != "__keyspace@0__:m move_from" {
		t.Fatalf("event %q", got)
	}
	if got := read(); got != "__keyspace@7__:m move_to" {
		t.Fatalf("event %q", got)
	}
}

// A replica announces the writes it applies from the master, and an expired
// key as the del the master streams, as Redis does (the store tests cover
// that a replica's own lazy expiry announces nothing).
func TestKeyspaceNotificationsOnReplica(t *testing.T) {
	master := startReplServer(t, nil)
	replica := startReplServer(t, func(cfg *config.Config) { cfg.Server.NotifyKeyspaceEvents = "EA" })
	mc := dialRepl(t, master)
	rc := replicate(t, replica, master)
	waitSynced(t, replica, master)
	events := keyEvents(t, dialRepl(t, replica))

	mc.do("LPUSH", "l", "x")
	mc.do("HSET", "h", "f", "v")
	mc.do("LPOP", "l")
	sameEvents(t, events(4), "lpush l", "hset h", "lpop l", "del l")

	mc.do("SET", "t", "1", "PX", "40")
	sameEvents(t, events(2), "set t", "expire t")
	sameEvents(t, events(1), "del t")
	if got := rc.str("GET", "t"); got != "<null>" {
		t.Fatalf("replica GET of an expired key: %s", got)
	}
}
