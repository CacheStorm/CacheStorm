package server

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/config"
	"github.com/cachestorm/cachestorm/internal/resp"
)

// seedSearch builds search state through every FT writer, a transaction and a
// script, ending with two documents in "articles", reachable as "news".
func seedSearch(t *testing.T, c *replClient) {
	t.Helper()
	for _, cmd := range [][]string{
		{"FT.CREATE", "articles", "SCHEMA", "title", "TEXT", "SORTABLE", "body", "TEXT", "tags", "TAG", "NOINDEX"},
		{"FT.CREATE", "scratch", "SCHEMA", "x", "TEXT"},
		{"FT.ADD", "articles", "doc1", "1", "FIELDS", "title", "stale words", "body", "obsolete"},
		{"FT.ADD", "articles", "doc1", "1", "REPLACE", "FIELDS", "title", "hello world", "body", "first\r\nbody", "tags", "a,b"},
		{"FT.ADD", "articles", "gone", "1", "FIELDS", "title", "hello gone"},
		{"FT.DEL", "articles", "gone"},
		{"FT.ALIASADD", "news", "articles"},
		{"FT.ALIASADD", "old", "scratch"},
		{"FT.ALIASUPDATE", "old", "articles"},
		{"FT.ALIASADD", "tmp", "articles"},
		{"FT.ALIASDEL", "tmp"},
		{"FT.DROPINDEX", "scratch"},
	} {
		if got := c.str(cmd...); got != "OK" && got != "1" {
			t.Fatalf("%v = %s", cmd, got)
		}
	}
	c.do("MULTI")
	c.do("FT.ADD", "news", "doc2", "0.5", "FIELDS", "title", "hello there")
	if got := c.str("EXEC"); got != "[OK]" {
		t.Fatalf("EXEC = %s", got)
	}
	if got := c.str("EVAL", "return redis.call('FT.ADD', 'articles', 'doc3', '1', 'FIELDS', 'title', 'scripted hello')", "0"); got != "OK" {
		t.Fatalf("EVAL FT.ADD = %s", got)
	}
	if got := c.str("FT.DEL", "articles", "doc3"); got != "1" {
		t.Fatalf("FT.DEL doc3 = %s", got)
	}
}

// seedDocuments writes seedSearch's documents again, into indexes that
// already exist.
func seedDocuments(t *testing.T, c *replClient) {
	t.Helper()
	for _, cmd := range [][]string{
		{"FT.ADD", "articles", "doc1", "1", "FIELDS", "title", "hello world", "body", "first\r\nbody", "tags", "a,b"},
		{"FT.ADD", "news", "doc2", "0.5", "FIELDS", "title", "hello there"},
	} {
		if got := c.str(cmd...); got != "OK" {
			t.Fatalf("%v = %s", cmd, got)
		}
	}
}

func searchIDs(t *testing.T, c *replClient, index, query string) string {
	t.Helper()
	v := c.do("FT.SEARCH", index, query, "NOCONTENT")
	if v.Type != resp.TypeArray || len(v.Array) == 0 {
		t.Fatalf("FT.SEARCH %s %s = %s", index, query, str(v))
	}
	ids := make([]string, 0, len(v.Array)-1)
	for _, e := range v.Array[1:] {
		ids = append(ids, str(e))
	}
	sort.Strings(ids)
	return strings.Join(ids, ",")
}

// checkSearch verifies the state seedSearch built.
func checkSearch(t *testing.T, c *replClient) {
	t.Helper()
	if got := c.str("FT._LIST"); got != "[articles]" {
		t.Fatalf("FT._LIST = %s", got)
	}
	for _, index := range []string{"articles", "news", "old"} {
		if got := searchIDs(t, c, index, "hello"); got != "doc1,doc2" {
			t.Fatalf("FT.SEARCH %s hello = %s, want doc1,doc2", index, got)
		}
	}
	if got := searchIDs(t, c, "articles", "stale"); got != "" {
		t.Fatalf("replaced document still matches its old title: %s", got)
	}
	if got := searchIDs(t, c, "articles", "first"); got != "doc1" {
		t.Fatalf("FT.SEARCH first = %s", got)
	}
	if got := c.do("FT.GET", "articles", "doc1"); got.Type != resp.TypeArray || !strings.Contains(sortedPairs(got), "body=first\r\nbody") {
		t.Fatalf("FT.GET doc1 = %s", str(got))
	}
	for _, missing := range []string{"gone", "doc3"} {
		if got := c.str("FT.GET", "articles", missing); got != "<null>" {
			t.Fatalf("deleted document %s came back: %s", missing, got)
		}
	}
	if got := c.do("FT.SEARCH", "tmp", "hello"); got.Type != resp.TypeError {
		t.Fatalf("deleted alias still resolves: %s", str(got))
	}
	if got := c.do("FT.INFO", "scratch"); got.Type != resp.TypeError {
		t.Fatalf("dropped index came back: %s", str(got))
	}
	if got := c.str("FT.TAGVALS", "articles", "tags"); got != "[a b]" {
		t.Fatalf("FT.TAGVALS = %s", got)
	}
}

func TestSearchSurvivesAOFRestart(t *testing.T) {
	cfg := aofRewriteConfig(t, t.TempDir(), nil)
	s := startReplServerCfg(t, cfg)
	seedSearch(t, dialRepl(t, s))
	checkSearch(t, dialRepl(t, s))
	stopReplServer(s)

	checkSearch(t, dialRepl(t, startReplServerCfg(t, cfg)))
}

