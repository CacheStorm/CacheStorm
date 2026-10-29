package server

import (
	"errors"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// invalidateChannel is the channel a RESP2 client subscribes to on the
// connection it names with CLIENT TRACKING ... REDIRECT.
const invalidateChannel = "__redis__:invalidate"

// trackedClient is one client with CLIENT TRACKING on.
type trackedClient struct {
	id       int64
	redirect int64
	bcast    bool
	prefixes []string
	optIn    bool
	optOut   bool
	noLoop   bool
	// caching is CLIENT CACHING for the next command: 1 yes, -1 no, 0 unset.
	caching int8
}

// tracker implements Redis client-side caching: it remembers which clients
// read which keys and sends each of them an invalidation message when one of
// those keys changes, then forgets the pair, as Redis' tracking table does.
// BCAST clients are told about every change to a key under their prefixes.
type tracker struct {
	mu      sync.Mutex
	clients map[int64]*trackedClient
	keys    map[string]map[int64]struct{}
	// bcast maps each registered prefix ("" for every key) to its clients.
	bcast map[string]map[int64]struct{}
	// endpoints are the connections that can receive out-of-band messages,
	// by client id: their own tracking messages and REDIRECT targets.
	endpoints map[int64]*Connection
	// tracking counts the clients with tracking on, so commands skip the
	// table entirely while nobody tracks.
	tracking atomic.Int64
}

func newTracker() *tracker {
	return &tracker{
		clients:   make(map[int64]*trackedClient),
		keys:      make(map[string]map[int64]struct{}),
		bcast:     make(map[string]map[int64]struct{}),
		endpoints: make(map[int64]*Connection),
	}
}

// register records that c can receive out-of-band messages.
func (t *tracker) register(c *Connection) {
	t.mu.Lock()
	t.endpoints[c.ID] = c
	t.mu.Unlock()
}

// forget drops a closed connection: its tracking state and its endpoint.
func (t *tracker) forget(id int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.endpoints, id)
	t.disableLocked(id)
}

func (t *tracker) hooks(s *Server) *command.TrackingHooks {
	return &command.TrackingHooks{
		Enable: func(id int64, sub *store.Subscriber, opts command.TrackingOptions) error {
			return t.enable(s, id, opts)
		},
		Disable: func(id int64) {
			t.mu.Lock()
			t.disableLocked(id)
			t.mu.Unlock()
		},
		Caching:  t.setCaching,
		Info:     t.info,
		Redirect: t.redirectOf,
	}
}

func (t *tracker) enable(s *Server, id int64, opts command.TrackingOptions) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if opts.Redirect != 0 && opts.Redirect != id {
		if _, ok := s.conns.Load(opts.Redirect); !ok {
			return errors.New("ERR The client ID you want redirect to does not exist")
		}
	}
	t.disableLocked(id)
	tc := &trackedClient{
		id:       id,
		redirect: opts.Redirect,
		bcast:    opts.BCast,
		prefixes: opts.Prefixes,
		optIn:    opts.OptIn,
		optOut:   opts.OptOut,
		noLoop:   opts.NoLoop,
	}
	t.clients[id] = tc
	t.tracking.Store(int64(len(t.clients)))
	if tc.bcast {
		prefixes := tc.prefixes
		if len(prefixes) == 0 {
			prefixes = []string{""}
		}
		for _, p := range prefixes {
			set := t.bcast[p]
			if set == nil {
				set = make(map[int64]struct{})
				t.bcast[p] = set
			}
			set[id] = struct{}{}
		}
	}
	return nil
}

// disableLocked turns tracking off for id. Keys it read stay in the table
// until they change; invalidation skips clients that are gone.
func (t *tracker) disableLocked(id int64) {
	tc, ok := t.clients[id]
	if !ok {
		return
	}
	delete(t.clients, id)
	t.tracking.Store(int64(len(t.clients)))
	if len(t.clients) == 0 {
		t.keys = make(map[string]map[int64]struct{})
	}
	if tc.bcast {
		for p, set := range t.bcast {
			delete(set, id)
			if len(set) == 0 {
				delete(t.bcast, p)
			}
		}
	}
}

