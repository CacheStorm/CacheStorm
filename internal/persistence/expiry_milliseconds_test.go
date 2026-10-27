package persistence_test

import (
	"path/filepath"
	"strconv"
	"testing"

	"github.com/cachestorm/cachestorm/internal/persistence"
	"github.com/cachestorm/cachestorm/internal/store"
)

func TestPersistenceExpiryMillisecondsFullRange(t *testing.T) {
	for _, at := range []int64{4102444800000, 16725225600000, 9223372036854775807} {
		for _, format := range []string{"aof", "rdb"} {
			t.Run(format+"/"+strconv.FormatInt(at, 10), func(t *testing.T) {
				src := store.NewStore()
				if err := src.Set("k", &store.StringValue{Data: []byte("v")}, store.SetOptions{ExpireAtMS: at}); err != nil {
					t.Fatal(err)
				}
				var dst *store.Store
				if format == "aof" {
					dst = rewriteTTLAndReplay(t, src)
				} else {
					path := filepath.Join(t.TempDir(), "dump.rdb")
					if err := persistence.NewRDBWriter(src, persistence.RDBConfig{Version: persistence.RDBVersion11}).Save(path); err != nil {
						t.Fatal(err)
					}
					dst = store.NewStore()
					if err := persistence.NewRDBReader(dst).Load(path); err != nil {
						t.Fatal(err)
					}
				}
				entry, ok := dst.Get("k")
				if !ok {
					t.Fatal("key missing after reload")
				}
				if got := string(entry.Value.(*store.StringValue).Data); got != "v" {
					t.Fatalf("value after reload = %q", got)
				}
				if got := entry.ExpiresAt.Load(); got != at {
					t.Fatalf("expiry after reload = %d, want %d", got, at)
				}
			})
		}
	}
}
