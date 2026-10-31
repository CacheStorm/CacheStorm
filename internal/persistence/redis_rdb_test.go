package persistence

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

// testdata/redis74.rdb was written by Redis 7.4 (SAVE) under allkeys-lfu:
// every type in each of its encodings, an expiring key, hash field
// expirations, a stream with a consumer group, a function library, a key in
// database 3, and "str" read 30 times.
func TestLoadRedis74RDB(t *testing.T) {
	base := store.NewStore()
	lfu, _ := store.ParseEvictionPolicy("allkeys-lfu")
	base.SetMemoryLimit(0, lfu, 0)
	dbs := store.NewDatabases(base, 16)
	r := NewRDBReaderForDatabases(dbs)
	if err := r.Load("testdata/redis74.rdb"); err != nil {
		t.Fatal(err)
	}
	db := dbs.DB(0)
	str := func(key string) string {
		e, ok := db.Get(key)
		if !ok {
			t.Fatalf("%s missing", key)
		}
		return string(e.Value.(*store.StringValue).Data)
	}
	if str("str") != "hello" || str("int") != "12345" || len(str("long")) != 100 || str("hll")[:4] != "HYLL" {
		t.Fatal("string values differ")
	}
	if e, _ := db.Get("ttl"); e == nil || e.ExpiresAt.Load() != 4102444800000 {
		t.Fatalf("ttl key = %+v", e)
	}
	if e, _ := db.GetShard("str").Get("str"); e == nil || e.LFUCount() <= 5 {
		t.Fatalf("str lost its LFU counter")
	}
	sizes := map[string]int{"list": 3, "qlist": 200, "iset": 3, "lset": 2, "hset": 200, "hash": 2, "bighash": 200, "hfe": 2, "hfebig": 200, "zset": 2, "bigzset": 200}
	for key, want := range sizes {
		e, ok := db.Get(key)
		if !ok {
			t.Fatalf("%s missing", key)
		}
		var n int
		switch v := e.Value.(type) {
		case *store.ListValue:
			n = len(v.Elements)
		case *store.SetValue:
			n = len(v.Members)
		case *store.HashValue:
			n = len(v.Fields)
		case *store.SortedSetValue:
			n = len(v.Members)
		}
		if n != want {
			t.Fatalf("%s has %d elements, want %d", key, n, want)
		}
	}
	if e, _ := db.Get("hfe"); e.Value.(*store.HashValue).FieldExpireAt("a") != 4102444800000 {
		t.Fatal("hash field expiration lost")
	}
	if e, _ := db.Get("stream"); e == nil || len(e.Value.(*store.StreamValue).Groups) != 1 {
		t.Fatal("stream or its consumer group missing")
	}
	if _, ok := dbs.DB(3).Get("db3"); !ok {
		t.Fatal("database 3 key missing")
	}
	codes, err := DecodeFunctionDump([]byte(r.AuxFields["cachestorm-functions"]))
	if err != nil || len(codes) != 1 {
		t.Fatalf("functions = %q, %v", codes, err)
	}
}

func TestCacheStormSnapshotsAreNotTakenForRedis(t *testing.T) {
	for _, c := range []struct {
		version int
		aux     []string
		redis   bool
	}{
		{11, []string{rdbFormatAux, "1"}, false},
		{11, []string{"redis-ver", "7.0.0"}, false},
		{9, []string{"redis-ver", "7.0.0"}, false},
		{10, []string{"redis-ver", "7.0.0"}, true},
		{11, []string{"redis-ver", "7.2.4"}, true},
		{12, []string{"redis-ver", "7.4.0"}, true},
	} {
		body := []byte{byte(len(c.aux[0]))}
		body = append(body, c.aux[0]...)
		body = append(body, byte(len(c.aux[1])))
		body = append(body, c.aux[1]...)
		if got := writtenByRedis(c.version, 0xFA, bytes.NewReader(body)); got != c.redis {
			t.Errorf("version %d aux %v: Redis = %v, want %v", c.version, c.aux, got, c.redis)
		}
	}
}
