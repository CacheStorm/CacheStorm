package server

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/config"
	"github.com/cachestorm/cachestorm/internal/resp"
)

func TestReplicationFunctionsFullSyncLiveAndRestart(t *testing.T) {
	master := startReplServer(t, nil)
	m := dialRepl(t, master)
	m.do("SET", "lookup", "master")
	m.do("SELECT", "3")
	m.do("SET", "lookup", "master-db3")
	m.do("SELECT", "0")
	code := "#!lua name=sync_library\nredis.register_function{function_name='sync_read', callback=function(keys) return redis.call('GET', keys[1]) end, flags={'no-writes'}}"
	if got := m.do("FUNCTION", "LOAD", code); string(got.Bulk) != "sync_library" {
		t.Fatal(got)
	}
	cfg := replTestConfig(t, func(c *config.Config) {
		c.Persistence.Enabled = true
		c.Persistence.AOF = true
		c.Persistence.AOFSync = "always"
		c.Persistence.DataDir = t.TempDir()
		c.Persistence.SnapshotInterval = "0"
	})
	replica := startReplServerCfg(t, cfg)
	r := dialRepl(t, replica)
	stale := "#!lua name=stale_library\nredis.register_function{function_name='sync_read', callback=function() return 'stale' end, flags={'no-writes'}}"
	if got := r.do("FUNCTION", "LOAD", stale); string(got.Bulk) != "stale_library" {
		t.Fatal(got)
	}
	r = replicate(t, replica, master)
	waitSynced(t, replica, master)
	if got := r.do("FCALL_RO", "sync_read", "1", "lookup"); string(got.Bulk) != "master" {
		t.Fatalf("full sync did not replace stale functions: %+v", got)
	}
	r.do("SELECT", "3")
	if got := r.do("FCALL_RO", "sync_read", "1", "lookup"); string(got.Bulk) != "master-db3" {
		t.Fatalf("function did not use the selected database: %+v", got)
	}
	r.do("SELECT", "0")
	live := "#!lua name=live_library\nredis.register_function{function_name='live_read', callback=function(keys, args) return args[1] end, flags={'no-writes'}}"
	if got := m.do("FUNCTION", "LOAD", live); string(got.Bulk) != "live_library" {
		t.Fatal(got)
	}
	waitSynced(t, replica, master)
	if got := r.do("FCALL_RO", "live_read", "0", "live"); string(got.Bulk) != "live" {
		t.Fatalf("live library not propagated: %+v", got)
	}
	m.do("FUNCTION", "DELETE", "live_library")
	waitSynced(t, replica, master)
	if got := r.do("FCALL_RO", "live_read", "0"); got.Type != resp.TypeError {
		t.Fatalf("deleted function still exists: %+v", got)
	}
	stopReplServer(replica)
	restarted := startReplServerCfg(t, cfg)
	rr := dialRepl(t, restarted)
	if got := rr.do("FCALL_RO", "sync_read", "1", "lookup"); string(got.Bulk) != "master" {
		t.Fatalf("replica AOF lost full-sync function library: %+v", got)
	}
	if got := rr.do("FCALL_RO", "live_read", "0"); got.Type != resp.TypeError {
		t.Fatalf("AOF resurrected deleted function: %+v", got)
	}
}

func TestReplicationEmptyFunctionSnapshotClearsOldLibraries(t *testing.T) {
	master := startReplServer(t, nil)
	replica := startReplServer(t, nil)
	r := dialRepl(t, replica)
	code := "#!lua name=stale\nredis.register_function('old_function', function() return 1 end)"
	if got := r.do("FUNCTION", "LOAD", code); string(got.Bulk) != "stale" {
		t.Fatal(got)
	}
	r = replicate(t, replica, master)
	waitSynced(t, replica, master)
	if got := r.do("FUNCTION", "LIST"); got.Type != resp.TypeArray || len(got.Array) != 0 {
		t.Fatalf("empty primary retained stale replica libraries: %+v", got)
	}
}
