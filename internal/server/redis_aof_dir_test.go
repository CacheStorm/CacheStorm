package server

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/cachestorm/cachestorm/internal/persistence"
	"github.com/cachestorm/cachestorm/internal/replication"
	"github.com/cachestorm/cachestorm/internal/store"
)

// A server started on the data directory of a Redis 7 server with
// appendonly yes loads the multi-part AOF Redis left there, instead of
// starting empty, and leaves Redis' files as they were.
func TestStartLoadsRedisMultiPartAOF(t *testing.T) {
	dir := t.TempDir()
	aofDir := filepath.Join(dir, "appendonlydir")
	if err := os.Mkdir(aofDir, 0o755); err != nil {
		t.Fatal(err)
	}
	src := store.NewDatabases(store.NewStore(), 16)
	_ = src.DB(0).Set("base", &store.StringValue{Data: []byte("1")}, store.SetOptions{})
	_ = src.DB(1).Set("gone", &store.StringValue{Data: []byte("x")}, store.SetOptions{})
	var rdb bytes.Buffer
	if _, err := persistence.WriteRedisRDB(&rdb, replication.DBSource{DBs: src}, persistence.RedisRDBInfo{}); err != nil {
		t.Fatal(err)
	}
	incr := "#TS:1794231792\r\n" +
		"*1\r\n$5\r\nMULTI\r\n*2\r\n$6\r\nSELECT\r\n$1\r\n1\r\n*2\r\n$3\r\nDEL\r\n$4\r\ngone\r\n" +
		"*3\r\n$3\r\nSET\r\n$1\r\nt\r\n$1\r\nx\r\n*1\r\n$4\r\nEXEC\r\n" +
		"#TS:1794231793\r\n" +
		"*2\r\n$6\r\nSELECT\r\n$1\r\n0\r\n*3\r\n$3\r\nSET\r\n$5\r\nafter\r\n$1\r\ny\r\n" +
		"*3\r\n$3\r\nSET\r\n$4\r\ntorn" // cut off by a crash
	files := map[string]string{
		"appendonly.aof.2.base.rdb": rdb.String(),
		"appendonly.aof 2.incr.aof": incr,
		"appendonly.aof.manifest": "file appendonly.aof.1.base.rdb seq 1 type h\n" +
			"file appendonly.aof.2.base.rdb seq 2 type b\n" +
			"file \"appendonly.aof 2.incr.aof\" seq 2 type i\n",
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(aofDir, name), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	s := newMultiDBAOFServer(t, dir)
	for _, c := range []struct {
		db        int
		key, want string
	}{{0, "base", "1"}, {0, "after", "y"}, {1, "t", "x"}, {1, "gone", "<missing>"}, {0, "torn", "<missing>"}} {
		if got := getString(t, s.dbs.DB(c.db), c.key); got != c.want {
			t.Errorf("db %d %s = %q, want %q", c.db, c.key, got, c.want)
		}
	}
	stopReplServer(s)
	for name, data := range files {
		got, err := os.ReadFile(filepath.Join(aofDir, name))
		if err != nil || string(got) != data {
			t.Fatalf("Redis file %s changed: %v", name, err)
		}
	}

	// The next start reads CacheStorm's own AOF.
	s = newMultiDBAOFServer(t, dir)
	defer stopReplServer(s)
	if got := getString(t, s.dbs.DB(1), "t"); got != "x" {
		t.Fatalf("after a restart t = %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "appendonly.aof")); err != nil {
		t.Fatal(err)
	}
}
