package server

import (
	"fmt"
	"testing"

	"github.com/cachestorm/cachestorm/internal/config"
	"github.com/cachestorm/cachestorm/internal/resp"
)

// seedTagHierarchy links p -> c in database 0 and q -> d in database 2, and
// tags keys with the children only.
func seedTagHierarchy(t *testing.T, c *replClient) {
	t.Helper()
	c.do("TAGLINK", "p", "c")
	c.do("TAGLINK", "gone", "c")
	c.do("TAGUNLINK", "gone", "c")
	for i := 0; i < 5; i++ {
		c.do("SETTAG", fmt.Sprintf("k%d", i), "v", "c")
	}
	c.do("SELECT", "2")
	c.do("TAGLINK", "q", "d")
	c.do("SETTAG", "x", "v", "d")
	c.do("SELECT", "0")
}

// checkTagHierarchy verifies seedTagHierarchy's links by invalidating the
// parents, which reach the keys only through their children.
func checkTagHierarchy(t *testing.T, c *replClient) {
	t.Helper()
	if got := c.str("TAGCHILDREN", "gone"); got != "[]" {
		t.Fatalf("unlinked parent came back with children %s", got)
	}
	if got := c.do("INVALIDATE", "p", "CASCADE"); got.Type != resp.TypeInteger || got.Int != 5 {
		t.Fatalf("INVALIDATE p CASCADE = %s, want 5", str(got))
	}
	c.do("SELECT", "2")
	if got := c.do("INVALIDATE", "q", "CASCADE"); got.Type != resp.TypeInteger || got.Int != 1 {
		t.Fatalf("INVALIDATE q CASCADE in db 2 = %s, want 1", str(got))
	}
	c.do("SELECT", "0")
}

func TestTagHierarchySurvivesSnapshotRestartX(t *testing.T) {
	cfg := replTestConfig(t, func(c *config.Config) {
		c.Persistence.Enabled = true
		c.Persistence.AOF = false
		c.Persistence.DataDir = t.TempDir()
		c.Persistence.SnapshotInterval = "0"
	})
	s := startReplServerCfg(t, cfg)
	c := dialRepl(t, s)
	seedTagHierarchy(t, c)
	if got := c.str("SAVE"); got != "OK" {
		t.Fatalf("SAVE = %s", got)
	}
	stopReplServer(s)
	checkTagHierarchy(t, dialRepl(t, startReplServerCfg(t, cfg)))
}

func TestTagHierarchySurvivesAOFRewriteX(t *testing.T) {
	cfg := aofRewriteConfig(t, t.TempDir(), func(c *config.Config) { c.Persistence.AOFRewritePercentage = 0 })
	s := startReplServerCfg(t, cfg)
	c := dialRepl(t, s)
	seedTagHierarchy(t, c)
	if got := c.do("BGREWRITEAOF"); got.Type != resp.TypeSimpleString {
		t.Fatalf("BGREWRITEAOF = %s", str(got))
	}
	waitAOFRewrites(t, s, 1)
	stopReplServer(s)
	checkTagHierarchy(t, dialRepl(t, startReplServerCfg(t, cfg)))
}

func TestTagHierarchyFullSyncX(t *testing.T) {
	master := startReplServer(t, nil)
	mc := dialRepl(t, master)
	seedTagHierarchy(t, mc)
	replica := startReplServer(t, nil)
	rc := replicate(t, replica, master)
	waitSynced(t, replica, master)
	if got := rc.str("TAGCHILDREN", "p"); got != "[c]" {
		t.Fatalf("replica TAGCHILDREN p = %s after a full sync", got)
	}
	// The master invalidates through the hierarchy; the replica must end
	// with the same keys.
	checkTagHierarchy(t, mc)
	waitSynced(t, replica, master)
	if want, got := dumpAll(t, dialRepl(t, master)), dumpAll(t, rc); fmt.Sprint(want) != fmt.Sprint(got) {
		t.Fatalf("replica diverged\nmaster:  %v\nreplica: %v", want, got)
	}
}
