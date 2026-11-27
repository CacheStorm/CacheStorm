package server

import (
	"math/rand"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/config"
)

var (
	blockLists = []string{"q0", "q1", "q2"}
	blockZSets = []string{"z0", "z1"}
)

// blockProducerCommand is a write a producer sends: pushes, adds and stream
// entries for the consumers to take, and now and then a write that takes the
// data away or changes its type under a blocked consumer.
func blockProducerCommand(rng *rand.Rand, n *int) []string {
	*n++
	list := blockLists[rng.Intn(len(blockLists))]
	other := blockLists[rng.Intn(len(blockLists))]
	zset := blockZSets[rng.Intn(len(blockZSets))]
	v := strconv.Itoa(*n)
	switch rng.Intn(24) {
	case 0, 1, 2, 3:
		return []string{"RPUSH", list, v, v + "b"}
	case 4, 5:
		return []string{"LPUSH", list, v}
	case 6, 7, 8:
		return []string{"ZADD", zset, strconv.Itoa(rng.Intn(50)), "m" + v}
	case 9, 10, 11:
		return []string{"XADD", "s", "*", "f", v}
	case 12:
		return []string{"DEL", list}
	case 13:
		return []string{"SET", list, "str"}
	case 14:
		return []string{"PEXPIRE", list, strconv.Itoa(1 + rng.Intn(5))}
	case 15:
		return []string{"RENAME", list, other}
	case 16:
		return []string{"LMOVE", list, other, "LEFT", "RIGHT"}
	case 17:
		return []string{"XGROUP", "CREATE", "s", "g", "0", "MKSTREAM"}
	case 18:
		return []string{"XTRIM", "s", "MAXLEN", "20"}
	case 19:
		return []string{"ZREM", zset, "m" + strconv.Itoa(rng.Intn(*n+1))}
	case 20:
		return []string{"SET", zset, "str"}
	case 21:
		return []string{"DEL", zset}
	case 22:
		return []string{"EVAL", "redis.call('RPUSH', KEYS[1], ARGV[1]) return redis.call('LPOP', KEYS[2])", "2", list, other, v}
	}
	return []string{"XDEL", "s", "0-1"}
}

// blockConsumerCommand is a blocking read a consumer waits in, with a short
// timeout so that some time out and some are served.
func blockConsumerCommand(rng *rand.Rand, id int) []string {
	list := blockLists[rng.Intn(len(blockLists))]
	other := blockLists[rng.Intn(len(blockLists))]
	timeout := "0.0" + strconv.Itoa(1+rng.Intn(4))
	switch rng.Intn(10) {
	case 0, 1:
		return []string{"BLPOP", "q0", "q1", "q2", timeout}
	case 2:
		return []string{"BRPOP", "q2", "q1", timeout}
	case 3:
		return []string{"BLMOVE", list, other, "RIGHT", "LEFT", timeout}
	case 4:
		return []string{"BRPOPLPUSH", list, other, timeout}
	case 5:
		return []string{"BLMPOP", timeout, "3", "q1", "q0", "q2", "LEFT", "COUNT", "2"}
	case 6:
		return []string{"BZPOPMIN", "z0", "z1", timeout}
	case 7:
		return []string{"BZMPOP", timeout, "2", "z1", "z0", "MAX", "COUNT", "2"}
	case 8:
		return []string{"XREADGROUP", "GROUP", "g", "c" + strconv.Itoa(id), "COUNT", "2", "BLOCK", strconv.Itoa(10 + rng.Intn(30)), "STREAMS", "s", ">"}
	}
	return []string{"XACK", "s", "g", "0-1"}
}

// streamGroupState is what a replica and an AOF load must agree on about the
// stream's consumer group: its position, its pending entries and who holds
// them.
func streamGroupState(c *replClient) string {
	var b strings.Builder
	for db := 0; db < 2; db++ {
		c.str("SELECT", strconv.Itoa(db))
		if c.str("TYPE", "s") != "stream" {
			continue
		}
		for _, g := range c.do("XINFO", "GROUPS", "s").Array {
			fields := map[string]string{}
			for i := 0; i+1 < len(g.Array); i += 2 {
				fields[string(g.Array[i].Bulk)] = str(g.Array[i+1])
			}
			b.WriteString("db" + strconv.Itoa(db) + " " + fields["name"] + " last=" + fields["last-delivered-id"] +
				" read=" + fields["entries-read"] + " pending=" + fields["pending"] + " ")
			b.WriteString(str(c.do("XPENDING", "s", fields["name"])) + "; ")
		}
	}
	c.str("SELECT", "0")
	return b.String()
}

