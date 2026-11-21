package server

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/config"
)

// XREADGROUP's effects reach replicas: the PEL, the group's last delivered
// ID and read counter, and consumers it created.
func TestXReadGroupReplicates(t *testing.T) {
	master := startReplServer(t, nil)
	replica := startReplServer(t, nil)
	mc := dialRepl(t, master)
	rc := replicate(t, replica, master)
	waitSynced(t, replica, master)
	mc.do("XADD", "s", "1-0", "f", "v")
	mc.do("XADD", "s", "2-0", "f", "v")
	mc.do("XADD", "s", "3-0", "f", "v")
	mc.do("XGROUP", "CREATE", "s", "g", "0")
	mc.do("XREADGROUP", "GROUP", "g", "c1", "COUNT", "2", "STREAMS", "s", ">")
	mc.do("XREADGROUP", "GROUP", "g", "c2", "NOACK", "STREAMS", "s", ">")
	mc.do("XREADGROUP", "GROUP", "g", "idle", "STREAMS", "s", ">")
	waitSynced(t, replica, master)
	for _, args := range [][]string{
		{"XPENDING", "s", "g"},
		{"XPENDING", "s", "g", "-", "+", "10"},
	} {
		m, r := mc.str(args...), rc.str(args...)
		if args[len(args)-1] == "10" {
			// Idle times differ by when each side looks.
			m, r = stripIdle(m), stripIdle(r)
		}
		if m != r {
			t.Fatalf("%v: master %s, replica %s", args, m, r)
		}
	}
	for _, f := range []string{"name", "consumers", "pending", "last-delivered-id", "entries-read", "lag"} {
		m, r := groupField(t, mc, f), groupField(t, rc, f)
		if m != r {
			t.Fatalf("XINFO GROUPS %s: master %s, replica %s", f, m, r)
		}
	}
}

func stripIdle(s string) string {
	parts := strings.Fields(strings.NewReplacer("[", " [ ", "]", " ] ").Replace(s))
	// [id consumer idle deliveries]: drop the third field of each row.
	for i := 0; i+4 < len(parts); i++ {
		if parts[i] == "[" && strings.Contains(parts[i+1], "-") {
			parts[i+3] = "_"
		}
	}
	return strings.Join(parts, " ")
}

func groupField(t *testing.T, c *replClient, field string) string {
	t.Helper()
	v := c.do("XINFO", "GROUPS", "s")
	g := v.Array[0].Array
	for i := 0; i+1 < len(g); i += 2 {
		if string(g[i].Bulk) == field {
			return str(g[i+1])
		}
	}
	return ""
}

