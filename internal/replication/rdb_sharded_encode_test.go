package replication

import (
	"bytes"
	"strconv"
	"testing"

	"github.com/cachestorm/cachestorm/internal/persistence"
	"github.com/cachestorm/cachestorm/internal/store"
)

// serialSource hides DBSource's shards, so the encoder walks it in one pass.
type serialSource struct{ DBSource }

func (s serialSource) EntryShards() {}

func TestShardedRDBEncodeMatchesSerial(t *testing.T) {
	dbs := store.NewDatabases(store.NewStore(), 16)
	for _, db := range []int{0, 3, 15} {
		s := dbs.DB(db)
		for i := range 2000 {
			k := "k" + strconv.Itoa(db) + ":" + strconv.Itoa(i)
			var v store.Value = &store.StringValue{Data: []byte("v" + k)}
			switch i % 5 {
			case 1:
				v = store.NewListFrom([][]byte{[]byte("a"), []byte(k)})
			case 2:
				v = store.NewHashFrom(map[string][]byte{"f": []byte(k)})
			case 3:
				v = store.NewSortedSetFrom(map[string]float64{k: float64(i)})
			case 4:
				v = store.NewListFrom(nil) // empty: left out
			}
			_ = s.Set(k, v, store.SetOptions{})
			if i%7 == 0 {
				e, _ := s.Get(k)
				e.ExpiresAt.Store(4102444800000 + int64(i))
			}
		}
	}
	info := persistence.RedisRDBInfo{Extensions: true}
	sharded, skippedA, err := persistence.EncodeRedisRDB(DBSource{DBs: dbs}, info)
	if err != nil {
		t.Fatal(err)
	}
	serial, skippedB, err := persistence.EncodeRedisRDB(serialSource{DBSource{DBs: dbs}}, info)
	if err != nil {
		t.Fatal(err)
	}
	if skippedA != 1200 || skippedB != 1200 {
		t.Fatalf("skipped %d and %d keys, want the 1200 empty lists", skippedA, skippedB)
	}
	if len(sharded) != len(serial) {
		t.Fatalf("sharded encoding is %d bytes, serial %d", len(sharded), len(serial))
	}
	var streamed bytes.Buffer
	if _, err := persistence.WriteRedisRDB(&streamed, DBSource{DBs: dbs}, info); err != nil {
		t.Fatal(err)
	}
	for _, data := range [][]byte{sharded, serial} {
		if err := persistence.SealRDBChecksum(data); err != nil {
			t.Fatal(err)
		}
	}
	for _, data := range [][]byte{sharded, serial, streamed.Bytes()} {
		loaded := store.NewDatabases(store.NewStore(), 16)
		if err := persistence.NewRDBReaderForDatabases(loaded).LoadBytes(data); err != nil {
			t.Fatal(err)
		}
		for _, db := range []int{0, 3, 15} {
			want, got := dbs.DB(db), loaded.DB(db)
			if got.KeyCount() != 1600 {
				t.Fatalf("db %d loaded %d keys, want 1600", db, got.KeyCount())
			}
			_ = want.ForEachLive(func(key string, e *store.Entry) error {
				g, ok := got.Get(key)
				if _, empty := e.Value.(*store.ListValue); empty && e.Value.(*store.ListValue).Len() == 0 {
					if ok {
						t.Fatalf("db %d: empty list %q was saved", db, key)
					}
					return nil
				}
				if !ok {
					t.Fatalf("db %d: %q lost", db, key)
				}
				a, _ := persistence.EncodeDump(e.Value)
				b, _ := persistence.EncodeDump(g.Value)
				if !bytes.Equal(a, b) || e.ExpiresAt.Load() != g.ExpiresAt.Load() {
					t.Fatalf("db %d: %q differs after the round trip", db, key)
				}
				return nil
			})
		}
	}
}
