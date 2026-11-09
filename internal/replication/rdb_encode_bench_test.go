package replication

import (
	"strconv"
	"testing"

	"github.com/cachestorm/cachestorm/internal/persistence"
	"github.com/cachestorm/cachestorm/internal/store"
)

// BenchmarkEncodeRedisRDB measures the encode step a snapshot runs while it
// holds the write gate.
func BenchmarkEncodeRedisRDB(b *testing.B) {
	dbs := store.NewDatabases(store.NewStore(), 16)
	db := dbs.DB(0)
	for i := range 200_000 {
		k := "key:" + strconv.Itoa(i)
		_ = db.Set(k, &store.StringValue{Data: []byte("val-" + k + "-xxxxxxxxxx")}, store.SetOptions{})
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, _, err := persistence.EncodeRedisRDBParts(DBSource{DBs: dbs}, persistence.RedisRDBInfo{Extensions: true}); err != nil {
			b.Fatal(err)
		}
	}
}
