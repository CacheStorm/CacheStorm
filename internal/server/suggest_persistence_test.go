package server

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/config"
)

func seedSuggest(t *testing.T, c *replClient) {
	t.Helper()
	for _, cmd := range [][]string{
		{"FT.SUGADD", "ac", "hello", "2"},
		{"FT.SUGADD", "ac", "help", "3", "PAYLOAD", "pl"},
		{"FT.SUGADD", "ac", "hello world", "1.5", "INCR"},
		{"FT.SUGADD", "ac", "hello world", "1.5", "INCR"},
		{"FT.SUGADD", "ac", "gone", "1"},
		{"FT.SUGDEL", "ac", "gone"},
	} {
		c.do(cmd...)
	}
}

func checkSuggest(t *testing.T, c *replClient) {
	t.Helper()
	if got := c.str("TYPE", "ac"); got != "trietype0" {
		t.Fatalf("TYPE = %s", got)
	}
	if got := c.str("FT.SUGLEN", "ac"); got != "3" {
		t.Fatalf("FT.SUGLEN = %s", got)
	}
	want := "[help 1.7320507764816284 pl hello 1 <null> hello world 0.9486833214759827 <null>]"
	if got := c.str("FT.SUGGET", "ac", "he", "WITHSCORES", "WITHPAYLOADS"); got != want {
		t.Fatalf("FT.SUGGET = %s, want %s", got, want)
	}
}

func TestSuggestSurvivesAOFRestartAndRewrite(t *testing.T) {
	dir := t.TempDir()
	cfg := aofRewriteConfig(t, dir, func(c *config.Config) { c.Persistence.AOFRewritePercentage = 0 })
	s := startReplServerCfg(t, cfg)
	c := dialRepl(t, s)
	seedSuggest(t, c)
	checkSuggest(t, c)
	stopReplServer(s)

	s = startReplServerCfg(t, cfg)
	c = dialRepl(t, s)
	checkSuggest(t, c)
	c.do("BGREWRITEAOF")
	waitAOFRewrites(t, s, 1)
	stopReplServer(s)

	checkSuggest(t, dialRepl(t, startReplServerCfg(t, cfg)))
}

func TestSuggestSurvivesSnapshotRestart(t *testing.T) {
	cfg := replTestConfig(t, func(c *config.Config) {
		c.Persistence.Enabled = true
		c.Persistence.AOF = false
		c.Persistence.DataDir = t.TempDir()
		c.Persistence.SnapshotInterval = "0"
	})
	s := startReplServerCfg(t, cfg)
	c := dialRepl(t, s)
	seedSuggest(t, c)
	if got := c.str("SAVE"); got != "OK" {
		t.Fatalf("SAVE = %s", got)
	}
	stopReplServer(s)

	checkSuggest(t, dialRepl(t, startReplServerCfg(t, cfg)))
}

func TestSuggestReplicatesAndRestores(t *testing.T) {
	master := startReplServer(t, nil)
	replica := startReplServer(t, nil)
	r := replicate(t, replica, master)
	m := dialRepl(t, master)
	seedSuggest(t, m)
	waitSynced(t, replica, master)
	checkSuggest(t, r)

	dump := m.do("DUMP", "ac")
	m.do("DEL", "ac")
	if got := m.str("RESTORE", "ac", "0", string(dump.Bulk)); got != "OK" {
		t.Fatalf("RESTORE = %s", got)
	}
	checkSuggest(t, m)
}
