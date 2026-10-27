package server

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/config"
	"github.com/cachestorm/cachestorm/internal/persistence"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func TestSnapshotSaveAndRestart(t *testing.T) {
	dir := t.TempDir()
	cfg := replTestConfig(t, func(c *config.Config) {
		c.Persistence.Enabled = true
		c.Persistence.AOF = false
		c.Persistence.DataDir = dir
		c.Persistence.SnapshotInterval = "0"
	})
	s := startReplServerCfg(t, cfg)
	c := dialRepl(t, s)
	c.do("SET", "same", "zero", "PXAT", "16725225600000")
	c.do("ADDTAG", "same", "\xfftag")
	c.do("SELECT", "2")
	c.do("SET", "same", "two")
	c.do("XADD", "events", "1-0", "\xfffield", "one", "\xfffield", "two")
	c.do("XADD", "events", "2-0", "field", "deleted")
	c.do("XDEL", "events", "2-0")
	c.do("XGROUP", "CREATE", "events", "workers", "0")
	c.do("XREADGROUP", "GROUP", "workers", "consumer", "STREAMS", "events", ">")
	if got := c.do("SAVE"); got.Type != resp.TypeSimpleString || got.Str != "OK" {
		t.Fatalf("SAVE = %+v, want OK and a durable snapshot", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "dump.rdb")); err != nil {
		t.Fatal(err)
	}
	stopReplServer(s)
	restored := startReplServerCfg(t, cfg)
	r := dialRepl(t, restored)
	if got := r.do("GET", "same"); string(got.Bulk) != "zero" {
		t.Fatalf("db0 GET = %+v", got)
	}
	if got := r.do("PEXPIRETIME", "same"); got.Int != 16725225600000 {
		t.Fatalf("restored expiry = %+v", got)
	}
	if e, ok := restored.Store().Get("same"); !ok || len(e.Tags) != 1 || e.Tags[0] != "\xfftag" {
		t.Fatal("snapshot lost binary tag metadata")
	}
	r.do("SELECT", "2")
	if got := r.do("GET", "same"); string(got.Bulk) != "two" {
		t.Fatalf("db2 GET = %+v", got)
	}
	if got := r.do("XPENDING", "events", "workers"); got.Type != resp.TypeArray || len(got.Array) != 4 || got.Array[0].Int != 1 {
		t.Fatalf("restored pending state = %+v", got)
	}
	if got := r.do("XRANGE", "events", "-", "+"); got.Type != resp.TypeArray || len(got.Array) != 1 || len(got.Array[0].Array[1].Array) != 4 || string(got.Array[0].Array[1].Array[0].Bulk) != "\xfffield" {
		t.Fatalf("restored stream field order = %+v", got)
	}
	e, ok := restored.Databases().DB(2).Get("events")
	if !ok {
		t.Fatal("restored stream missing")
	}
	stream := e.Value.(*store.StreamValue)
	stream.RLock()
	last, deleted, added := stream.LastID, stream.MaxDeletedID, stream.EntriesAdded
	stream.RUnlock()
	if last != "2-0" || deleted != "2-0" || added != 2 {
		t.Fatalf("restored stream metadata: last=%s deleted=%s added=%d", last, deleted, added)
	}
	if got := r.do("XADD", "events", "2-0", "field", "new"); got.Type != resp.TypeError {
		t.Fatalf("restored stream accepted a reused ID: %+v", got)
	}
	if got := r.do("XADD", "events", "3-0", "field", "new"); string(got.Bulk) != "3-0" {
		t.Fatalf("restored stream could not append: %+v", got)
	}
}

func snapshotConfig(t *testing.T, dir string) *config.Config {
	t.Helper()
	return replTestConfig(t, func(c *config.Config) {
		c.Persistence.Enabled = true
		c.Persistence.AOF = false
		c.Persistence.DataDir = dir
		c.Persistence.SnapshotInterval = "0"
	})
}