// Producers fill lists, sorted sets and a stream while consumers wait on
// them in blocking pops, moves and group reads, some woken, some timing out,
// some finding the key deleted, retyped, renamed or swapped away. A second
// replica attaches with a full sync and the AOF is rewritten meanwhile. Every
// replica and an AOF load end with the master's data and group state.
func TestConcurrentBlockingCommandsReplicateX(t *testing.T) {
	cfg := aofRewriteConfig(t, t.TempDir(), func(c *config.Config) { c.Persistence.AOFRewritePercentage = 0 })
	master := startReplServerCfg(t, cfg)
	replica := startReplServer(t, nil)
	replicate(t, replica, master)
	late := startReplServer(t, nil)
	dialRepl(t, master).do("XGROUP", "CREATE", "s", "g", "0", "MKSTREAM")

	var producing sync.WaitGroup
	var consuming sync.WaitGroup
	var done atomic.Bool
	errs := make(chan error, 16)
	for id := 0; id < 4; id++ {
		producing.Add(1)
		go func(id int) {
			defer producing.Done()
			rng := rand.New(rand.NewSource(int64(id)))
			c := dialRepl(t, master)
			n := id * 1_000_000
			for r := 0; r < 120; r++ {
				var cmds [][]string
				multi := rng.Intn(4) == 0
				if multi {
					cmds = append(cmds, []string{"MULTI"})
				}
				for i := 0; i < 6; i++ {
					cmds = append(cmds, blockProducerCommand(rng, &n))
				}
				if multi {
					cmds = append(cmds, []string{"EXEC"})
				}
				if rng.Intn(40) == 0 {
					cmds = append(cmds, []string{"SWAPDB", "0", "1"})
				}
				var buf []byte
				for _, cmd := range cmds {
					buf = append(buf, encodeArgs(cmd...)...)
				}
				_ = c.conn.SetDeadline(time.Now().Add(20 * time.Second))
				if _, err := c.conn.Write(buf); err != nil {
					errs <- err
					return
				}
				for range cmds {
					if _, err := c.rd.ReadValue(); err != nil {
						errs <- err
						return
					}
				}
				time.Sleep(time.Duration(rng.Intn(2)) * time.Millisecond)
			}
		}(id)
	}
	for id := 0; id < 4; id++ {
		consuming.Add(1)
		go func(id int) {
			defer consuming.Done()
			rng := rand.New(rand.NewSource(int64(100 + id)))
			c := dialRepl(t, master)
			for !done.Load() {
				_ = c.conn.SetDeadline(time.Now().Add(20 * time.Second))
				if _, err := c.conn.Write(encodeArgs(blockConsumerCommand(rng, id)...)); err != nil {
					errs <- err
					return
				}
				if _, err := c.rd.ReadValue(); err != nil {
					errs <- err
					return
				}
			}
		}(id)
	}

	admin := dialRepl(t, master)
	time.Sleep(40 * time.Millisecond)
	admin.do("BGREWRITEAOF")
	replicate(t, late, master)
	waitAOFRewrites(t, master, 1)

	producing.Wait()
	done.Store(true)
	consuming.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond)

	mc := dialRepl(t, master)
	wantKeys := dumpAll(t, mc)
	wantGroups := streamGroupState(mc)
	for _, r := range []struct {
		name string
		s    *Server
	}{{"replica", replica}, {"late replica", late}} {
		waitSynced(t, r.s, master)
		c := dialRepl(t, r.s)
		if got := dumpAll(t, c); !reflect.DeepEqual(got, wantKeys) {
			t.Fatalf("%s keys differ\nmaster: %v\n%s: %v", r.name, wantKeys, r.name, got)
		}
		if got := streamGroupState(c); got != wantGroups {
			t.Fatalf("%s groups differ\nmaster: %s\n%s: %s", r.name, wantGroups, r.name, got)
		}
	}
	restarted := checkAOFRestart(t, master, cfg)
	rc := dialRepl(t, restarted)
	if got := dumpAll(t, rc); !reflect.DeepEqual(got, wantKeys) {
		t.Fatalf("keys after an AOF restart\nbefore: %v\nafter:  %v", wantKeys, got)
	}
	if got := streamGroupState(rc); got != wantGroups {
		t.Fatalf("groups after an AOF restart\nbefore: %s\nafter:  %s", wantGroups, got)
	}
}
