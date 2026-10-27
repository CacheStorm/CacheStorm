package server

import (
	"bytes"
	"strconv"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/config"
	"github.com/cachestorm/cachestorm/internal/replication"
	"github.com/cachestorm/cachestorm/internal/store"

	"github.com/cachestorm/cachestorm/internal/resp"
)

func TestHLLSnapshotRetainsCardinality(t *testing.T) {
	cfg := snapshotConfig(t, t.TempDir())
	cfg.Persistence.AOF = false
	s := startReplServerCfg(t, cfg)
	c := dialRepl(t, s)
	c.do("PFADD", "h", "a", "b", "", "long-element", "\x00binary")
	before := c.do("PFCOUNT", "h")
	if before.Type != resp.TypeInteger || before.Int == 0 {
		t.Fatalf("bad control: %#v", before)
	}
	if got := c.do("SAVE"); got.Str != "OK" {
		t.Fatalf("save=%#v", got)
	}
	stopReplServer(s)
	restored := startReplServerCfg(t, cfg)
	rc := dialRepl(t, restored)
	after := rc.do("PFCOUNT", "h")
	if after.Type != resp.TypeInteger || after.Int != before.Int {
		t.Fatalf("snapshot changed HLL: before=%#v after=%#v", before, after)
	}
}

func TestHLLFullSyncRetainsCardinality(t *testing.T) {
	master := startReplServer(t, nil)
	mc := dialRepl(t, master)
	mc.do("PFADD", "h", "a", "b", "c")
	replica := startReplServer(t, nil)
	rc := replicate(t, replica, master)
	waitSynced(t, replica, master)
	expected, actual := mc.do("PFCOUNT", "h"), rc.do("PFCOUNT", "h")
	if actual.Type != resp.TypeInteger || actual.Int != expected.Int {
		t.Fatalf("full sync changed HLL: %#v / %#v", expected, actual)
	}
}

func TestHLLAOFRewriteAndCacheReplay(t *testing.T) {
	for _, rewrite := range []bool{false, true} {
		t.Run(strconv.FormatBool(rewrite), func(t *testing.T) {
			cfg := aofTxConfig(t)
			s, run := aofTxStart(t, cfg)
			run("PFADD", "h", "a", "b", "")
			run("PEXPIREAT", "h", "4102444800000")
			run("ADDTAG", "h", "tag")
			if got := run("PFCOUNT", "h"); got != ":3\r\n" {
				t.Fatalf("count=%q", got)
			}
			entry, _ := s.store.Get("h")
			expected := entry.Value.String()
			if rewrite {
				if err := s.aof.Rewrite(replication.DBSource{DBs: s.dbs}); err != nil {
					t.Fatal(err)
				}
			}
			stopReplServer(s)
			restored, err := newOwnedTestServer(t, cfg)
			if err != nil {
				t.Fatal(err)
			}
			got, ok := restored.store.Get("h")
			if !ok || got.Value.String() != expected || got.ExpiresAt.Load() != 4102444800000 || len(got.TagList()) != 1 {
				t.Fatal("rewrite lost HLL data/cache/metadata")
			}
		})
	}
}

func TestHLLCountReadsRemainAvailableAfterAOFFailure(t *testing.T) {
	s := fullAOFServer(t)
	state := &store.HLLState{}
	state.Add([]byte("a"))
	state.Add([]byte("b"))
	data, err := state.Encode(true)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.store.Set("h", &store.StringValue{Data: data}, store.SetOptions{}); err != nil {
		t.Fatal(err)
	}
	run := func(name string, args ...string) string {
		var out bytes.Buffer
		argv := make([][]byte, len(args))
		for i := range args {
			argv[i] = []byte(args[i])
		}
		ctx := command.NewContext(name, argv, s.store, resp.NewWriter(&out))
		if err := s.router.Execute(ctx); err != nil {
			ctx.Writer.WriteError(err.Error())
		}
		return out.String()
	}
	if got := run("SET", "break", "v"); !strings.HasPrefix(got, "-MISCONF") {
		t.Fatalf("did not fail AOF: %q", got)
	}
	if got := run("PFCOUNT", "h"); got != ":2\r\n" {
		t.Fatalf("read blocked/misaligned after AOF failure: %q", got)
	}
	entry, _ := s.store.Get("h")
	if entry.Value.String() != string(data) {
		t.Fatal("failed writer still mutated count cache")
	}
}

func TestReplicaLocalHLLCountDoesNotInvalidateCheckpoint(t *testing.T) {
	master := startReplServer(t, nil)
	mc := dialRepl(t, master)
	mc.do("PFADD", "h", "a", "b")
	cfg := replTestConfig(t, func(c *config.Config) {
		c.Persistence.Enabled, c.Persistence.AOF = true, true
		c.Persistence.DataDir = t.TempDir()
		c.Replication.Role = "replica"
		c.Replication.MasterHost = "127.0.0.1"
		c.Replication.MasterPort = master.testPort()
	})
	replica := startReplServerCfg(t, cfg)
	rc := dialRepl(t, replica)
	waitLinkUp(t, rc)
	waitSynced(t, replica, master)
	for _, mode := range []string{"direct", "exec", "script"} {
		state := &store.HLLState{}
		state.Add([]byte("a"))
		state.Add([]byte("b"))
		data, _ := state.Encode(true)
		if err := replica.store.Set("h", &store.StringValue{Data: data}, store.SetOptions{}); err != nil {
			t.Fatal(err)
		}
		var got *resp.Value
		switch mode {
		case "direct":
			got = rc.do("PFCOUNT", "h")
		case "exec":
			rc.do("MULTI")
			rc.do("PFCOUNT", "h")
			got = rc.do("EXEC")
		case "script":
			got = rc.do("EVAL", "return redis.call('PFCOUNT',KEYS[1])", "1", "h")
		}
		if got.Type == resp.TypeError {
			t.Fatalf("replica count %s: %#v", mode, got)
		}
		if _, ok := replica.repl.SavedState(); !ok {
			t.Fatalf("cache-only %s invalidated history", mode)
		}
	}
}

func TestReplicaHLLCacheSurvivesPartialRestart(t *testing.T) {
	master := startReplServer(t, nil)
	mc := dialRepl(t, master)
	mc.do("PFADD", "h", "a", "b")
	cfg := replTestConfig(t, func(c *config.Config) {
		c.Persistence.Enabled, c.Persistence.AOF = true, true
		c.Persistence.DataDir = t.TempDir()
		c.Replication.Role = "replica"
		c.Replication.MasterHost = "127.0.0.1"
		c.Replication.MasterPort = master.testPort()
	})
	replica := startReplServerCfg(t, cfg)
	rc := dialRepl(t, replica)
	waitLinkUp(t, rc)
	waitSynced(t, replica, master)
	if got := rc.do("PFCOUNT", "h"); got.Int != 2 || got.Type != resp.TypeInteger {
		t.Fatalf("count=%#v", got)
	}
	stopReplServer(replica)
	mc.do("PFADD", "h", "c")
	restarted := startReplServerCfg(t, cfg)
	r2 := dialRepl(t, restarted)
	waitLinkUp(t, r2)
	waitSynced(t, restarted, master)
	if got := r2.do("PFCOUNT", "h"); got.Int != 3 || got.Type != resp.TypeInteger {
		t.Fatalf("restarted count=%#v", got)
	}
	full, partial, _ := syncStats(master)
	if full != 1 || partial != 1 {
		t.Fatalf("full=%d partial=%d", full, partial)
	}
}