func TestSearchSurvivesAOFRewrite(t *testing.T) {
	dir := t.TempDir()
	cfg := aofRewriteConfig(t, dir, func(c *config.Config) { c.Persistence.AOFRewritePercentage = 0 })
	s := startReplServerCfg(t, cfg)
	c := dialRepl(t, s)
	seedSearch(t, c)
	if got := c.do("BGREWRITEAOF"); got.Type != resp.TypeSimpleString {
		t.Fatalf("BGREWRITEAOF = %s", str(got))
	}
	waitAOFRewrites(t, s, 1)
	data, err := os.ReadFile(filepath.Join(dir, "appendonly.aof"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "FT.CREATE") || strings.Contains(string(data), "FT.DEL") {
		t.Fatalf("rewritten AOF does not hold the compacted search state:\n%q", data)
	}
	if got := c.str("FT.ADD", "news", "doc4", "1", "FIELDS", "title", "after rewrite"); got != "OK" {
		t.Fatalf("FT.ADD after rewrite = %s", got)
	}
	stopReplServer(s)

	r := dialRepl(t, startReplServerCfg(t, cfg))
	if got := searchIDs(t, r, "articles", "rewrite"); got != "doc4" {
		t.Fatalf("write after the rewrite lost: %s", got)
	}
	if got := r.str("FT.DEL", "articles", "doc4"); got != "1" {
		t.Fatal(got)
	}
	checkSearch(t, r)
}

func TestSearchSurvivesSnapshotRestart(t *testing.T) {
	cfg := replTestConfig(t, func(c *config.Config) {
		c.Persistence.Enabled = true
		c.Persistence.AOF = false
		c.Persistence.DataDir = t.TempDir()
		c.Persistence.SnapshotInterval = "0"
	})
	s := startReplServerCfg(t, cfg)
	c := dialRepl(t, s)
	seedSearch(t, c)
	if got := c.str("SAVE"); got != "OK" {
		t.Fatalf("SAVE = %s", got)
	}
	stopReplServer(s)

	checkSearch(t, dialRepl(t, startReplServerCfg(t, cfg)))
}

func TestSearchSnapshotSeedsAOF(t *testing.T) {
	dir := t.TempDir()
	snapCfg := replTestConfig(t, func(c *config.Config) {
		c.Persistence.Enabled = true
		c.Persistence.DataDir = dir
		c.Persistence.SnapshotInterval = "0"
	})
	s := startReplServerCfg(t, snapCfg)
	c := dialRepl(t, s)
	seedSearch(t, c)
	if got := c.str("SAVE"); got != "OK" {
		t.Fatalf("SAVE = %s", got)
	}
	stopReplServer(s)

	cfg := aofRewriteConfig(t, dir, nil)
	stopReplServer(startReplServerCfg(t, cfg))
	if err := os.Remove(filepath.Join(dir, "dump.rdb")); err != nil {
		t.Fatal(err)
	}
	checkSearch(t, dialRepl(t, startReplServerCfg(t, cfg)))
}

func TestSearchReplicatesLiveAndRefusesReplicaWrites(t *testing.T) {
	master := startReplServer(t, nil)
	replica := startReplServer(t, nil)
	r := replicate(t, replica, master)
	seedSearch(t, dialRepl(t, master))
	waitSynced(t, replica, master)
	checkSearch(t, r)
	if got := r.do("FT.ADD", "articles", "local", "1", "FIELDS", "title", "hello"); got.Type != resp.TypeError || !strings.HasPrefix(got.Err, "READONLY") {
		t.Fatalf("replica accepted FT.ADD: %s", str(got))
	}
	if got := r.do("FT.CREATE", "local"); got.Type != resp.TypeError || !strings.HasPrefix(got.Err, "READONLY") {
		t.Fatalf("replica accepted FT.CREATE: %s", str(got))
	}
}

func TestSearchFullSyncReplacesReplicaStateAndPersists(t *testing.T) {
	master := startReplServer(t, nil)
	seedSearch(t, dialRepl(t, master))
	cfg := aofRewriteConfig(t, t.TempDir(), nil)
	replica := startReplServerCfg(t, cfg)
	r := dialRepl(t, replica)
	r.do("FT.CREATE", "stale")
	r.do("FT.ADD", "stale", "x", "1", "FIELDS", "title", "hello")
	r = replicate(t, replica, master)
	waitSynced(t, replica, master)
	checkSearch(t, r)
	stopReplServer(replica)

	checkSearch(t, dialRepl(t, startReplServerCfg(t, cfg)))
}

// FLUSHALL deletes the documents, which are keys, and keeps the index
// definitions, as in Redis.
func TestSearchSurvivesFlushAll(t *testing.T) {
	cfg := aofRewriteConfig(t, t.TempDir(), nil)
	s := startReplServerCfg(t, cfg)
	c := dialRepl(t, s)
	seedSearch(t, c)
	c.do("SET", "k", "v")
	if got := c.str("FLUSHALL"); got != "OK" {
		t.Fatal(got)
	}
	if got := c.do("FT.SEARCH", "articles", "hello", "NOCONTENT"); len(got.Array) != 1 || got.Array[0].Int != 0 {
		t.Fatalf("FT.SEARCH after FLUSHALL = %+v, want no documents", got)
	}
	stopReplServer(s)

	r := dialRepl(t, startReplServerCfg(t, cfg))
	if got := r.str("EXISTS", "k"); got != "0" {
		t.Fatalf("flushed key came back: %s", got)
	}
	seedDocuments(t, r)
	checkSearch(t, r)
}
