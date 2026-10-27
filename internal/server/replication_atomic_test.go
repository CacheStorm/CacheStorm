package server

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestFailedFullSyncCommitPreservesDataAndAOFDatabase(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "data")
	cfg := snapshotConfig(t, dir)
	cfg.Persistence.AOF = true
	cfg.Persistence.AOFSync = "always"
	s := startReplServerCfg(t, cfg)
	c := dialRepl(t, s)
	c.do("SET", "old", "kept")
	c.do("FUNCTION", "LOAD", "#!lua name=atomic_old\nredis.register_function('old_function', function() return 1 end)")
	c.do("SELECT", "3")
	c.do("SET", "db3", "three")
	entry, _ := s.Store().Get("old")
	version := s.Store().WatchKey("old")
	defer s.Store().UnwatchKey("old")
	staging := s.Databases().NewStaging()
	if err := staging.DB(0).Set("new", &store.StringValue{Data: []byte("replacement")}, store.SetOptions{}); err != nil {
		t.Fatal(err)
	}
	replay := s.router.ForkForReplay(staging.DB(0), []string{"FUNCTION"})
	aof := filepath.Join(dir, "appendonly.aof")
	before, err := os.ReadFile(aof)
	if err != nil {
		t.Fatal(err)
	}
	offline := dir + ".offline"
	if err := os.Rename(dir, offline); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir, []byte("injected directory outage"), 0600); err != nil {
		t.Fatal(err)
	}
	restore := func() {
		if _, err := os.Stat(offline); err == nil {
			os.Remove(dir)
			if err := os.Rename(offline, dir); err != nil {
				t.Error(err)
			}
		}
	}
	t.Cleanup(restore)
	s.LockWrites()
	err = s.CommitFullSync(staging, replay)
	s.UnlockWrites()
	restore()
	if err == nil {
		t.Fatal("obstructed AOF rewrite succeeded")
	}
	if current, ok := s.Store().Get("old"); !ok || current != entry || s.Store().Exists("new") || s.Store().GetVersion("old") != version {
		t.Fatal("failed commit changed live data or WATCH")
	}
	after, err := os.ReadFile(aof)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("failed rewrite changed the previous AOF")
	}
	if got := c.do("FCALL", "old_function", "0"); got.Int != 1 {
		t.Fatalf("failed commit replaced functions: %+v", got)
	}
	c.do("SELECT", "0")
	c.do("SET", "after-failure", "correct-db")
	stopReplServer(s)
	recovered := startReplServerCfg(t, cfg)
	r := dialRepl(t, recovered)
	if got := r.do("GET", "after-failure"); string(got.Bulk) != "correct-db" {
		t.Fatalf("failed rewrite lost the AOF's selected database: %+v", got)
	}
	r.do("SELECT", "3")
	if got := r.do("GET", "db3"); string(got.Bulk) != "three" {
		t.Fatalf("failed rewrite lost database 3: %+v", got)
	}
}
