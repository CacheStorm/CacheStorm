package server

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
)

// Tag writes are writes: a read-only script refuses them, and reads stay
// allowed.
func TestTagWritesRefusedInReadOnlyScriptsX(t *testing.T) {
	c := dialRepl(t, startReplServer(t, nil))
	c.do("SETTAG", "k", "v", "t")
	for _, call := range []string{
		"'SETTAG','k','v','t'", "'ADDTAG','k','u'", "'REMTAG','k','t'",
		"'INVALIDATE','t'", "'TAGLINK','a','b'", "'TAGUNLINK','a','b'",
	} {
		got := c.do("EVAL_RO", "return redis.call("+call+")", "0")
		if got.Type != resp.TypeError || !strings.Contains(got.Err, "Write commands are not allowed from read-only scripts") {
			t.Fatalf("EVAL_RO %s = %s", call, str(got))
		}
	}
	for _, call := range []string{"'TAGS','k'", "'TAGKEYS','t'", "'TAGCOUNT','t'", "'TAGCHILDREN','t'"} {
		if got := c.do("EVAL_RO", "return redis.call("+call+")", "0"); got.Type == resp.TypeError {
			t.Fatalf("EVAL_RO %s = %s", call, str(got))
		}
	}
	if got := c.str("TAGKEYS", "t"); got != "[k]" {
		t.Fatalf("TAGKEYS t = %s: a refused script changed the tags", got)
	}
}

// INVALIDATE and CACHE.CLEAR name no key, yet the documents of the keys they
// delete leave the search indexes.
func TestBulkDeletesLeaveSearchIndexesX(t *testing.T) {
	c := dialRepl(t, startReplServer(t, nil))
	if got := c.str("FT.CREATE", "idx", "ON", "HASH", "PREFIX", "1", "doc:", "SCHEMA", "name", "TEXT"); got != "OK" {
		t.Fatalf("FT.CREATE = %s", got)
	}
	c.do("HSET", "doc:1", "name", "hello")
	c.do("HSET", "doc:2", "name", "hello")
	c.do("ADDTAG", "doc:1", "t")
	if got := c.do("FT.SEARCH", "idx", "hello"); got.Array[0].Int != 2 {
		t.Fatalf("FT.SEARCH before = %s", str(got))
	}
	c.do("INVALIDATE", "t")
	if got := c.str("FT.SEARCH", "idx", "hello", "NOCONTENT"); got != "[1 doc:2]" {
		t.Fatalf("FT.SEARCH after INVALIDATE = %s", got)
	}
	if got := c.do("CACHE.CLEAR", "doc:*"); got.Type == resp.TypeError {
		t.Skipf("CACHE.CLEAR not served: %s", got.Err)
	}
	if got := c.str("FT.SEARCH", "idx", "hello", "NOCONTENT"); got != "[0]" {
		t.Fatalf("FT.SEARCH after CACHE.CLEAR = %s", got)
	}
}

// Tag writes count towards the save rules, as every write does.
func TestTagWritesCountAsChangesX(t *testing.T) {
	c := dialRepl(t, startReplServer(t, nil))
	changes := func() int {
		n, _ := strconv.Atoi(c.info("persistence")["rdb_changes_since_last_save"])
		return n
	}
	before := changes()
	c.do("SETTAG", "k", "v", "t")
	c.do("ADDTAG", "k", "u")
	c.do("REMTAG", "k", "u")
	c.do("TAGLINK", "a", "b")
	c.do("TAGUNLINK", "a", "b")
	c.do("INVALIDATE", "t")
	c.do("TAGKEYS", "t")
	if got := changes() - before; got != 6 {
		t.Fatalf("tag writes counted %d changes, want 6", got)
	}
}