func (t *tracker) setCaching(id int64, yes bool) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	tc, ok := t.clients[id]
	if !ok || (!tc.optIn && !tc.optOut) {
		return errors.New("ERR CLIENT CACHING can be called only when the client is in tracking mode with OPTIN or OPTOUT mode enabled")
	}
	if yes && !tc.optIn {
		return errors.New("ERR CLIENT CACHING YES is only valid when tracking is enabled in OPTIN mode")
	}
	if !yes && !tc.optOut {
		return errors.New("ERR CLIENT CACHING NO is only valid when tracking is enabled in OPTOUT mode")
	}
	if yes {
		tc.caching = 1
	} else {
		tc.caching = -1
	}
	return nil
}

func (t *tracker) redirectOf(id int64) int64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	if tc, ok := t.clients[id]; ok {
		return tc.redirect
	}
	return -1
}

func (t *tracker) info(id int64) *resp.Value {
	t.mu.Lock()
	defer t.mu.Unlock()
	tc, ok := t.clients[id]
	if !ok {
		return resp.PairsValue([]*resp.Value{
			resp.BulkString("flags"), resp.SetValue([]*resp.Value{resp.BulkString("off")}),
			resp.BulkString("redirect"), resp.IntegerValue(-1),
			resp.BulkString("prefixes"), resp.ArrayValue([]*resp.Value{}),
		})
	}
	flags := []*resp.Value{resp.BulkString("on")}
	for _, f := range []struct {
		set  bool
		name string
	}{{tc.bcast, "bcast"}, {tc.optIn, "optin"}, {tc.optOut, "optout"}, {tc.caching == 1, "caching-yes"}, {tc.caching == -1, "caching-no"}, {tc.noLoop, "noloop"}} {
		if f.set {
			flags = append(flags, resp.BulkString(f.name))
		}
	}
	redirect := tc.redirect
	prefixes := make([]*resp.Value, 0, len(tc.prefixes))
	for _, p := range tc.prefixes {
		prefixes = append(prefixes, resp.BulkString(p))
	}
	return resp.PairsValue([]*resp.Value{
		resp.BulkString("flags"), resp.SetValue(flags),
		resp.BulkString("redirect"), resp.IntegerValue(redirect),
		resp.BulkString("prefixes"), resp.ArrayValue(prefixes),
	})
}

// active reports whether any client has tracking on.
func (t *tracker) active() bool {
	return t.tracking.Load() > 0
}

// trackRead records the keys a read-only command of client id read. It runs
// while the command still holds the write gate, so no write to those keys
// can slip in between the read and the registration.
func (t *tracker) trackRead(id int64, keys []string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	tc, ok := t.clients[id]
	if !ok {
		return
	}
	caching := tc.caching
	tc.caching = 0
	if tc.bcast || len(keys) == 0 {
		return
	}
	if (tc.optIn && caching != 1) || (tc.optOut && caching == -1) {
		return
	}
	for _, k := range keys {
		set := t.keys[k]
		if set == nil {
			set = make(map[int64]struct{})
			t.keys[k] = set
		}
		set[id] = struct{}{}
	}
}

// clearCaching drops a pending CLIENT CACHING once the next command ran
// without reading a key.
func (t *tracker) clearCaching(id int64) {
	t.mu.Lock()
	if tc, ok := t.clients[id]; ok {
		tc.caching = 0
	}
	t.mu.Unlock()
}

// invalidate tells every client tracking one of keys that it changed. writer
// is the client that changed them, which NOLOOP clients are not told about.
func (t *tracker) invalidate(writer int64, keys []string) {
	if len(keys) == 0 || !t.active() {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.keys) == 0 && len(t.bcast) == 0 {
		return
	}
	pending := make(map[int64][]string)
	for _, k := range keys {
		if set, ok := t.keys[k]; ok {
			for id := range set {
				pending[id] = append(pending[id], k)
			}
			delete(t.keys, k)
		}
		for p, set := range t.bcast {
			if !strings.HasPrefix(k, p) {
				continue
			}
			for id := range set {
				pending[id] = append(pending[id], k)
			}
		}
	}
	for id, ks := range pending {
		tc, ok := t.clients[id]
		if !ok || (tc.noLoop && id == writer) {
			continue
		}
		t.sendLocked(tc, ks)
	}
}

