package server

import (
	"strings"
	"testing"
)

// The per-command stats entries are cached between calls; CONFIG RESETSTAT
// must still start every command's counters again from zero.
func TestCommandStatsAfterResetStat(t *testing.T) {
	s := startReplServerCfg(t, replTestConfig(t, nil))
	c := dialRepl(t, s)
	calls := func(cmd string) string {
		for _, line := range strings.Split(str(c.do("INFO", "commandstats")), "\r\n") {
			if rest, ok := strings.CutPrefix(line, "cmdstat_"+cmd+":calls="); ok {
				return rest[:strings.IndexByte(rest, ',')]
			}
		}
		return "0"
	}
	c.str("CONFIG", "RESETSTAT")
	for range 3 {
		c.str("SET", "k", "v")
	}
	c.str("GET", "k")
	if got := calls("set"); got != "3" {
		t.Fatalf("set calls = %s, want 3", got)
	}
	c.str("CONFIG", "RESETSTAT")
	c.str("SET", "k", "v")
	if got := calls("set"); got != "1" {
		t.Fatalf("set calls after RESETSTAT = %s, want 1", got)
	}
	if got := calls("get"); got != "0" {
		t.Fatalf("get calls after RESETSTAT = %s, want 0", got)
	}
}