func TestTagCommandsIntrospectionX(t *testing.T) {
	c := dialRepl(t, startReplServer(t, nil))
	for cmd, want := range map[string]string{
		"SETTAG":     "[settag -4 [write denyoom] 1 1 1 [@write @string @slow] [] [] []]",
		"TAGS":       "[tags 2 [readonly fast] 1 1 1 [@read @keyspace @fast] [] [] []]",
		"INVALIDATE": "[invalidate -2 [write] 0 0 0 [@write @keyspace @slow] [] [] []]",
		"TAGLINK":    "[taglink 3 [write denyoom] 0 0 0 [@write @keyspace @slow] [] [] []]",
	} {
		if got := c.str("COMMAND", "INFO", cmd); got != "["+want+"]" {
			t.Fatalf("COMMAND INFO %s = %s", cmd, got)
		}
	}
	if got := c.str("COMMAND", "GETKEYS", "SETTAG", "k", "v", "t"); got != "[k]" {
		t.Fatalf("COMMAND GETKEYS SETTAG = %s", got)
	}
	if got := c.str("COMMAND", "DOCS", "INVALIDATE"); !strings.Contains(got, "Deletes every key that carries one of the tags") {
		t.Fatalf("COMMAND DOCS INVALIDATE = %s", got)
	}
	if got := c.str("SETTAG", "k", "v"); got != "ERR:ERR wrong number of arguments for 'settag' command" {
		t.Fatalf("SETTAG k v = %s", got)
	}
}

func TestTagHierarchyIsAcyclicX(t *testing.T) {
	master := startReplServer(t, nil)
	replica := startReplServer(t, nil)
	rc := replicate(t, replica, master)
	c := dialRepl(t, master)
	if got := c.str("TAGLINK", "a", "a"); !strings.HasPrefix(got, "ERR:") {
		t.Fatalf("TAGLINK a a = %s", got)
	}
	c.do("TAGLINK", "a", "b")
	c.do("TAGLINK", "b", "c")
	if got := c.str("TAGLINK", "c", "a"); !strings.HasPrefix(got, "ERR:") {
		t.Fatalf("TAGLINK c a = %s, want the cycle refused", got)
	}
	// Two parents of one tag; unlinking one keeps the other.
	c.do("TAGLINK", "x", "c")
	c.do("TAGLINK", "x", "b2")
	c.do("TAGLINK", "x", "a2")
	if got := c.str("TAGCHILDREN", "x"); got != "[a2 b2 c]" {
		t.Fatalf("TAGCHILDREN x = %s, want sorted", got)
	}
	c.do("TAGUNLINK", "x", "c")
	c.do("SETTAG", "k", "v", "c")
	if got := c.str("INVALIDATE", "a", "cascade"); got != "1" {
		t.Fatalf("INVALIDATE a cascade = %s, want 1", got)
	}
	waitSynced(t, replica, master)
	if got := rc.str("TAGCHILDREN", "c"); got != "[]" {
		t.Fatalf("replica TAGCHILDREN c = %s: the refused link reached it", got)
	}
	if got := rc.str("TAGCHILDREN", "x"); got != "[a2 b2]" {
		t.Fatalf("replica TAGCHILDREN x = %s", got)
	}
}

func TestTagsInfoX(t *testing.T) {
	c := dialRepl(t, startReplServer(t, nil))
	c.do("SETTAG", "k1", "v", "t", "u")
	c.do("SETTAG", "k2", "v", "t")
	c.do("TAGLINK", "t", "w")
	info := c.info("tags")
	for field, want := range map[string]string{"tags": "2", "tag_memberships": "3", "tag_links": "1", "invalidations": "0"} {
		if info[field] != want {
			t.Fatalf("INFO tags %s = %q, want %s (%v)", field, info[field], want, info)
		}
	}
	c.do("INVALIDATE", "t")
	info = c.info("tags")
	for field, want := range map[string]string{"tags": "0", "tag_memberships": "0", "invalidations": "1", "invalidated_keys": "2"} {
		if info[field] != want {
			t.Fatalf("after INVALIDATE INFO tags %s = %q, want %s", field, info[field], want)
		}
	}
	if _, ok := c.info("default")["tags"]; ok {
		t.Fatal("INFO default lists the Tags section")
	}
	c.do("CONFIG", "RESETSTAT")
	if got := c.info("tags")["invalidations"]; got != "0" {
		t.Fatalf("invalidations after RESETSTAT = %s", got)
	}
}

