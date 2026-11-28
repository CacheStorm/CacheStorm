package server

import (
	"bufio"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"reflect"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/config"
	"github.com/cachestorm/cachestorm/internal/resp"
)

// watchClient is a replClient that reports errors instead of failing the
// test, so that it can run on a goroutine of its own.
type watchClient struct {
	conn net.Conn
	rd   *resp.Reader
}

func (c *watchClient) do(args ...string) (*resp.Value, error) {
	_ = c.conn.SetDeadline(time.Now().Add(20 * time.Second))
	if _, err := c.conn.Write(encodeArgs(args...)); err != nil {
		return nil, err
	}
	return c.rd.ReadValue()
}

// ok runs a command whose reply must not be an error.
func (c *watchClient) ok(args ...string) (*resp.Value, error) {
	v, err := c.do(args...)
	if err == nil && v.Type == resp.TypeError {
		err = fmt.Errorf("%v: %s", args, v.Err)
	}
	return v, err
}

var watchKeys = []string{"wa", "wb", "wc"}

// watchRound runs one randomly chosen step against c and returns how much it
// added to each counter. Optimistic steps read counters under WATCH and
// write them back plus one inside MULTI; the others increment directly,
// touch the counters without changing them, or swap the two databases.
func watchRound(c *watchClient, rng *rand.Rand) (map[string]int, error) {
	if _, err := c.ok("SELECT", strconv.Itoa(rng.Intn(2))); err != nil {
		return nil, err
	}
	k := watchKeys[rng.Intn(len(watchKeys))]
	one := map[string]int{k: 1}
	switch n := rng.Intn(20); {
	case n < 10:
		keys := []string{k}
		if n >= 7 {
			if k2 := watchKeys[rng.Intn(len(watchKeys))]; k2 != k {
				keys = append(keys, k2)
			}
		}
		if _, err := c.ok(append([]string{"WATCH"}, keys...)...); err != nil {
			return nil, err
		}
		next := make([]string, len(keys))
		for i, key := range keys {
			v, err := c.ok("GET", key)
			if err != nil {
				return nil, err
			}
			cur := 0
			if !v.IsNull {
				if cur, err = strconv.Atoi(string(v.Bulk)); err != nil {
					return nil, fmt.Errorf("GET %s: %q", key, v.Bulk)
				}
			}
			next[i] = strconv.Itoa(cur + 1)
		}
		if _, err := c.ok("MULTI"); err != nil {
			return nil, err
		}
		for i, key := range keys {
			var cmd []string
			switch rng.Intn(4) {
			case 0:
				cmd = []string{"SET", key, next[i]}
			case 1:
				cmd = []string{"SET", key, next[i], "KEEPTTL"}
			case 2:
				cmd = []string{"SET", key, next[i], "TAGS", "wt"}
			default:
				cmd = []string{"EVAL", "return redis.call('SET', KEYS[1], ARGV[1])", "1", key, next[i]}
			}
			if _, err := c.ok(cmd...); err != nil {
				return nil, err
			}
		}
		v, err := c.ok("EXEC")
		if err != nil {
			return nil, err
		}
		if v.IsNull {
			return nil, nil
		}
		for _, r := range v.Array {
			if r.Type == resp.TypeError {
				return nil, fmt.Errorf("EXEC of %v: %s", keys, r.Err)
			}
		}
		added := map[string]int{}
		for _, key := range keys {
			added[key]++
		}
		return added, nil
	case n < 12:
		_, err := c.ok("INCR", k)
		return one, err
	case n < 14:
		if _, err := c.ok("MULTI"); err != nil {
			return nil, err
		}
		if _, err := c.ok("INCR", k); err != nil {
			return nil, err
		}
		_, err := c.ok("EXEC")
		return one, err
	case n < 15:
		_, err := c.ok("EVAL", "return redis.call('INCR', KEYS[1])", "1", k)
		return one, err
	case n < 19:
		// Writes that leave the counter's value as it is.
		cmd := [][]string{{"EXPIRE", k, "1000"}, {"PERSIST", k}, {"ADDTAG", k, "wt"}, {"REMTAG", k, "wt"}}[n-15]
		_, err := c.ok(cmd...)
		return nil, err
	default:
		_, err := c.ok("SWAPDB", "0", "1")
		return nil, err
	}
}