// Redis 8.4's XREADGROUP CLAIM takes over entries pending longer than the
// minimum idle time, oldest delivery first, before new entries, and replies
// with each entry's idle time and delivery count.
func TestXReadGroupClaim(t *testing.T) {
	s := startReplServerCfg(t, replTestConfig(t, nil))
	c := dialRepl(t, s)
	expect := func(want string, args ...string) {
		t.Helper()
		if got := c.str(args...); got != want {
			t.Fatalf("%v = %q, want %q", args, got, want)
		}
	}
	expect("ERR:ERR The CLAIM option is only supported by XREADGROUP. You called XREAD instead.", "XREAD", "CLAIM", "10", "STREAMS", "s", "0")
	expect("ERR:ERR min-idle-time is not an integer or out of range", "XREADGROUP", "GROUP", "g", "c", "CLAIM", "x", "STREAMS", "s", ">")
	expect("ERR:ERR min-idle-time must be a positive integer", "XREADGROUP", "GROUP", "g", "c", "CLAIM", "-1", "STREAMS", "s", ">")

	c.do("XADD", "s", "1-0", "f", "a")
	c.do("XADD", "s", "2-0", "f", "b")
	c.do("XGROUP", "CREATE", "s", "g", "0")
	expect("[[s [[1-0 [f a]] [2-0 [f b]]]]]", "XREADGROUP", "GROUP", "g", "c1", "STREAMS", "s", ">")
	// Nothing is idle long enough and nothing is new.
	expect("<nullarray>", "XREADGROUP", "GROUP", "g", "c2", "CLAIM", "60000", "STREAMS", "s", ">")
	time.Sleep(60 * time.Millisecond)
	c.do("XADD", "s", "3-0", "f", "c")
	got := c.str("XREADGROUP", "GROUP", "g", "c2", "CLAIM", "50", "STREAMS", "s", ">")
	// The two idle entries, delivered once, then the new one.
	if !strings.HasPrefix(got, "[[s [[1-0 [f a] ") || !strings.Contains(got, " 1] [2-0 [f b] ") || !strings.HasSuffix(got, " 1] [3-0 [f c] 0 0]]]]") {
		t.Fatalf("XREADGROUP CLAIM = %s", got)
	}
	got = c.str("XPENDING", "s", "g", "-", "+", "10")
	if !strings.Contains(got, "[1-0 c2 ") || !strings.Contains(got, "[2-0 c2 ") || !strings.Contains(got, "[3-0 c2 ") {
		t.Fatalf("XPENDING after CLAIM = %s", got)
	}
	if !strings.Contains(got, " 2] [2-0") {
		t.Fatalf("claimed entry's delivery count not raised: %s", got)
	}
	// COUNT covers the claimed entries first.
	time.Sleep(60 * time.Millisecond)
	got = c.str("XREADGROUP", "GROUP", "g", "c3", "COUNT", "1", "CLAIM", "50", "STREAMS", "s", ">")
	if !strings.HasPrefix(got, "[[s [[1-0 [f a] ") || strings.Contains(got, "2-0") {
		t.Fatalf("XREADGROUP COUNT 1 CLAIM = %s", got)
	}
	// The history form ignores CLAIM.
	expect("[[s [[1-0 [f a]]]]]", "XREADGROUP", "GROUP", "g", "c3", "CLAIM", "0", "STREAMS", "s", "0")

	// A blocked reader wakes when an entry becomes claimable.
	c.do("XGROUP", "CREATE", "s", "g2", "$")
	c.do("XADD", "s", "4-0", "f", "d")
	c.do("XREADGROUP", "GROUP", "g2", "a", "STREAMS", "s", ">")
	start := time.Now()
	got = c.str("XREADGROUP", "GROUP", "g2", "b", "BLOCK", "5000", "CLAIM", "200", "STREAMS", "s", ">")
	if !strings.HasPrefix(got, "[[s [[4-0 [f d] ") {
		t.Fatalf("blocked CLAIM = %s", got)
	}
	if d := time.Since(start); d < 150*time.Millisecond || d > 3*time.Second {
		t.Fatalf("blocked CLAIM returned after %v", d)
	}
}

// An AOF restart keeps what XREADGROUP did to the group.
func TestXReadGroupSurvivesAOFRestart(t *testing.T) {
	cfg := replTestConfig(t, func(c *config.Config) {
		c.Persistence.Enabled, c.Persistence.AOF = true, true
		c.Persistence.DataDir = t.TempDir()
	})
	s := startReplServerCfg(t, cfg)
	c := dialRepl(t, s)
	c.do("XADD", "s", "1-0", "f", "v")
	c.do("XADD", "s", "2-0", "f", "v")
	c.do("XGROUP", "CREATE", "s", "g", "0")
	c.do("XREADGROUP", "GROUP", "g", "c", "COUNT", "1", "STREAMS", "s", ">")
	c.do("XREADGROUP", "GROUP", "g", "n", "NOACK", "STREAMS", "s", ">")
	want := c.str("XPENDING", "s", "g")
	stopReplServer(s)
	restarted, err := newOwnedTestServer(t, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := restarted.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	rc := dialRepl(t, restarted)
	if got := rc.str("XPENDING", "s", "g"); got != want {
		t.Fatalf("XPENDING after restart = %s, want %s", got, want)
	}
	if got := groupField(t, rc, "last-delivered-id"); got != "2-0" {
		t.Fatalf("last-delivered-id after restart = %s", got)
	}
	if got := groupField(t, rc, "consumers"); got != "2" {
		t.Fatalf("consumers after restart = %s", got)
	}
}
