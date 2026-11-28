package server

import (
	"bufio"
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
)

var trackKeys = []string{"t0", "t1", "t2", "t3", "t4", "t5"}

const trackNil = "<nil>"

// trackingReader is a client-side cache over a RESP3 connection with
// tracking on: it caches what GET answered and drops what an invalidation
// names, the way client libraries do.
type trackingReader struct {
	c     *watchClient
	mode  string
	cache map[string]string
	// reads and drops count the values cached and dropped.
	reads, drops int
}

func dialTrackingReader(s *Server, mode string) (*trackingReader, error) {
	conn, err := net.DialTimeout("tcp", "127.0.0.1:"+strconv.Itoa(s.testPort()), 2*time.Second)
	if err != nil {
		return nil, err
	}
	r := &trackingReader{c: &watchClient{conn: conn, rd: resp.NewReader(bufio.NewReader(conn))}, mode: mode, cache: map[string]string{}}
	if _, err := r.do("HELLO", "3"); err != nil {
		return nil, err
	}
	args := []string{"CLIENT", "TRACKING", "ON"}
	switch mode {
	case "bcast":
		args = append(args, "BCAST", "PREFIX", "t")
	case "optin":
		args = append(args, "OPTIN")
	}
	if v, err := r.do(args...); err != nil || v.Type == resp.TypeError {
		return nil, fmt.Errorf("%v: %v %v", args, v, err)
	}
	return r, nil
}

// reply reads the next reply, applying the invalidations that come first.
func (r *trackingReader) reply() (*resp.Value, error) {
	for {
		_ = r.c.conn.SetDeadline(time.Now().Add(20 * time.Second))
		v, err := r.c.rd.ReadValue()
		if err != nil {
			return nil, err
		}
		if v.Type != resp.TypePush {
			return v, nil
		}
		if len(v.Array) != 2 || string(v.Array[0].Bulk) != "invalidate" {
			return nil, fmt.Errorf("unexpected push %s", str(v))
		}
		keys := v.Array[1]
		if keys.Type == resp.TypeNull || keys.IsNull {
			r.drops += len(r.cache)
			r.cache = map[string]string{}
			continue
		}
		for _, k := range keys.Array {
			if _, ok := r.cache[string(k.Bulk)]; ok {
				r.drops++
				delete(r.cache, string(k.Bulk))
			}
		}
	}
}

func (r *trackingReader) do(args ...string) (*resp.Value, error) {
	_ = r.c.conn.SetDeadline(time.Now().Add(20 * time.Second))
	if _, err := r.c.conn.Write(encodeArgs(args...)); err != nil {
		return nil, err
	}
	return r.reply()
}

// get reads key and caches the value.
func (r *trackingReader) get(key string) error {
	if r.mode == "optin" {
		if _, err := r.do("CLIENT", "CACHING", "yes"); err != nil {
			return err
		}
	}
	v, err := r.do("GET", key)
	if err != nil {
		return err
	}
	r.reads++
	switch {
	case v.Type == resp.TypeError:
		return fmt.Errorf("GET %s: %s", key, v.Err)
	case v.Type == resp.TypeNull || v.IsNull:
		r.cache[key] = trackNil
	default:
		r.cache[key] = string(v.Bulk)
	}
	return nil
}

