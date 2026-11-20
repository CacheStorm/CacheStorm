package server

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/config"
	"github.com/cachestorm/cachestorm/internal/hashslot"
)

// Redis 8's cluster-compatibility-sample-ratio on a standalone server.
func TestClusterCompatibilityStatistic(t *testing.T) {
	s := startReplServerCfg(t, replTestConfig(t, nil))
	c := dialRepl(t, s)
	if hashslot.Key("a") == hashslot.Key("b") {
		t.Fatal("a and b must be in different slots")
	}
	if _, ok := c.info("stats")["cluster_incompatible_ops"]; ok {
		t.Fatal("cluster_incompatible_ops is shown with the ratio at 0")
	}
	if got := c.str("CONFIG", "SET", "cluster-compatibility-sample-ratio", "101"); got != "ERR:ERR CONFIG SET failed (possibly related to argument 'cluster-compatibility-sample-ratio') - argument must be between 0 and 100 inclusive" {
		t.Fatalf("ratio 101: %s", got)
	}
	if got := c.str("CONFIG", "SET", "cluster-compatibility-sample-ratio", "100"); got != "OK" {
		t.Fatalf("CONFIG SET: %s", got)
	}
	defer c.str("CONFIG", "SET", "cluster-compatibility-sample-ratio", "0")
	c.str("CONFIG", "RESETSTAT")

	ops := func() string {
		t.Helper()
		v, ok := c.info("stats")["cluster_incompatible_ops"]
		if !ok {
			t.Fatal("no cluster_incompatible_ops")
		}
		return v
	}
	steps := []struct {
		want string
		args []string
	}{
		{"0", []string{"MSET", "{t}a", "1", "{t}b", "2"}},
		{"0", []string{"GET", "a"}},
		{"1", []string{"MSET", "a", "1", "b", "2"}},
		{"1", []string{"SELECT", "0"}},
		{"2", []string{"SELECT", "1"}},
		{"2", []string{"SELECT", "0"}},
		{"3", []string{"MOVE", "a", "1"}},
		{"4", []string{"COPY", "b", "{b}2", "DB", "1"}},
		{"4", []string{"COPY", "b", "{b}3"}},
		{"5", []string{"SWAPDB", "0", "1"}},
		{"6", []string{"SWAPDB", "0", "1"}},
		{"6", []string{"RPUSH", "l", "1", "2"}},
		{"7", []string{"SORT", "l", "BY", "w_*"}},
		{"7", []string{"SORT", "l", "BY", "{l}w_*", "GET", "#"}},
		{"8", []string{"SORT", "l", "GET", "{x}*"}},
		{"8", []string{"SORT", "l", "BY", "nosort"}},
		{"", []string{"MULTI"}},
		{"", []string{"SET", "a", "1"}},
		{"", []string{"SET", "b", "1"}},
		{"9", []string{"EXEC"}},
		{"10", []string{"EVAL", "#!lua\nredis.call('get','a') redis.call('get','b')", "0"}},
		{"10", []string{"EVAL", "#!lua\nredis.call('get','a') redis.call('get','a')", "0"}},
		{"11", []string{"EVAL", "#!lua\nredis.call('get','b')", "1", "a"}},
		{"11", []string{"EVAL", "#!lua flags=allow-cross-slot-keys\nredis.call('get','b')", "1", "a"}},
		// Without a shebang a script may use keys of any slot.
		{"11", []string{"EVAL", "redis.call('get','b')", "1", "a"}},
		{"12", []string{"EVAL", "redis.call('mget','a','b')", "0"}},
		{"13", []string{"EVAL", "#!lua flags=no-cluster\nreturn 1", "0"}},
		{"14", []string{"EVAL", "return 1", "2", "a", "b"}},
	}
	for _, st := range steps {
		c.do(st.args...)
		if st.want == "" {
			continue // inside MULTI, where INFO would be queued
		}
		if got := ops(); got != st.want {
			t.Fatalf("after %q: cluster_incompatible_ops %s, want %s", st.args, got, st.want)
		}
	}
	c.str("CONFIG", "RESETSTAT")
	if got := ops(); got != "0" {
		t.Fatalf("after RESETSTAT: %s", got)
	}
}

func TestClusterSortPatternsAndBusProtection(t *testing.T) {
	_, c := startClusterServer(t, t.TempDir(), func(cfg *config.Config) { cfg.Cluster.RequireFullCoverage = false })
	if got := c.str("CLUSTER", "ADDSLOTSRANGE", "0", "16383"); got != "OK" {
		t.Fatalf("ADDSLOTSRANGE: %s", got)
	}
	c.str("RPUSH", "l", "1")
	for _, tc := range [][]string{
		{"ERR:ERR BY option of SORT denied in Cluster mode when keys formed by the pattern may be in different slots.", "SORT", "l", "BY", "w_*"},
		{"ERR:ERR GET option of SORT denied in Cluster mode when keys formed by the pattern may be in different slots.", "SORT", "l", "GET", "w_*"},
		{"[1]", "SORT", "l", "BY", "{l}w_*"},
		{"[<null>]", "SORT", "l", "GET", "{l}w_*"},
		{"[1]", "SORT", "l", "BY", "nosort", "GET", "#"},
		{"no", "CONFIG", "GET", "tls-cluster"},
		{"ERR:ERR CONFIG SET failed (possibly related to argument 'cluster-bus-port-protected-mode') - can't enable cluster-bus-port-protected-mode while tls-cluster is disabled, as that leaves the cluster bus port unauthenticated; enable tls-cluster first, or in the same CONFIG SET command", "CONFIG", "SET", "cluster-bus-port-protected-mode", "yes"},
		{"OK", "CONFIG", "SET", "cluster-bus-port-protected-mode", "no"},
	} {
		got := c.str(tc[1:]...)
		if tc[1] == "CONFIG" && tc[2] == "GET" && len(got) > 0 {
			got = got[len("[tls-cluster ") : len(got)-1]
		}
		if got != tc[0] {
			t.Fatalf("%q: %s, want %s", tc[1:], got, tc[0])
		}
	}
}
