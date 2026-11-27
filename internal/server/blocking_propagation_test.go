package server

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// A blocked pop woken by a transaction that fills two of its keys takes from
// the key filled first, which need not be the first it names. Recorded as
// itself, the replica and an AOF load ran it again over both keys filled and
// took from the first one named instead. It is recorded, as in Redis, as the
// plain pop of the key it took from.
func TestServedBlockingPopsRecordTheKeyTheyTook(t *testing.T) {
	for _, tc := range []struct {
		name  string
		cmd   []string
		fill  func(key string) []string
		plain string
	}{
		{"BLPOP", []string{"BLPOP", "k1", "k2", "5"}, func(k string) []string { return []string{"RPUSH", k, "a", "b"} }, "LPOP"},
		{"BRPOP", []string{"BRPOP", "k1", "k2", "5"}, func(k string) []string { return []string{"RPUSH", k, "a", "b"} }, "RPOP"},
		{"BLMPOP", []string{"BLMPOP", "5", "2", "k1", "k2", "LEFT", "COUNT", "1"}, func(k string) []string { return []string{"RPUSH", k, "a", "b"} }, "LPOP"},
		{"BZPOPMIN", []string{"BZPOPMIN", "k1", "k2", "5"}, func(k string) []string { return []string{"ZADD", k, "1", "a", "2", "b"} }, "ZPOPMIN"},
		{"BZMPOP", []string{"BZMPOP", "5", "2", "k1", "k2", "MAX"}, func(k string) []string { return []string{"ZADD", k, "1", "a", "2", "b"} }, "ZPOPMAX"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := aofRewriteConfig(t, t.TempDir(), nil)
			master := startReplServerCfg(t, cfg)
			replica := startReplServer(t, nil)
			replicate(t, replica, master)
			c := dialRepl(t, master)
			b := dialRepl(t, master)
			served := make(chan string, 1)
			go func() { served <- str(b.do(tc.cmd...)) }()
			eventually(t, 5*time.Second, "client blocked", func() bool {
				return strings.Contains(str(c.do("INFO", "clients")), "blocked_clients:1")
			})
			c.do("MULTI")
			c.do(tc.fill("k2")...)
			c.do(tc.fill("k1")...)
			c.do("EXEC")
			if got := <-served; !strings.Contains(got, "k2") {
				t.Fatalf("%s served %s, want k2", tc.name, got)
			}
			want := dumpAll(t, c)
			waitSynced(t, replica, master)
			if got := dumpAll(t, dialRepl(t, replica)); !reflect.DeepEqual(got, want) {
				t.Fatalf("replica %v, master %v", got, want)
			}
			restarted := checkAOFRestart(t, master, cfg)
			if got := dumpAll(t, dialRepl(t, restarted)); !reflect.DeepEqual(got, want) {
				t.Fatalf("after an AOF load %v, before %v", got, want)
			}
			aof, err := os.ReadFile(filepath.Join(cfg.Persistence.DataDir, "appendonly.aof"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(aof), tc.plain+"\r\n$2\r\nk2\r\n") {
				t.Fatalf("the AOF has no %s k2:\n%q", tc.plain, aof)
			}
			if strings.Contains(string(aof), tc.name) {
				t.Fatalf("the AOF recorded %s itself:\n%q", tc.name, aof)
			}
		})
	}
}

