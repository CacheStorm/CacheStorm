package server

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/config"
	"github.com/cachestorm/cachestorm/internal/store"
)

// rawHashFields returns the fields physically stored in the hash at key,
// without the lazy expiry a lookup applies.
func rawHashFields(st *store.Store, key string) []string {
	entry, ok := st.GetShard(key).Get(key)
	if !ok {
		return nil
	}
	h, ok := entry.Value.(*store.HashValue)
	if !ok {
		return nil
	}
	h.RLock()
	defer h.RUnlock()
	var out []string
	for f := range h.Fields {
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}

func checkFieldTTLs(t *testing.T, c *replClient, key string, want map[string]int64) {
	t.Helper()
	for f, at := range want {
		got := c.do("HPEXPIRETIME", key, "FIELDS", "1", f)
		if len(got.Array) != 1 || got.Array[0].Int != at {
			t.Fatalf("HPEXPIRETIME %s %s = %+v, want %d", key, f, got, at)
		}
	}
}

func TestHashFieldTTLsSurviveAOFRestartAndRewrite(t *testing.T) {
	dir := t.TempDir()
	cfg := aofRewriteConfig(t, dir, func(c *config.Config) { c.Persistence.AOFRewritePercentage = 0 })
	s := startReplServerCfg(t, cfg)
	c := dialRepl(t, s)
	c.do("HSET", "h", "abs", "1", "rel", "2", "plain", "3", "persisted", "4", "short", "5", "gone", "6")
	c.do("HPEXPIREAT", "h", "16725225600000", "FIELDS", "2", "abs", "persisted")
	c.do("HEXPIRE", "h", "1000", "FIELDS", "1", "rel")
	c.do("HPERSIST", "h", "FIELDS", "1", "persisted")
	c.do("HPEXPIRE", "h", "50", "FIELDS", "1", "short")
	c.do("HEXPIRE", "h", "0", "FIELDS", "1", "gone")
	c.do("HSETEX", "s", "PXAT", "16725225600001", "FIELDS", "1", "f", "v")
	rel := c.do("HPEXPIRETIME", "h", "FIELDS", "1", "rel").Array[0].Int
	eventually(t, 3*time.Second, "short field expired", func() bool {
		return len(rawHashFields(s.Store(), "h")) == 4
	})
	aof, err := os.ReadFile(filepath.Join(dir, "appendonly.aof"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(aof), "HDEL\r\n$1\r\nh\r\n$5\r\nshort") {
		t.Fatal("the active expiry of a field must be logged as HDEL")
	}
	if strings.Contains(string(aof), "HEXPIRE\r\n") {
		t.Fatal("HEXPIRE must be logged as HPEXPIREAT")
	}
	want := map[string]int64{"abs": 16725225600000, "rel": rel, "plain": -1, "persisted": -1, "short": -2, "gone": -2}

	stopReplServer(s)
	s = startReplServerCfg(t, cfg)
	c = dialRepl(t, s)
	checkFieldTTLs(t, c, "h", want)
	checkFieldTTLs(t, c, "s", map[string]int64{"f": 16725225600001})

	c.do("BGREWRITEAOF")
	waitAOFRewrites(t, s, 1)
	stopReplServer(s)
	s = startReplServerCfg(t, cfg)
	c = dialRepl(t, s)
	checkFieldTTLs(t, c, "h", want)
	checkFieldTTLs(t, c, "s", map[string]int64{"f": 16725225600001})
	if got := c.do("HLEN", "h"); got.Int != 4 {
		t.Fatalf("HLEN after rewrite = %+v", got)
	}
}

func TestHashFieldTTLsSurviveSnapshotRestart(t *testing.T) {
	dir := t.TempDir()
	cfg := snapshotConfig(t, dir)
	s := startReplServerCfg(t, cfg)
	c := dialRepl(t, s)
	c.do("HSET", "h", "a", "1", "b", "2", "c", "3")
	c.do("HPEXPIREAT", "h", "16725225600000", "FIELDS", "1", "a")
	c.do("HPEXPIRE", "h", "300", "FIELDS", "1", "c")
	c.do("HSET", "plain", "x", "y")
	c.do("HSET", "doomed", "x", "y")
	c.do("HPEXPIRE", "doomed", "300", "FIELDS", "1", "x")
	if got := c.do("SAVE"); got.Str != "OK" {
		t.Fatalf("SAVE = %+v", got)
	}
	stopReplServer(s)
	time.Sleep(350 * time.Millisecond)
	s = startReplServerCfg(t, cfg)
	c = dialRepl(t, s)
	checkFieldTTLs(t, c, "h", map[string]int64{"a": 16725225600000, "b": -1, "c": -2})
	if got := c.do("HGETALL", "plain"); len(got.Array) != 2 {
		t.Fatalf("plain hash = %+v", got)
	}
	if got := c.do("EXISTS", "doomed"); got.Int != 0 {
		t.Fatal("a hash whose fields all expired before the load must not come back")
	}
}

func TestHashFieldExpiryReplicatesAsHDEL(t *testing.T) {
	master := startReplServer(t, nil)
	mc := dialRepl(t, master)
	mc.do("HSET", "synced", "a", "1", "b", "2")
	mc.do("HPEXPIREAT", "synced", "16725225600000", "FIELDS", "1", "a")
	replica := startReplServer(t, nil)
	rc := replicate(t, replica, master)
	waitSynced(t, replica, master)
	checkFieldTTLs(t, rc, "synced", map[string]int64{"a": 16725225600000, "b": -1})

	master.dbs.StopExpiry()
	mc.do("HSET", "h", "a", "1", "b", "2")
	mc.do("HPEXPIRE", "h", "100", "FIELDS", "1", "a")
	waitSynced(t, replica, master)
	if got := rc.do("HTTL", "h", "FIELDS", "1", "a"); len(got.Array) != 1 || got.Array[0].Int <= 0 {
		t.Fatalf("replica HTTL = %+v", got)
	}
	time.Sleep(150 * time.Millisecond)
	if got := rc.do("HGET", "h", "a"); !got.IsNull {
		t.Fatalf("a replica must hide an expired field: %+v", got)
	}
	if got := rawHashFields(replica.Store(), "h"); len(got) != 2 {
		t.Fatalf("the replica removed the field itself: %v", got)
	}
	master.dbs.StartExpiry()
	eventually(t, 3*time.Second, "replica receives HDEL", func() bool {
		return len(rawHashFields(replica.Store(), "h")) == 1
	})
	if got := rawHashFields(master.Store(), "h"); len(got) != 1 {
		t.Fatalf("master fields %v", got)
	}

	mc.do("HPEXPIRE", "h", "50", "FIELDS", "1", "b")
	eventually(t, 3*time.Second, "replica drops the emptied hash", func() bool {
		return !replica.Store().GetShard("h").Exists("h")
	})
}

func TestHashFieldActiveExpiryWithoutReads(t *testing.T) {
	s := startReplServer(t, nil)
	c := dialRepl(t, s)
	for _, key := range []string{"h1", "h2", "h3"} {
		c.do("HSET", key, "gone", "1", "kept", "2")
		c.do("HPEXPIRE", key, "50", "FIELDS", "1", "gone")
	}
	c.do("HSET", "all", "x", "1")
	c.do("HPEXPIRE", "all", "50", "FIELDS", "1", "x")
	eventually(t, 3*time.Second, "fields expire without reads", func() bool {
		for _, key := range []string{"h1", "h2", "h3"} {
			if len(rawHashFields(s.Store(), key)) != 1 {
				return false
			}
		}
		return !s.Store().GetShard("all").Exists("all")
	})
	info := c.do("INFO", "stats")
	if !strings.Contains(string(info.Bulk), "expired_subkeys:") {
		t.Fatal("INFO stats must report expired_subkeys")
	}
}
