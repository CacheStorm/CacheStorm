package server

import (
	"strconv"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
)

// next reads one frame without sending anything.
func (c *replClient) next(wait time.Duration) (*resp.Value, bool) {
	c.t.Helper()
	_ = c.conn.SetReadDeadline(time.Now().Add(wait))
	v, err := c.rd.ReadValue()
	if err != nil {
		return nil, false
	}
	return v, true
}

func hello3(t *testing.T, c *replClient) {
	t.Helper()
	got := c.do("HELLO", "3")
	if got.Type != resp.TypeMap || got.Map != nil && got.Map["proto"] == nil {
		t.Fatalf("HELLO 3 = %+v", got)
	}
	if got.Map["proto"].Int != 3 {
		t.Fatalf("HELLO 3 proto = %+v", got.Map["proto"])
	}
}

// invalidated reads frames until an invalidation push arrives and returns
// its keys (nil for a flush).
func invalidated(t *testing.T, c *replClient) []string {
	t.Helper()
	v, ok := c.next(3 * time.Second)
	if !ok {
		t.Fatal("no invalidation arrived")
	}
	if v.Type != resp.TypePush || len(v.Array) != 2 || string(v.Array[0].Bulk) != "invalidate" {
		t.Fatalf("expected an invalidate push, got %+v", v)
	}
	if v.Array[1].Type == resp.TypeNull {
		return nil
	}
	keys := make([]string, 0, len(v.Array[1].Array))
	for _, k := range v.Array[1].Array {
		keys = append(keys, string(k.Bulk))
	}
	return keys
}

func noFrame(t *testing.T, c *replClient) {
	t.Helper()
	if v, ok := c.next(200 * time.Millisecond); ok {
		t.Fatalf("unexpected frame %+v", v)
	}
}

func TestRESP3Replies(t *testing.T) {
	s := startReplServer(t, nil)
	c := dialRepl(t, s)
	hello3(t, c)
	if got := c.do("GET", "missing"); got.Type != resp.TypeNull {
		t.Fatalf("GET of a missing key in RESP3 = %+v, want the RESP3 null", got)
	}
	c.do("HSET", "h", "f", "v")
	if got := c.do("HGETALL", "h"); got.Type != resp.TypeMap || string(got.Map["f"].Bulk) != "v" {
		t.Fatalf("HGETALL in RESP3 = %+v, want a map", got)
	}
	if got := c.do("HELLO", "2"); got.Type != resp.TypeArray {
		t.Fatalf("HELLO 2 = %+v, want a flat array", got)
	}
	if got := c.do("GET", "missing"); got.Type != resp.TypeBulkString || !got.IsNull {
		t.Fatalf("GET after HELLO 2 = %+v, want a null bulk string", got)
	}
	if got := c.do("HELLO", "4"); got.Type != resp.TypeError {
		t.Fatalf("HELLO 4 = %+v, want NOPROTO", got)
	}
}

func TestRESP3PubSubIsPushAndAllowsCommands(t *testing.T) {
	s := startReplServer(t, nil)
	sub := dialRepl(t, s)
	hello3(t, sub)
	if got := sub.do("SUBSCRIBE", "ch"); got.Type != resp.TypePush || string(got.Array[0].Bulk) != "subscribe" {
		t.Fatalf("SUBSCRIBE ack in RESP3 = %+v, want a push", got)
	}
	if got := sub.do("SET", "x", "1"); got.Type != resp.TypeSimpleString {
		t.Fatalf("SET while subscribed in RESP3 = %+v, want OK", got)
	}
	pub := dialRepl(t, s)
	if got := pub.do("PUBLISH", "ch", "hello"); got.Int != 1 {
		t.Fatalf("PUBLISH = %+v", got)
	}
	v, ok := sub.next(3 * time.Second)
	if !ok || v.Type != resp.TypePush || string(v.Array[0].Bulk) != "message" || string(v.Array[2].Bulk) != "hello" {
		t.Fatalf("message in RESP3 = %+v, want a push", v)
	}
}

func TestTrackingDefaultModeRESP3(t *testing.T) {
	s := startReplServer(t, nil)
	c := dialRepl(t, s)
	hello3(t, c)
	if got := c.do("CLIENT", "TRACKING", "ON"); got.Type != resp.TypeSimpleString {
		t.Fatalf("CLIENT TRACKING ON = %+v", got)
	}
	w := dialRepl(t, s)
	w.do("SET", "k", "1")
	w.do("SET", "untracked", "1")
	if got := c.do("GET", "k"); string(got.Bulk) != "1" {
		t.Fatalf("GET = %+v", got)
	}
	w.do("SET", "untracked", "2")
	noFrame(t, c)
	w.do("SET", "k", "2")
	if keys := invalidated(t, c); len(keys) != 1 || keys[0] != "k" {
		t.Fatalf("invalidated %v, want [k]", keys)
	}
	// The pair is forgotten after one invalidation until the key is read
	// again.
	w.do("SET", "k", "3")
	noFrame(t, c)
	c.do("GET", "k")
	w.do("DEL", "k")
	if keys := invalidated(t, c); len(keys) != 1 || keys[0] != "k" {
		t.Fatalf("invalidated %v after DEL, want [k]", keys)
	}
	c.do("GET", "k2")
	w.do("FLUSHALL")
	if keys := invalidated(t, c); keys != nil {
		t.Fatalf("FLUSHALL invalidated %v, want a null key list", keys)
	}
	info := c.do("CLIENT", "TRACKINGINFO")
	if info.Type != resp.TypeMap || string(info.Map["flags"].Array[0].Bulk) != "on" {
		t.Fatalf("CLIENT TRACKINGINFO = %+v", info)
	}
	c.do("CLIENT", "TRACKING", "OFF")
	c.do("GET", "k")
	w.do("SET", "k", "4")
	noFrame(t, c)
}