// invalidateAll tells every tracking client that the whole keyspace changed,
// as FLUSHALL and FLUSHDB do, with a null key list.
func (t *tracker) invalidateAll(writer int64) {
	if !t.active() {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.keys = make(map[string]map[int64]struct{})
	for id, tc := range t.clients {
		if tc.noLoop && id == writer {
			continue
		}
		t.sendLocked(tc, nil)
	}
}

// sendLocked delivers an invalidation for keys (nil for everything) to tc's
// own connection when it speaks RESP3, or as a message on
// __redis__:invalidate to its REDIRECT target.
func (t *tracker) sendLocked(tc *trackedClient, keys []string) {
	target := tc.id
	if tc.redirect != 0 {
		target = tc.redirect
	}
	c, ok := t.endpoints[target]
	if !ok || c.subscriber == nil {
		return
	}
	if c.writer.IsRESP3() {
		c.subscriber.Send(invalidationPush(keys))
		return
	}
	// RESP2 has no push messages: only a REDIRECT target in pub/sub mode
	// can be told, as in Redis.
	if target == tc.id || c.subscriber.TotalSubscriptions() == 0 {
		return
	}
	c.subscriber.Send(invalidationMessage(keys))
}

func appendBulkString(b []byte, s string) []byte {
	b = append(b, '$')
	b = strconv.AppendInt(b, int64(len(s)), 10)
	b = append(b, '\r', '\n')
	b = append(b, s...)
	return append(b, '\r', '\n')
}

func appendKeyList(b []byte, keys []string, resp3 bool) []byte {
	if keys == nil {
		if resp3 {
			return append(b, '_', '\r', '\n')
		}
		return append(b, "*-1\r\n"...)
	}
	b = append(b, '*')
	b = strconv.AppendInt(b, int64(len(keys)), 10)
	b = append(b, '\r', '\n')
	for _, k := range keys {
		b = appendBulkString(b, k)
	}
	return b
}

// invalidationPush is the RESP3 push ["invalidate", keys].
func invalidationPush(keys []string) []byte {
	b := append([]byte(">2\r\n"), "$10\r\ninvalidate\r\n"...)
	return appendKeyList(b, keys, true)
}

// invalidationMessage is the RESP2 pub/sub message on __redis__:invalidate
// that a REDIRECT target receives.
func invalidationMessage(keys []string) []byte {
	b := append([]byte("*3\r\n"), "$7\r\nmessage\r\n"...)
	b = appendBulkString(b, invalidateChannel)
	return appendKeyList(b, keys, false)
}

// trackCommand records the keys a tracking client's read-only command read.
// The caller still holds the write gate for the command.
func (s *Server) trackCommand(ctx *command.Context, failed bool) {
	if s == nil || s.tracker == nil || !s.tracker.active() {
		return
	}
	if failed || command.IsWriteCommand(ctx.Command) {
		s.tracker.clearCaching(ctx.ClientID)
		return
	}
	switch ctx.Command {
	case "CLIENT", "MULTI", "EXEC", "DISCARD", "WATCH", "UNWATCH":
		return
	}
	s.tracker.trackRead(ctx.ClientID, ctx.CommandKeys())
}

// invalidateTracked sends the invalidations a write of cmd causes.
func (s *Server) invalidateTracked(writer int64, cmd string, args [][]byte) {
	if s.tracker == nil || !s.tracker.active() {
		return
	}
	switch cmd {
	case "FLUSHALL", "FLUSHDB", "SWAPDB":
		s.tracker.invalidateAll(writer)
		return
	}
	if strings.HasPrefix(cmd, "FT.") {
		return
	}
	s.tracker.invalidate(writer, command.CommandKeys(cmd, args))
}
