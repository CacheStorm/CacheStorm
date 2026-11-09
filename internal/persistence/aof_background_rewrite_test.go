package persistence

import (
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

// shardedDBs is a ShardedEntrySource over a store.Databases.
type shardedDBs struct{ dbs *store.Databases }

func (s shardedDBs) DatabaseEntries() []map[string]interface{} { return nil }

func (s shardedDBs) ForEachEntry(fn func(db int, key string, entry interface{}) error) error {
	for _, shard := range s.EntryShards() {
		if err := shard.Each(func(key string, e *store.Entry) error { return fn(shard.DB, key, e) }); err != nil {
			return err
		}
	}
	return nil
}

func (s shardedDBs) EntryShards() []EntryShard {
	var out []EntryShard
	for i := 0; i < s.dbs.Count(); i++ {
		db := s.dbs.Existing(i)
		if db == nil {
			continue
		}
		for j := 0; j < store.NumShards; j++ {
			out = append(out, EntryShard{DB: i, Each: func(fn func(string, *store.Entry) error) error {
				return db.ForEachLiveInShard(j, fn)
			}})
		}
	}
	return out
}

func TestAOFRewriteKeepsAppendsMadeWhileItRuns(t *testing.T) {
	w, path := startTestAOF(t)
	arg := func(s ...string) [][]byte {
		out := make([][]byte, len(s))
		for i, x := range s {
			out[i] = []byte(x)
		}
		return out
	}
	dbs := store.NewDatabases(store.NewStore(), 16)
	_ = dbs.DB(0).Set("base0", &store.StringValue{Data: []byte("x")}, store.SetOptions{})
	_ = dbs.DB(2).Set("base2", &store.StringValue{Data: []byte("y")}, store.SetOptions{})
	// The file is in database 4 when the rewrite starts.
	if err := w.AppendDB(4, "SET", arg("old", "4")); err != nil {
		t.Fatal(err)
	}
	r, err := w.BeginRewrite(shardedDBs{dbs})
	if err != nil {
		t.Fatal(err)
	}
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(w.AppendDB(4, "SET", arg("t1", "4")))
	must(w.AppendTransaction([]Command{
		{Name: "SET", Args: arg("t2", "4"), DB: 4},
		{Name: "SET", Args: arg("t3", "6"), DB: 6},
	}))
	must(r.Finish())
	must(w.AppendDB(6, "SET", arg("after", "6")))
	must(w.AppendDB(0, "SET", arg("after0", "0")))
	want := []string{
		"SET base0 x",
		"SELECT 2", "SET base2 y",
		"SELECT 0",
		"SELECT 4", "SET t1 4",
		"MULTI", "SET t2 4", "SELECT 6", "SET t3 6", "EXEC",
		"SET after 6",
		"SELECT 0", "SET after0 0",
	}
	if got := loadNames(t, path); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("AOF:\n got %q\nwant %q", got, want)
	}
}

func TestAOFRewriteWithoutAppendsEndsInDatabaseZero(t *testing.T) {
	w, path := startTestAOF(t)
	dbs := store.NewDatabases(store.NewStore(), 16)
	_ = dbs.DB(3).Set("k", &store.StringValue{Data: []byte("v")}, store.SetOptions{})
	r, err := w.BeginRewrite(shardedDBs{dbs})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Finish(); err != nil {
		t.Fatal(err)
	}
	if err := w.AppendDB(0, "SET", [][]byte{[]byte("z"), []byte("0")}); err != nil {
		t.Fatal(err)
	}
	want := []string{"SELECT 3", "SET k v", "SELECT 0", "SET z 0"}
	if got := loadNames(t, path); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("AOF:\n got %q\nwant %q", got, want)
	}
}