// trackWriteCommand returns a write to the tracked keys: plain and tagged
// writes, short expiries, INVALIDATE, renames and copies, transactions,
// scripts, and now and then a FLUSHDB or a SWAPDB.
func trackWriteCommand(rng *rand.Rand, big bool) [][]string {
	k := trackKeys[rng.Intn(len(trackKeys))]
	k2 := trackKeys[rng.Intn(len(trackKeys))]
	v := strconv.Itoa(rng.Intn(1000))
	if big && rng.Intn(2) == 0 {
		v += strings.Repeat("x", 20_000+rng.Intn(20_000))
	}
	tag := "tg" + strconv.Itoa(rng.Intn(3))
	switch rng.Intn(22) {
	case 0, 1:
		return [][]string{{"SET", k, v}}
	case 2:
		return [][]string{{"SET", k, v, "TAGS", tag}}
	case 3:
		return [][]string{{"SETTAG", k, v, tag}}
	case 4:
		return [][]string{{"INVALIDATE", tag}}
	case 5:
		return [][]string{{"DEL", k}}
	case 6:
		return [][]string{{"APPEND", k, v}}
	case 7:
		return [][]string{{"SET", k, v, "PX", strconv.Itoa(1 + rng.Intn(5))}}
	case 8:
		return [][]string{{"PEXPIRE", k, strconv.Itoa(1 + rng.Intn(5))}}
	case 9:
		return [][]string{{"RENAME", k, k2}}
	case 10:
		return [][]string{{"COPY", k, k2, "REPLACE"}}
	case 11:
		return [][]string{{"MULTI"}, {"SET", k, v}, {"DEL", k2}, {"EXEC"}}
	case 12:
		return [][]string{{"EVAL", "redis.call('SET', KEYS[1], ARGV[1]); return redis.call('DEL', KEYS[2])", "2", k, k2, v}}
	case 13:
		return [][]string{{"MSET", k, v, k2, v}}
	case 14:
		return [][]string{{"GETDEL", k}}
	case 15:
		return [][]string{{"GETEX", k, "PX", strconv.Itoa(1 + rng.Intn(5))}}
	case 16:
		return [][]string{{"SETRANGE", k, strconv.Itoa(rng.Intn(4)), v}}
	case 17:
		return [][]string{{"ADDTAG", k, tag}}
	case 18:
		return [][]string{{"INCR", k}}
	case 19:
		return [][]string{{"SET", k, v, "KEEPTTL", "GET"}}
	case 20:
		if rng.Intn(10) == 0 {
			return [][]string{{"FLUSHDB"}}
		}
		return [][]string{{"PERSIST", k}}
	default:
		if rng.Intn(5) == 0 {
			return [][]string{{"SWAPDB", "0", "1"}}
		}
		return [][]string{{"MOVE", k, "1"}}
	}
}

// Clients caching values under CLIENT TRACKING (default, BCAST and OPTIN)
// while others write, retag, invalidate, expire, rename, swap and flush the
// keys never keep a stale value: once the writes stop and the invalidations
// they caused are delivered, every cached value is the server's.
func TestTrackingCachesStayFreshX(t *testing.T) {
	t.Run("master", func(t *testing.T) {
		s := startReplServer(t, nil)
		runTrackingEpochs(t, s, s, false, nil)
	})
	// Readers on a replica are told of the writes the master sends it.
	t.Run("replica", func(t *testing.T) {
		master := startReplServer(t, nil)
		replica := startReplServer(t, nil)
		replicate(t, replica, master)
		runTrackingEpochs(t, replica, master, false, func() { waitSynced(t, replica, master) })
	})
	// Keys evicted under maxmemory are invalidated like deleted ones.
	t.Run("eviction", func(t *testing.T) {
		s := startReplServer(t, nil)
		admin := dialRepl(t, s)
		used, _ := strconv.ParseInt(admin.info("memory")["used_memory"], 10, 64)
		admin.do("CONFIG", "SET", "maxmemory-policy", "allkeys-lru")
		admin.do("CONFIG", "SET", "maxmemory", strconv.FormatInt(used+150_000, 10))
		before, _ := strconv.Atoi(admin.info("stats")["evicted_keys"])
		runTrackingEpochs(t, s, s, true, nil)
		if after, _ := strconv.Atoi(admin.info("stats")["evicted_keys"]); after == before {
			t.Fatal("nothing was evicted")
		}
	})
}