// Writes that change nothing are not recorded, as Redis propagates only what
// made the dataset dirty: deletes and pops of missing keys, removals of
// missing members, a timed-out blocking pop, an empty store over a missing
// destination, and a tag link made twice or removed when absent.
func TestAOFWritesThatChangeNothingAreNotRecorded(t *testing.T) {
	cfg := aofTxConfig(t)
	_, run := aofTxStart(t, cfg)
	run("RPUSH", "l", "a", "b")
	run("ZADD", "z", "1", "m")
	run("XADD", "x", "1-1", "f", "v")
	run("XGROUP", "CREATE", "x", "g", "0")
	run("XGROUP", "CREATECONSUMER", "x", "g", "c")
	run("TAGLINK", "p", "c")
	path := filepath.Join(cfg.Persistence.DataDir, "appendonly.aof")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"DEL", "missing"}, {"UNLINK", "missing", "other"},
		{"LPUSHX", "missing", "a"}, {"RPUSHX", "missing", "a"},
		{"LPOP", "missing"}, {"RPOP", "missing", "2"}, {"LPOP", "l", "0"},
		{"LMPOP", "1", "missing", "LEFT"}, {"ZMPOP", "1", "missing", "MIN"},
		{"ZPOPMIN", "missing"}, {"ZPOPMAX", "z", "0"},
		{"BLPOP", "missing", "0.01"}, {"BRPOP", "missing", "0.01"}, {"BLMPOP", "0.01", "1", "missing", "LEFT"},
		{"BLMOVE", "missing", "d", "LEFT", "LEFT", "0.01"}, {"BRPOPLPUSH", "missing", "d", "0.01"},
		{"BZPOPMIN", "missing", "0.01"}, {"BZMPOP", "0.01", "1", "missing", "MIN"},
		{"SREM", "missing", "a"}, {"ZREM", "missing", "a"}, {"HDEL", "missing", "f"},
		{"LREM", "missing", "0", "a"}, {"XDEL", "missing", "1-1"}, {"XTRIM", "missing", "MAXLEN", "1"},
		{"ZREMRANGEBYRANK", "missing", "0", "1"}, {"ZREMRANGEBYSCORE", "missing", "0", "1"},
		{"LTRIM", "missing", "0", "1"}, {"LTRIM", "l", "0", "-1"},
		{"LINSERT", "missing", "BEFORE", "a", "b"}, {"LINSERT", "l", "BEFORE", "zz", "b"},
		{"ZINTERSTORE", "dst", "1", "missing"}, {"ZUNIONSTORE", "dst", "1", "missing"},
		{"ZDIFFSTORE", "dst", "1", "missing"}, {"ZRANGESTORE", "dst", "missing", "0", "-1"},
		{"BITOP", "AND", "dst", "missing"},
		{"XGROUP", "DESTROY", "x", "nogroup"}, {"XGROUP", "DELCONSUMER", "x", "g", "nobody"},
		{"XGROUP", "CREATECONSUMER", "x", "g", "c"},
		{"TAGLINK", "p", "c"}, {"TAGUNLINK", "p", "other"},
	} {
		run(args[0], args[1:]...)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("writes that changed nothing were recorded:\n%q", after[len(before):])
	}
}

// XREADGROUP creates its consumer before it blocks. That was recorded only
// when the read ended, after a SWAPDB made while it waited, so the AOF and
// the replicas created the consumer in the other database (and an AOF load
// failed on it). It is recorded before the read lets the gate go, as Redis
// propagates it when the command blocks.
func TestBlockedXREADGROUPRecordsItsConsumerBeforeASwap(t *testing.T) {
	cfg := aofRewriteConfig(t, t.TempDir(), nil)
	master := startReplServerCfg(t, cfg)
	replica := startReplServer(t, nil)
	replicate(t, replica, master)
	c := dialRepl(t, master)
	b := dialRepl(t, master)
	c.do("XGROUP", "CREATE", "s", "g", "$", "MKSTREAM")
	served := make(chan string, 1)
	go func() {
		served <- str(b.do("XREADGROUP", "GROUP", "g", "reader", "BLOCK", "3000", "STREAMS", "s", ">"))
	}()
	eventually(t, 5*time.Second, "client blocked", func() bool {
		return strings.Contains(str(c.do("INFO", "clients")), "blocked_clients:1")
	})
	c.do("SWAPDB", "0", "1")
	<-served
	consumers := func(rc *replClient) string {
		rc.do("SELECT", "1")
		defer rc.do("SELECT", "0")
		return str(rc.do("XINFO", "CONSUMERS", "s", "g"))
	}
	want := consumers(c)
	if !strings.Contains(want, "reader") {
		t.Fatalf("the master's db 1 has no consumer reader: %s", want)
	}
	waitSynced(t, replica, master)
	if got := consumers(dialRepl(t, replica)); !strings.Contains(got, "reader") {
		t.Fatalf("the replica's db 1 has no consumer reader: %s", got)
	}
	restarted := checkAOFRestart(t, master, cfg)
	if got := consumers(dialRepl(t, restarted)); !strings.Contains(got, "reader") {
		t.Fatalf("after an AOF load db 1 has no consumer reader: %s", got)
	}
}