// Clients incrementing the same counters through WATCH ... EXEC, plain
// INCR, transactions and scripts, while others expire, retag and swap the
// databases under them, lose no increment: a watched transaction commits
// only when nothing changed what it read. The replica and the AOF end with
// the master's counters.
func TestConcurrentWatchTransactionsX(t *testing.T) {
	cfg := aofRewriteConfig(t, t.TempDir(), func(c *config.Config) { c.Persistence.AOFRewritePercentage = 0 })
	master := startReplServerCfg(t, cfg)
	replica := startReplServer(t, nil)
	replicate(t, replica, master)

	const clients, rounds = 8, 250
	var (
		mu      sync.Mutex
		applied = map[string]int{}
		commits int
		wg      sync.WaitGroup
		errs    = make(chan error, clients)
	)
	for id := 0; id < clients; id++ {
		conn, err := net.DialTimeout("tcp", "127.0.0.1:"+strconv.Itoa(master.testPort()), 2*time.Second)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { conn.Close() })
		c := &watchClient{conn: conn, rd: resp.NewReader(bufio.NewReader(conn))}
		wg.Add(1)
		go func(seed int64) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(seed))
			for i := 0; i < rounds; i++ {
				added, err := watchRound(c, rng)
				if err != nil {
					errs <- err
					return
				}
				mu.Lock()
				for k, n := range added {
					applied[k] += n
				}
				if len(added) > 0 {
					commits++
				}
				mu.Unlock()
			}
		}(time.Now().UnixNano() + int64(id))
	}
	wg.Wait()
	close(errs)
	if err := errors.Join(func() (all []error) {
		for err := range errs {
			all = append(all, err)
		}
		return
	}()...); err != nil {
		t.Fatal(err)
	}

	// SWAPDB moves a counter between the databases, so each counter is the
	// sum of its value in both.
	mc := dialRepl(t, master)
	got := map[string]int{}
	for db := 0; db < 2; db++ {
		mc.str("SELECT", strconv.Itoa(db))
		for _, k := range watchKeys {
			if v := mc.do("GET", k); !v.IsNull {
				n, err := strconv.Atoi(string(v.Bulk))
				if err != nil {
					t.Fatalf("db %d %s = %q", db, k, v.Bulk)
				}
				got[k] += n
			}
		}
	}
	mc.str("SELECT", "0")
	for _, k := range watchKeys {
		if got[k] != applied[k] {
			t.Errorf("%s: counters add up to %d, but %d increments committed", k, got[k], applied[k])
		}
	}
	if t.Failed() {
		t.FailNow()
	}
	t.Logf("%d increments committed by %d steps", applied["wa"]+applied["wb"]+applied["wc"], commits)

	wantKeys := dumpAll(t, mc)
	waitSynced(t, replica, master)
	if got := dumpAll(t, dialRepl(t, replica)); !reflect.DeepEqual(got, wantKeys) {
		t.Fatalf("replica differs\n%s", mapDiff(wantKeys, got))
	}
	restarted := checkAOFRestart(t, master, cfg)
	if got := dumpAll(t, dialRepl(t, restarted)); !reflect.DeepEqual(got, wantKeys) {
		t.Fatalf("after an AOF restart\n%s", mapDiff(wantKeys, got))
	}
}

// A WATCH stays with its database index across SWAPDB, as in Redis: a key
// missing from both databases is not touched by the swap, and a later write
// to it in the watched database, not in the one the old data moved to,
// fails the transaction.
func TestWatchStaysWithItsDatabaseAcrossSwapdb(t *testing.T) {
	s := startReplServer(t, nil)
	c, other := dialRepl(t, s), dialRepl(t, s)
	run := func(writeDB string) string {
		c.str("FLUSHALL")
		c.str("SELECT", "0")
		c.str("WATCH", "x")
		other.str("SWAPDB", "0", "1")
		other.str("SELECT", writeDB)
		other.str("SET", "x", "1")
		c.str("MULTI")
		c.str("SET", "x", "2")
		return c.str("EXEC")
	}
	if got := run("0"); got != "<nullarray>" {
		t.Fatalf("a write to the watched database after SWAPDB: EXEC = %s, want it to fail", got)
	}
	if got := run("1"); got != "[OK]" {
		t.Fatalf("a write to the other database after SWAPDB: EXEC = %s, want [OK]", got)
	}
}