// runTrackingEpochs runs tracking readers on rs and writers on ws; synced,
// when set, waits until rs has every write ws made.
func runTrackingEpochs(t *testing.T, rs, ws *Server, big bool, synced func()) {
	s := rs
	var readers []*trackingReader
	for _, mode := range []string{"default", "default", "default", "bcast", "optin"} {
		r, err := dialTrackingReader(s, mode)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { r.c.conn.Close() })
		readers = append(readers, r)
	}
	var writersConns []*watchClient
	const writers, epochs, rounds = 4, 60, 12
	for id := 0; id < writers; id++ {
		conn, err := net.DialTimeout("tcp", "127.0.0.1:"+strconv.Itoa(ws.testPort()), 2*time.Second)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { conn.Close() })
		writersConns = append(writersConns, &watchClient{conn: conn, rd: resp.NewReader(bufio.NewReader(conn))})
	}
	plain, wplain := dialRepl(t, rs), dialRepl(t, ws)
	seed := time.Now().UnixNano()
	t.Logf("seed %d", seed)

	// Each epoch: readers fill their caches and keep reading while the
	// writers send a burst; then the writes stop, the invalidations are
	// taken in and every cached value must be the server's. A missed
	// invalidation shows here before a later write to the key hides it.
	for epoch := 0; epoch < epochs; epoch++ {
		for _, r := range readers {
			for _, k := range trackKeys {
				if err := r.get(k); err != nil {
					t.Fatal(err)
				}
			}
		}
		var (
			stop atomic.Bool
			wg   sync.WaitGroup
			rwg  sync.WaitGroup
			errs = make(chan error, writers+len(readers))
		)
		for i, r := range readers {
			rwg.Add(1)
			go func(r *trackingReader, rng *rand.Rand) {
				defer rwg.Done()
				for !stop.Load() {
					if err := r.get(trackKeys[rng.Intn(len(trackKeys))]); err != nil {
						errs <- err
						return
					}
				}
			}(r, rand.New(rand.NewSource(seed+int64(epoch*100+i))))
		}
		for i, c := range writersConns {
			wg.Add(1)
			go func(c *watchClient, rng *rand.Rand) {
				defer wg.Done()
				for n := 0; n < rounds; n++ {
					for _, cmd := range trackWriteCommand(rng, big) {
						if _, err := c.do(cmd...); err != nil {
							errs <- err
							return
						}
					}
				}
			}(c, rand.New(rand.NewSource(seed+int64(epoch*100+50+i))))
		}
		wg.Wait()
		stop.Store(true)
		rwg.Wait()
		close(errs)
		for err := range errs {
			t.Fatal(err)
		}

		// Let the short expiries pass and read every key, which expires
		// those still stored; then every reader takes in what it was sent.
		time.Sleep(10 * time.Millisecond)
		for _, k := range trackKeys {
			wplain.do("GET", k)
		}
		if synced != nil {
			synced()
		}
		for _, r := range readers {
			if v, err := r.do("PING"); err != nil || str(v) != "PONG" {
				t.Fatalf("PING: %v %v", str(v), err)
			}
		}
		want := map[string]string{}
		for _, k := range trackKeys {
			if v := plain.do("GET", k); v.IsNull || v.Type == resp.TypeNull {
				want[k] = trackNil
			} else {
				want[k] = string(v.Bulk)
			}
		}
		for i, r := range readers {
			for k, v := range r.cache {
				if want[k] != v {
					t.Errorf("epoch %d: reader %d (%s) caches %s = %.40q, the server has %.40q", epoch, i, r.mode, k, v, want[k])
				}
			}
		}
		if t.Failed() {
			t.FailNow()
		}
	}
	for i, r := range readers {
		t.Logf("reader %d (%s): %d values cached, %d dropped", i, r.mode, r.reads, r.drops)
	}
}

// A key removed while a tracking client's read of it runs (an expiry, an
// eviction: removals that do not wait for the read's stripe) reaches the
// client as an invalidation after the read, though the read registers the
// key only once it ran; a flush meanwhile invalidates every key it read.
func TestTrackingReportsAKeyRemovedDuringTheRead(t *testing.T) {
	s := startReplServer(t, nil)
	c := dialRepl(t, s)
	hello3(t, c)
	if got := c.str("CLIENT", "TRACKING", "ON"); got != "OK" {
		t.Fatalf("CLIENT TRACKING ON = %s", got)
	}
	id := c.do("CLIENT", "ID").Int

	s.tracker.beginRead(id, []string{"k"})
	s.tracker.invalidate(0, []string{"k"})
	s.tracker.trackRead(id, []string{"k"})
	if got := invalidated(t, c); len(got) != 1 || got[0] != "k" {
		t.Fatalf("invalidated %v, want [k]", got)
	}
	// The invalidated key is not registered: a later change tells nothing.
	s.tracker.invalidate(0, []string{"k"})
	noFrame(t, c)

	s.tracker.beginRead(id, []string{"a", "b"})
	s.tracker.invalidateAll(0)
	if got := invalidated(t, c); got != nil {
		t.Fatalf("a flush invalidated %v, want everything", got)
	}
	s.tracker.trackRead(id, []string{"a", "b"})
	if got := invalidated(t, c); len(got) != 2 {
		t.Fatalf("after a flush during the read, invalidated %v, want [a b]", got)
	}
}