func TestSnapshotBackgroundCaptureAndFailure(t *testing.T) {
	dir := t.TempDir()
	s := startReplServerCfg(t, snapshotConfig(t, dir))
	c := dialRepl(t, s)
	c.do("SET", "k", "captured")
	s.snapshotMu.Lock()
	s.snapshotLastTime = 1
	write := s.snapshotWrite
	entered, release := make(chan struct{}), make(chan struct{})
	s.snapshotWrite = func(data []byte) error {
		close(entered)
		<-release
		return write(data)
	}
	s.snapshotMu.Unlock()
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
		s.snapshotWG.Wait()
	})
	if got := c.do("BGSAVE"); got.Str != "Background saving started" {
		t.Fatalf("BGSAVE = %+v", got)
	}
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("background writer did not start")
	}
	if _, err := os.Stat(filepath.Join(dir, "dump.rdb")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("snapshot committed before writer release: %v", err)
	}
	if got := c.do("LASTSAVE"); got.Int != 1 {
		t.Fatalf("LASTSAVE advanced before completion: %+v", got)
	}
	if got := c.do("BGSAVE"); got.Type != resp.TypeError {
		t.Fatalf("overlapping BGSAVE = %+v", got)
	}
	if got := c.do("SAVE"); got.Type != resp.TypeError {
		t.Fatalf("overlapping SAVE = %+v", got)
	}
	if got := c.do("INFO", "persistence"); !strings.Contains(string(got.Bulk), "rdb_bgsave_in_progress:1") {
		t.Fatalf("INFO while saving = %+v", got)
	}
	if got := c.do("SET", "k", "after-capture"); got.Str != "OK" {
		t.Fatalf("writes blocked during background I/O: %+v", got)
	}
	close(release)
	s.snapshotWG.Wait()
	dst := store.NewStore()
	if err := persistence.NewRDBReader(dst).Load(filepath.Join(dir, "dump.rdb")); err != nil {
		t.Fatal(err)
	}
	if e, ok := dst.Get("k"); !ok || string(e.Value.(*store.StringValue).Data) != "captured" {
		t.Fatal("background save did not preserve its capture point")
	}
	last := c.do("LASTSAVE").Int
	if last <= 1 {
		t.Fatal("successful background completion did not advance LASTSAVE")
	}
	s.snapshotMu.Lock()
	s.snapshotWrite = func([]byte) error { return errors.New("injected disk failure") }
	s.snapshotMu.Unlock()
	if got := c.do("SAVE"); got.Type != resp.TypeError {
		t.Fatalf("failed SAVE = %+v", got)
	}
	if got := c.do("LASTSAVE"); got.Int != last {
		t.Fatalf("failed save advanced LASTSAVE: %+v", got)
	}
	if got := c.do("INFO", "persistence"); !strings.Contains(string(got.Bulk), "rdb_last_bgsave_status:err") {
		t.Fatalf("INFO after failure = %+v", got)
	}
}

func TestSnapshotHooksArePerServer(t *testing.T) {
	dir1, dir2 := t.TempDir(), t.TempDir()
	s1 := startReplServerCfg(t, snapshotConfig(t, dir1))
	c1 := dialRepl(t, s1)
	c1.do("SET", "k", "first")
	s2 := startReplServerCfg(t, snapshotConfig(t, dir2))
	c2 := dialRepl(t, s2)
	c2.do("SET", "k", "second")
	for _, c := range []*replClient{c1, c2} {
		if got := c.do("SAVE"); got.Str != "OK" {
			t.Fatalf("SAVE = %+v", got)
		}
	}
	for dir, want := range map[string]string{dir1: "first", dir2: "second"} {
		dst := store.NewStore()
		if err := persistence.NewRDBReader(dst).Load(filepath.Join(dir, "dump.rdb")); err != nil {
			t.Fatal(err)
		}
		if e, ok := dst.Get("k"); !ok || string(e.Value.(*store.StringValue).Data) != want {
			t.Fatalf("server snapshot in %s contains another server's state", dir)
		}
	}
}

func TestSnapshotPeriodicWriterAndStop(t *testing.T) {
	cfg := snapshotConfig(t, t.TempDir())
	cfg.Persistence.SnapshotInterval = "10ms"
	s, err := newOwnedTestServer(t, cfg)
	if err != nil {
		t.Fatal(err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	write := s.snapshotWrite
	s.snapshotWrite = func(data []byte) error {
		close(entered)
		<-release
		return write(data)
	}
	if err := s.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { stopReplServer(s) })
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
	})
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("periodic snapshot did not start")
	}
	done := make(chan struct{})
	go func() {
		stopReplServer(s)
		close(done)
	}()
	select {
	case <-s.stopCh:
	case <-time.After(5 * time.Second):
		t.Fatal("Stop did not begin")
	}
	select {
	case <-done:
		t.Fatal("Stop returned while the snapshot writer was still blocked")
	default:
	}
	close(release)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Stop did not join the snapshot writer")
	}
	if _, err := os.Stat(s.snapshotPath()); err != nil {
		t.Fatal(err)
	}
}