func TestTrackingOwnWritesAndNoLoop(t *testing.T) {
	s := startReplServer(t, nil)
	c := dialRepl(t, s)
	hello3(t, c)
	c.do("CLIENT", "TRACKING", "ON")
	c.do("GET", "k")
	// The reply and the push travel separately, so either may come first.
	first := c.do("SET", "k", "1")
	second, ok := c.next(3 * time.Second)
	if !ok {
		t.Fatal("own write: missing frame")
	}
	if first.Type == resp.TypePush {
		first, second = second, first
	}
	if first.Type != resp.TypeSimpleString || second.Type != resp.TypePush || string(second.Array[0].Bulk) != "invalidate" {
		t.Fatalf("own write frames = %+v, %+v", first, second)
	}

	n := dialRepl(t, s)
	hello3(t, n)
	n.do("CLIENT", "TRACKING", "ON", "NOLOOP")
	n.do("GET", "q")
	n.do("SET", "q", "1")
	noFrame(t, n)
}

func TestTrackingBroadcastPrefixes(t *testing.T) {
	s := startReplServer(t, nil)
	c := dialRepl(t, s)
	hello3(t, c)
	if got := c.do("CLIENT", "TRACKING", "ON", "BCAST", "PREFIX", "user:"); got.Type != resp.TypeSimpleString {
		t.Fatalf("CLIENT TRACKING BCAST = %+v", got)
	}
	w := dialRepl(t, s)
	w.do("SET", "order:1", "x")
	noFrame(t, c)
	w.do("SET", "user:1", "x")
	if keys := invalidated(t, c); len(keys) != 1 || keys[0] != "user:1" {
		t.Fatalf("BCAST invalidated %v", keys)
	}
	if got := c.do("CLIENT", "TRACKING", "ON", "PREFIX", "a"); got.Type != resp.TypeError {
		t.Fatalf("PREFIX without BCAST = %+v, want an error", got)
	}
}

func TestTrackingOptIn(t *testing.T) {
	s := startReplServer(t, nil)
	c := dialRepl(t, s)
	hello3(t, c)
	c.do("CLIENT", "TRACKING", "ON", "OPTIN")
	c.do("GET", "plain")
	c.do("CLIENT", "CACHING", "YES")
	c.do("GET", "cached")
	w := dialRepl(t, s)
	w.do("SET", "plain", "1")
	noFrame(t, c)
	w.do("SET", "cached", "1")
	if keys := invalidated(t, c); len(keys) != 1 || keys[0] != "cached" {
		t.Fatalf("OPTIN invalidated %v", keys)
	}
	if got := c.do("CLIENT", "CACHING", "NO"); got.Type != resp.TypeError {
		t.Fatalf("CLIENT CACHING NO in OPTIN mode = %+v, want an error", got)
	}
}

func TestTrackingRedirectRESP2(t *testing.T) {
	s := startReplServer(t, nil)
	r := dialRepl(t, s)
	rid := r.do("CLIENT", "ID").Int
	if got := r.do("SUBSCRIBE", invalidateChannel); got.Type != resp.TypeArray {
		t.Fatalf("SUBSCRIBE = %+v", got)
	}
	c := dialRepl(t, s)
	if got := c.do("CLIENT", "TRACKING", "ON", "REDIRECT", strconv.FormatInt(rid, 10)); got.Type != resp.TypeSimpleString {
		t.Fatalf("CLIENT TRACKING ON REDIRECT = %+v", got)
	}
	if got := c.do("CLIENT", "GETREDIR"); got.Int != rid {
		t.Fatalf("CLIENT GETREDIR = %+v, want %d", got, rid)
	}
	c.do("GET", "k")
	w := dialRepl(t, s)
	w.do("SET", "k", "1")
	v, ok := r.next(3 * time.Second)
	if !ok || v.Type != resp.TypeArray || len(v.Array) != 3 || string(v.Array[0].Bulk) != "message" ||
		string(v.Array[1].Bulk) != invalidateChannel || string(v.Array[2].Array[0].Bulk) != "k" {
		t.Fatalf("redirected invalidation = %+v", v)
	}
	if got := c.do("CLIENT", "TRACKING", "ON", "REDIRECT", "999999"); got.Type != resp.TypeError {
		t.Fatalf("REDIRECT to a missing client = %+v, want an error", got)
	}
}

func TestTrackingExpiryInvalidates(t *testing.T) {
	s := startReplServer(t, nil)
	c := dialRepl(t, s)
	hello3(t, c)
	c.do("CLIENT", "TRACKING", "ON")
	w := dialRepl(t, s)
	w.do("SET", "short", "1", "PX", "100")
	c.do("GET", "short")
	if keys := invalidated(t, c); len(keys) != 1 || keys[0] != "short" {
		t.Fatalf("expiry invalidated %v", keys)
	}
}

// An error reply earlier on the connection must not stop later reads from
// being tracked: redis-py sends CLIENT SETINFO, which older servers refuse,
// before it turns tracking on.
func TestTrackingAfterAnEarlierError(t *testing.T) {
	s := startReplServer(t, nil)
	c := dialRepl(t, s)
	hello3(t, c)
	if got := c.do("NOSUCHCOMMAND"); got.Type != resp.TypeError {
		t.Fatalf("unknown command = %+v", got)
	}
	c.do("CLIENT", "TRACKING", "ON")
	c.do("GET", "k")
	dialRepl(t, s).do("SET", "k", "1")
	if keys := invalidated(t, c); len(keys) != 1 || keys[0] != "k" {
		t.Fatalf("invalidated %v", keys)
	}
}