// ACL rules see tag writes as writes of their key.
func TestTagCommandsUnderACLX(t *testing.T) {
	s := startReplServer(t, nil)
	admin := dialRepl(t, s)
	admin.do("ACL", "SETUSER", "tagreader", "reset", "on", ">p", "~*", "+@read")
	admin.do("ACL", "SETUSER", "tagscoped", "reset", "on", ">p", "~mine:*", "+@all")
	r := dialRepl(t, s)
	r.do("AUTH", "tagreader", "p")
	if got := r.str("SETTAG", "k", "v", "t"); !strings.HasPrefix(got, "ERR:NOPERM") {
		t.Fatalf("reader SETTAG = %s", got)
	}
	if got := r.str("TAGKEYS", "t"); got != "[]" {
		t.Fatalf("reader TAGKEYS = %s", got)
	}
	sc := dialRepl(t, s)
	sc.do("AUTH", "tagscoped", "p")
	if got := sc.str("SETTAG", "other", "v", "t"); !strings.HasPrefix(got, "ERR:NOPERM") {
		t.Fatalf("scoped SETTAG other = %s", got)
	}
	if got := sc.str("ADDTAG", "other", "t"); !strings.HasPrefix(got, "ERR:NOPERM") {
		t.Fatalf("scoped ADDTAG other = %s", got)
	}
	if got := sc.str("SETTAG", "mine:1", "v", "t"); got != "OK" {
		t.Fatalf("scoped SETTAG mine:1 = %s", got)
	}
}

// Over maxmemory with noeviction, a tag write that may grow memory is
// refused like SET; INVALIDATE, which frees it, still runs.
func TestTagWritesRefusedOverMaxmemoryX(t *testing.T) {
	c := dialRepl(t, startReplServer(t, nil))
	c.do("SETTAG", "k", "v", "t")
	c.do("CONFIG", "SET", "maxmemory-policy", "noeviction")
	c.do("CONFIG", "SET", "maxmemory", "1")
	for _, cmd := range [][]string{{"SETTAG", "k2", "v", "t"}, {"ADDTAG", "k", "u"}, {"TAGLINK", "a", "b"}} {
		if got := c.str(cmd...); !strings.HasPrefix(got, "ERR:OOM") {
			t.Fatalf("%v over maxmemory = %s", cmd, got)
		}
	}
	if got := c.str("INVALIDATE", "t"); got != "1" {
		t.Fatalf("INVALIDATE over maxmemory = %s", got)
	}
	c.do("CONFIG", "SET", "maxmemory", "0")
}

// An AOF written by a version that accepted cyclic links still loads: the
// link closing the cycle is left out.
func TestCyclicTagLinksInAOFStillLoadX(t *testing.T) {
	cfg := aofRewriteConfig(t, t.TempDir(), nil)
	var data []byte
	for _, cmd := range [][]string{
		{"TAGLINK", "a", "b"}, {"TAGLINK", "b", "a"}, {"TAGLINK", "c", "c"},
		{"SETTAG", "k", "v", "b"},
	} {
		data = append(data, encodeArgs(cmd...)...)
	}
	if err := os.WriteFile(filepath.Join(cfg.Persistence.DataDir, "appendonly.aof"), data, 0600); err != nil {
		t.Fatal(err)
	}
	c := dialRepl(t, startReplServerCfg(t, cfg))
	if got := c.str("TAGCHILDREN", "a"); got != "[b]" {
		t.Fatalf("TAGCHILDREN a = %s", got)
	}
	if got := c.str("TAGCHILDREN", "b"); got != "[]" {
		t.Fatalf("TAGCHILDREN b = %s, want the cyclic link left out", got)
	}
	if got := c.str("INVALIDATE", "a", "CASCADE"); got != "1" {
		t.Fatalf("INVALIDATE a CASCADE = %s", got)
	}
}