func TestSnapshotSeedsMissingAOFAndDefersToExistingAOF(t *testing.T) {
	dir := t.TempDir()
	cfg := snapshotConfig(t, dir)
	s := startReplServerCfg(t, cfg)
	c := dialRepl(t, s)
	c.do("FUNCTION", "FLUSH")
	t.Cleanup(func() { saveExec(t, s, "FUNCTION", "FLUSH") })
	code := "#!lua name=snaplib\nredis.register_function('snapshot_read', function(keys, args) return redis.call('GET', keys[1]) end)"
	if got := c.do("FUNCTION", "LOAD", code); string(got.Bulk) != "snaplib" {
		t.Fatalf("FUNCTION LOAD = %+v", got)
	}
	c.do("SET", "k", "snapshot")
	c.do("ADDTAG", "k", "persisted-tag")
	c.do("SELECT", "3")
	c.do("SET", "other-db", "present")
	if got := c.do("SAVE"); got.Str != "OK" {
		t.Fatal(got)
	}
	stopReplServer(s)
	saveExec(t, s, "FUNCTION", "FLUSH")
	cfg.Persistence.AOF = true
	cfg.Persistence.AOFSync = "always"
	seeded := startReplServerCfg(t, cfg)
	seedClient := dialRepl(t, seeded)
	if got := seedClient.do("GET", "k"); string(got.Bulk) != "snapshot" {
		t.Fatalf("seeded GET = %+v", got)
	}
	if got := seedClient.do("FCALL", "snapshot_read", "1", "k"); string(got.Bulk) != "snapshot" {
		t.Fatalf("snapshot lost function library: %+v", got)
	}
	seedClient.do("SET", "new-key", "aof")
	stopReplServer(seeded)
	saveExec(t, seeded, "FUNCTION", "FLUSH")
	if err := os.WriteFile(filepath.Join(dir, "dump.rdb"), []byte("invalid old snapshot"), 0600); err != nil {
		t.Fatal(err)
	}
	restarted := startReplServerCfg(t, cfg)
	r := dialRepl(t, restarted)
	if got := r.do("GET", "k"); string(got.Bulk) != "snapshot" {
		t.Fatalf("AOF lost seeded snapshot: %+v", got)
	}
	if got := r.do("GET", "new-key"); string(got.Bulk) != "aof" {
		t.Fatalf("AOF did not take priority: %+v", got)
	}
	if got := r.do("FCALL", "snapshot_read", "1", "k"); string(got.Bulk) != "snapshot" {
		t.Fatalf("AOF seed lost function library: %+v", got)
	}
	entry, ok := restarted.Store().Get("k")
	if !ok || len(entry.Tags) != 1 || entry.Tags[0] != "persisted-tag" {
		t.Fatalf("AOF seed lost snapshot tags: %+v", entry)
	}
	r.do("SELECT", "3")
	if got := r.do("GET", "other-db"); string(got.Bulk) != "present" {
		t.Fatalf("AOF seed lost database 3: %+v", got)
	}
}

func TestSnapshotCorruptionStopsStartup(t *testing.T) {
	dir := t.TempDir()
	cfg := snapshotConfig(t, dir)
	s := startReplServerCfg(t, cfg)
	c := dialRepl(t, s)
	c.do("SET", "k", "value")
	if got := c.do("SAVE"); got.Str != "OK" {
		t.Fatal(got)
	}
	stopReplServer(s)
	data, err := os.ReadFile(s.snapshotPath())
	if err != nil {
		t.Fatal(err)
	}
	data[len(data)/2] ^= 1
	if err := os.WriteFile(s.snapshotPath(), data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := newOwnedTestServer(t, cfg); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("startup with corrupt snapshot = %v", err)
	}
}
