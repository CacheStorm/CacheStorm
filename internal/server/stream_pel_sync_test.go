package server

import (
	"strconv"
	"strings"
	"testing"
)

// streamPELAfterTrim delivers stream entries to a group and trims some of
// them away while they are still pending, which leaves them in the PEL.
func streamPELAfterTrim(c *replClient) {
	c.do("XGROUP", "CREATE", "s", "g", "0", "MKSTREAM")
	for i := 1; i <= 30; i++ {
		c.do("XADD", "s", strconv.Itoa(i)+"-0", "f", "v")
	}
	c.do("XREADGROUP", "GROUP", "g", "c1", "COUNT", "5", "STREAMS", "s", ">")
	c.do("XTRIM", "s", "MAXLEN", "20")
	c.do("XREADGROUP", "GROUP", "g", "c2", "COUNT", "5", "STREAMS", "s", ">")
}

// Entries trimmed or deleted from a stream while pending stay in the group's
// PEL. A full sync and a rewritten AOF carry the PEL as XCLAIM ... FORCE,
// which ignores an ID the stream no longer has, so a replica that synced
// and a server restarted from a rewritten AOF lost them. They keep them, as
// Redis' RDB does.
func TestPendingEntriesOfTrimmedEntriesSurviveSyncAndRewrite(t *testing.T) {
	cfg := aofRewriteConfig(t, t.TempDir(), nil)
	master := startReplServerCfg(t, cfg)
	c := dialRepl(t, master)
	streamPELAfterTrim(c)
	want := streamGroupState(c)
	if want == "" || !containsAll(want, "pending=10", "1-0") {
		t.Fatalf("bad control: %s", want)
	}

	late := startReplServer(t, nil)
	replicate(t, late, master)
	waitSynced(t, late, master)
	if got := streamGroupState(dialRepl(t, late)); got != want {
		t.Fatalf("after a full sync\nmaster:  %s\nreplica: %s", want, got)
	}

	c.do("BGREWRITEAOF")
	waitAOFRewrites(t, master, 1)
	restarted := checkAOFRestart(t, master, cfg)
	if got := streamGroupState(dialRepl(t, restarted)); got != want {
		t.Fatalf("after a rewritten AOF loaded\nbefore: %s\nafter:  %s", want, got)
	}
}

func TestPendingEntriesOfTrimmedEntriesSurviveASnapshot(t *testing.T) {
	cfg := snapshotConfig(t, t.TempDir())
	cfg.Persistence.AOF = false
	s := startReplServerCfg(t, cfg)
	c := dialRepl(t, s)
	streamPELAfterTrim(c)
	want := streamGroupState(c)
	if got := c.do("SAVE"); got.Str != "OK" {
		t.Fatalf("SAVE = %s", str(got))
	}
	stopReplServer(s)
	if got := streamGroupState(dialRepl(t, startReplServerCfg(t, cfg))); got != want {
		t.Fatalf("after a snapshot restart\nbefore: %s\nafter:  %s", want, got)
	}
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if !strings.Contains(s, p) {
			return false
		}
	}
	return true
}
